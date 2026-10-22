package cmdhistory

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// MaxPerHost is how many commands are kept for one host; the least recently
// used are dropped first.
const MaxPerHost = 5000

// maxList bounds what one query returns, newest first.
const maxList = 20000

// ErrTooNew means the file was written by a newer version of the application.
var ErrTooNew = errors.New("command history: database is newer than this build")

const schemaVersion = 1

// Entry is one remembered command.
type Entry struct {
	Command string
	Used    time.Time // when it was last typed or picked
	Count   int       // how many times
}

// Store keeps the history in a SQLite file of its own, apart from the settings
// and the vault: it is never part of a backup or a sync, and it can be deleted
// without losing anything else. The file is private to the user (0600 inside a
// 0700 directory) and deleted rows are overwritten, not just unlinked.
type Store struct {
	db *sql.DB
}

// Open opens or creates the history at path.
func Open(path string) (*Store, error) {
	if path == "" || strings.ContainsAny(path, "?#") {
		return nil, errors.New("command history: bad database path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("command history: %w", err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("command history: %w", err)
	}
	file.Close()
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(DELETE)&_pragma=secure_delete(1)")
	if err != nil {
		return nil, fmt.Errorf("command history: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("command history: %w", err)
	}
	if version > schemaVersion {
		return ErrTooNew
	}
	if version == schemaVersion {
		return nil
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS commands (
		id      INTEGER PRIMARY KEY AUTOINCREMENT,
		host    TEXT NOT NULL,
		command TEXT NOT NULL,
		used    INTEGER NOT NULL,
		count   INTEGER NOT NULL,
		UNIQUE (host, command)
	);
	CREATE INDEX IF NOT EXISTS commands_used ON commands (host, used DESC);
	PRAGMA user_version = 1`)
	if err != nil {
		return fmt.Errorf("command history: %w", err)
	}
	return nil
}

// Close closes the file.
func (s *Store) Close() error { return s.db.Close() }

// Record notes that a command was used on a host.
func (s *Store) Record(host, command string, now time.Time) error {
	if host == "" || command == "" {
		return errors.New("command history: host and command are required")
	}
	return s.recordBatch([]queuedRecord{{host: host, command: command, used: now}})
}

// recordBatch records several commands in one transaction and trims each
// touched host once. Queue uses it to merge a burst of Enters.
func (s *Store) recordBatch(batch []queuedRecord) error {
	if len(batch) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	hosts := make(map[string]struct{}, len(batch))
	for _, record := range batch {
		if _, err := tx.Exec(`INSERT INTO commands (host, command, used, count) VALUES (?, ?, ?, 1)
			ON CONFLICT (host, command) DO UPDATE SET used = excluded.used, count = count + 1`,
			record.host, record.command, record.used.UnixMilli()); err != nil {
			return err
		}
		hosts[record.host] = struct{}{}
	}
	for host := range hosts {
		if _, err := tx.Exec(`DELETE FROM commands WHERE host = ? AND id IN
			(SELECT id FROM commands WHERE host = ? ORDER BY used DESC, id DESC LIMIT -1 OFFSET ?)`,
			host, host, MaxPerHost); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// List returns the commands of one host, or of every host when host is empty
// (a command typed on several hosts is one entry, with their counts added).
// Newest first.
func (s *Store) List(host string) ([]Entry, error) {
	var rows *sql.Rows
	var err error
	if host == "" {
		rows, err = s.db.Query(`SELECT command, MAX(used), SUM(count) FROM commands GROUP BY command ORDER BY MAX(used) DESC LIMIT ?`, maxList)
	} else {
		rows, err = s.db.Query(`SELECT command, used, count FROM commands WHERE host = ? ORDER BY used DESC, id DESC LIMIT ?`, host, maxList)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []Entry
	for rows.Next() {
		var entry Entry
		var millis int64
		if err := rows.Scan(&entry.Command, &millis, &entry.Count); err != nil {
			return nil, err
		}
		entry.Used = time.UnixMilli(millis)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// Delete forgets a command on one host, or on every host when host is empty.
func (s *Store) Delete(host, command string) error {
	if host == "" {
		_, err := s.db.Exec(`DELETE FROM commands WHERE command = ?`, command)
		return err
	}
	_, err := s.db.Exec(`DELETE FROM commands WHERE host = ? AND command = ?`, host, command)
	return err
}

// Clear forgets everything, and shrinks the file so the text is gone from it.
func (s *Store) Clear() error { return s.forget(`DELETE FROM commands`) }

// ForgetHost forgets everything typed on one host.
func (s *Store) ForgetHost(host string) error {
	return s.forget(`DELETE FROM commands WHERE host = ?`, host)
}

func (s *Store) forget(statement string, args ...any) error {
	if _, err := s.db.Exec(statement, args...); err != nil {
		return err
	}
	_, err := s.db.Exec(`VACUUM`)
	return err
}

// Len is how many commands are kept in all.
func (s *Store) Len() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM commands`).Scan(&count)
	return count, err
}
