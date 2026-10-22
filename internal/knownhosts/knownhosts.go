// Package knownhosts manages the application's OpenSSH known_hosts file.
package knownhosts

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	// ErrNotFound means that at least one requested entry no longer exists.
	ErrNotFound = errors.New("knownhosts: entry not found")
	// ErrChanged means that another writer changed the file during a rewrite.
	ErrChanged = errors.New("knownhosts: file changed during update")
)

// ID identifies an entry by its content and duplicate occurrence. It remains
// stable when unrelated lines are inserted or removed.
type ID string

// Entry is one displayable public-key entry from an OpenSSH known_hosts file.
type Entry struct {
	ID           ID
	Line         int
	Marker       string
	HostPatterns []string
	KeyType      string
	Fingerprint  string
	Comment      string
	Hashed       bool
}

// File serializes management operations for one known_hosts path. Callers
// should retain one File per path while management UI is active.
type File struct {
	path string
	mu   sync.Mutex
}

// New returns a manager for path. It does not create or open the file.
func New(path string) *File {
	return &File{path: path}
}

// List returns all parseable public-key entries. Blank, comment, malformed,
// and otherwise unknown lines remain in the file but are not returned.
func (file *File) List() ([]Entry, error) {
	file.mu.Lock()
	defer file.mu.Unlock()

	data, err := file.read()
	if err != nil {
		return nil, err
	}
	_, entries := parseDocument(data)
	return entries, nil
}

// Search returns entries whose host pattern, key type, fingerprint, or comment
// contains query, ignoring case. An empty query returns every entry.
func (file *File) Search(query string) ([]Entry, error) {
	entries, err := file.List()
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return entries, nil
	}

	matches := entries[:0]
	for _, entry := range entries {
		if entryMatches(entry, query) {
			matches = append(matches, entry)
		}
	}
	return matches, nil
}

// DeleteLines atomically deletes the entries at lines. Every requested line
// must still contain a parseable entry or the file is left unchanged. This is
// the safe boundary for forgetting old keys before the connection flow writes
// a newly accepted key.
func (file *File) DeleteLines(lines ...int) error {
	wanted := make(map[int]struct{}, len(lines))
	for _, line := range lines {
		wanted[line] = struct{}{}
	}
	return file.delete(func(entry Entry) bool {
		_, ok := wanted[entry.Line]
		return ok
	}, len(wanted), os.Rename)
}

// DeleteIDs atomically deletes the entries with ids. IDs remain valid across
// unrelated line insertions and deletions. Every requested ID must still exist
// or the file is left unchanged.
func (file *File) DeleteIDs(ids ...ID) error {
	wanted := make(map[ID]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	return file.delete(func(entry Entry) bool {
		_, ok := wanted[entry.ID]
		return ok
	}, len(wanted), os.Rename)
}

func (file *File) read() ([]byte, error) {
	if file.path == "" {
		return nil, errors.New("knownhosts: empty path")
	}
	data, err := os.ReadFile(file.path)
	if err != nil {
		return nil, fmt.Errorf("knownhosts: read %q: %w", file.path, err)
	}
	return data, nil
}

func entryMatches(entry Entry, query string) bool {
	if strings.Contains(strings.ToLower(entry.KeyType), query) ||
		strings.Contains(strings.ToLower(entry.Fingerprint), query) ||
		strings.Contains(strings.ToLower(entry.Comment), query) {
		return true
	}
	for _, pattern := range entry.HostPatterns {
		if strings.Contains(strings.ToLower(pattern), query) {
			return true
		}
	}
	return false
}
