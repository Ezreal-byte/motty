package sftpclient

import (
	"bytes"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

func newVerifyWorker(t *testing.T) *Worker {
	t.Helper()
	worker := NewWorker(func() (Remote, error) { return nil, nil }, nil)
	t.Cleanup(worker.Close)
	return worker
}

// trackingReaderAt reports how many reads were in flight at once and gives each
// one a little latency, so a serial verifier cannot reach the same maximum.
type trackingReaderAt struct {
	reader   *bytes.Reader
	delay    time.Duration
	inFlight atomic.Int32
	max      atomic.Int32
}

func (tracker *trackingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	now := tracker.inFlight.Add(1)
	for {
		seen := tracker.max.Load()
		if now <= seen || tracker.max.CompareAndSwap(seen, now) {
			break
		}
	}
	defer tracker.inFlight.Add(-1)
	time.Sleep(tracker.delay)
	return tracker.reader.ReadAt(p, off)
}

func (tracker *trackingReaderAt) Read(p []byte) (int, error) { return tracker.reader.Read(p) }

func (tracker *trackingReaderAt) Seek(offset int64, whence int) (int64, error) {
	return tracker.reader.Seek(offset, whence)
}

type plainReader struct{ io.Reader }

func TestVerifyPrefixComparesInParallelAndLeavesOffsetsAtThePrefix(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 16*1024) // 256 KiB
	worker := newVerifyWorker(t)
	source := &trackingReaderAt{reader: bytes.NewReader(data), delay: 2 * time.Millisecond}
	partial := &trackingReaderAt{reader: bytes.NewReader(data), delay: 2 * time.Millisecond}
	if err := worker.verifyPrefix(source, partial, int64(len(data)), 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := source.max.Load(); got < 2 {
		t.Fatalf("source reads never overlapped: max in flight %d", got)
	}
	if got := partial.max.Load(); got < 2 {
		t.Fatalf("partial reads never overlapped: max in flight %d", got)
	}
	if offset, _ := source.Seek(0, io.SeekCurrent); offset != int64(len(data)) {
		t.Fatalf("source offset = %d, want %d", offset, len(data))
	}
	if offset, _ := partial.Seek(0, io.SeekCurrent); offset != int64(len(data)) {
		t.Fatalf("partial offset = %d, want %d", offset, len(data))
	}
}

func TestVerifyPrefixReportsAMismatch(t *testing.T) {
	data := bytes.Repeat([]byte("a"), 200<<10)
	other := append([]byte(nil), data...)
	other[100<<10] = 'b'
	worker := newVerifyWorker(t)
	err := worker.verifyPrefix(bytes.NewReader(data), bytes.NewReader(other), int64(len(data)), 0, 0)
	if !errors.Is(err, ErrResumeMismatch) {
		t.Fatalf("err = %v, want ErrResumeMismatch", err)
	}
}

func TestVerifyPrefixFallsBackForReadersWithoutOffsets(t *testing.T) {
	data := bytes.Repeat([]byte("sequential "), 4096)
	worker := newVerifyWorker(t)
	source := plainReader{bytes.NewReader(data)}
	partial := plainReader{bytes.NewReader(data)}
	if err := worker.verifyPrefix(source, partial, int64(len(data)), 0, 0); err != nil {
		t.Fatal(err)
	}
	if source.Reader.(*bytes.Reader).Len() != 0 || partial.Reader.(*bytes.Reader).Len() != 0 {
		t.Fatal("the sequential fallback did not consume the prefix")
	}
}

func TestVerifyPrefixStopsOnCancel(t *testing.T) {
	data := bytes.Repeat([]byte("c"), 128<<10)
	worker := newVerifyWorker(t)
	worker.Cancel()
	err := worker.verifyPrefix(bytes.NewReader(data), bytes.NewReader(data), int64(len(data)), 0, 0)
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("err = %v, want ErrCanceled", err)
	}
}
