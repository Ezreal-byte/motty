package cmdhistory

import (
	"errors"
	"sync"
	"time"
)

// queueFlushDelay batches commands entered in a burst. The window thread only
// appends to the queue; one background goroutine performs the SQLite commit.
const queueFlushDelay = 100 * time.Millisecond

// queuedRecord is one command waiting for its transaction.
type queuedRecord struct {
	host    string
	command string
	used    time.Time
}

// Queue writes command history on a goroutine. Reads and removals flush first,
// so callers see what they just recorded, while Enter no longer waits for a
// SQLite commit on the window thread.
type Queue struct {
	store *Store

	mu       sync.Mutex
	pending  []queuedRecord
	err      error // the last background write failure, until a flush succeeds
	closed   bool
	closeErr error
	wake     chan struct{}
	stop     chan struct{}
	done     chan struct{}

	writeMu   sync.Mutex // serializes recordBatch calls
	closeOnce sync.Once
}

// NewQueue starts a writer for store.
func NewQueue(store *Store) *Queue {
	q := &Queue{
		store: store,
		wake:  make(chan struct{}, 1),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	go q.run()
	return q
}

func (q *Queue) run() {
	defer close(q.done)
	for {
		select {
		case <-q.wake:
			timer := time.NewTimer(queueFlushDelay)
			select {
			case <-timer.C:
			case <-q.stop:
				timer.Stop()
				_ = q.flush()
				return
			}
			_ = q.flush()
		case <-q.stop:
			_ = q.flush()
			return
		}
	}
}

// Record appends a command to the queue. The disk write happens on the writer
// goroutine; use Flush or any read method to wait for it.
func (q *Queue) Record(host, command string, now time.Time) error {
	if host == "" || command == "" {
		return errors.New("command history: host and command are required")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return errors.New("command history: store is closed")
	}
	q.pending = append(q.pending, queuedRecord{host: host, command: command, used: now})
	select {
	case q.wake <- struct{}{}:
	default:
	}
	return nil
}

// flush commits everything queued, or returns the database error. A failed
// batch stays queued so a later flush can retry it.
func (q *Queue) flush() error {
	q.writeMu.Lock()
	defer q.writeMu.Unlock()
	q.mu.Lock()
	batch := q.pending
	q.pending = nil
	q.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	if err := q.store.recordBatch(batch); err != nil {
		q.mu.Lock()
		q.pending = append(batch, q.pending...)
		q.err = err
		q.mu.Unlock()
		return err
	}
	q.mu.Lock()
	q.err = nil
	q.mu.Unlock()
	return nil
}

// Error is the last background write failure, if a later write has not
// succeeded. Record itself cannot report a disk error without waiting for the
// commit; the window checks this after recording so the status can still say a
// history file is unavailable.
func (q *Queue) Error() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.err
}

// Flush waits for the queued commands to be committed.
func (q *Queue) Flush() error { return q.flush() }

// List returns the history after flushing what is still queued.
func (q *Queue) List(host string) ([]Entry, error) {
	if err := q.flush(); err != nil {
		return nil, err
	}
	return q.store.List(host)
}

// Delete forgets one command after flushing what is still queued.
func (q *Queue) Delete(host, command string) error {
	if err := q.flush(); err != nil {
		return err
	}
	return q.store.Delete(host, command)
}

// Clear forgets everything after flushing what is still queued.
func (q *Queue) Clear() error {
	if err := q.flush(); err != nil {
		return err
	}
	return q.store.Clear()
}

// ForgetHost forgets one host after flushing what is still queued.
func (q *Queue) ForgetHost(host string) error {
	if err := q.flush(); err != nil {
		return err
	}
	return q.store.ForgetHost(host)
}

// Len counts the stored commands after flushing what is still queued.
func (q *Queue) Len() (int, error) {
	if err := q.flush(); err != nil {
		return 0, err
	}
	return q.store.Len()
}

// Close flushes the queue, stops the writer and closes the file.
func (q *Queue) Close() error {
	q.closeOnce.Do(func() {
		q.mu.Lock()
		q.closed = true
		q.mu.Unlock()
		close(q.stop)
		<-q.done
		err := q.flush()
		if closeErr := q.store.Close(); err == nil {
			err = closeErr
		}
		q.mu.Lock()
		q.closeErr = err
		q.mu.Unlock()
	})
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closeErr
}
