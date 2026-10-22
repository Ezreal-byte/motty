package workbench

import (
	"testing"
	"time"
)

func TestSynchronizedUpdateSnapshotOutlivesItsProgramOnlyUntilTheTimeout(t *testing.T) {
	terminal := newTerminal(20, 3)
	now := time.Duration(0)
	terminal.clock = func() time.Duration { return now }
	terminal.screen.SetSyncHold(terminal.holdSync)

	terminal.screen.Write([]byte("before"))
	terminal.screen.Write([]byte("\x1b[?2026hAFTER"))
	if terminal.sync == nil {
		t.Fatal("the update did not take a snapshot")
	}
	if got := terminal.presentedRow(0).Cells; len(got) == 0 || got[0].R != 'b' {
		t.Fatalf("presented row = %+v, want the screen from before the block", got)
	}
	if got := terminal.screen.RowText(0); got != "beforeAFTER" {
		t.Fatalf("live row = %q, the model must keep running during the block", got)
	}

	now = terminalSyncTimeout + time.Millisecond
	terminal.expireSync()
	if terminal.sync != nil {
		t.Fatal("a block without ESU held the screen past its timeout")
	}
	if got := terminal.presentedRow(0); len(got.Cells) == 0 || got.Cells[0].R != 'b' {
		t.Fatalf("presented row after expiry = %+v", got)
	}
}

func TestSyncSnapshotReusesItsCells(t *testing.T) {
	terminal := newTerminal(80, 24)
	terminal.clock = func() time.Duration { return 0 }
	terminal.screen.Write([]byte("hello\r\nworld"))
	terminal.holdSync(true)
	terminal.holdSync(false)
	if allocs := testing.AllocsPerRun(10, func() {
		terminal.holdSync(true)
		terminal.holdSync(false)
	}); allocs != 0 {
		t.Fatalf("a warm synchronized update allocates %v times", allocs)
	}
}

func BenchmarkHoldSyncScreen(b *testing.B) {
	terminal := newTerminal(200, 50)
	terminal.clock = func() time.Duration { return 0 }
	b.ReportAllocs()
	for b.Loop() {
		terminal.holdSync(true)
		terminal.holdSync(false)
	}
}
