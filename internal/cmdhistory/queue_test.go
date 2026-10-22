package cmdhistory

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestQueueFlushesOnReadAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	queue := NewQueue(store)
	base := time.Unix(1_700_000_000, 0)
	if err := queue.Record("a", "ls", base); err != nil {
		t.Fatal(err)
	}
	if err := queue.Record("a", "pwd", base.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	entries, err := queue.List("a") // flushes the two pending records
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Command != "pwd" || entries[1].Command != "ls" {
		t.Fatalf("entries = %+v", entries)
	}
	if err := queue.Record("a", "whoami", base.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := queue.Close(); err != nil {
		t.Fatal(err)
	}
	if err := queue.Close(); err != nil { // closing twice is harmless
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	entries, err = again.List("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("after close = %d entries, want 3", len(entries))
	}
}

func TestQueueRejectsAnEmptyCommand(t *testing.T) {
	store, _ := openStore(t)
	queue := NewQueue(store)
	defer queue.Close()
	if err := queue.Record("", "ls", time.Now()); err == nil {
		t.Fatal("an empty host was accepted")
	}
	if err := queue.Record("a", "", time.Now()); err == nil {
		t.Fatal("an empty command was accepted")
	}
}

// A burst of Enter presses is one transaction on the writer goroutine instead
// of one per command on the window thread.
func BenchmarkQueueRecordBurst(b *testing.B) {
	path := filepath.Join(b.TempDir(), "history.db")
	store, err := Open(path)
	if err != nil {
		b.Fatal(err)
	}
	queue := NewQueue(store)
	defer queue.Close()
	now := time.Unix(1_700_000_000, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for index := 0; index < 1000; index++ {
			if err := queue.Record("host", fmt.Sprintf("command-%d", index), now); err != nil {
				b.Fatal(err)
			}
		}
		if err := queue.Flush(); err != nil {
			b.Fatal(err)
		}
	}
}
