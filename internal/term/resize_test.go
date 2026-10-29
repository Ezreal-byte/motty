package term

import (
	"reflect"
	"testing"
)

func TestResizeRestoresUntouchedClippedRows(t *testing.T) {
	terminal := New(16, 4, 20)
	feed(terminal, "0123456789ABCDEF\r\nsecond-row-END\r\nprompt> ")
	firstID := terminal.Row(0).ID
	for _, width := range []int{8, 4, 12, 6, 16} {
		terminal.Resize(width, 4)
		checkInvariants(t, terminal)
	}
	expectText(t, terminal, "0123456789ABCDEF\nsecond-row-END\nprompt>")
	if terminal.Row(0).ID != firstID {
		t.Fatal("resizing changed the identity of untouched output")
	}
}

func TestResizeRestoresClippedCellMetadata(t *testing.T) {
	terminal := New(16, 3, 20)
	terminal.SetClock(func() int64 { return 123 })
	feed(terminal, "\x1b[31;44mabc\u754ce\u0301\x1b]8;;https://example.test\x07LINK\x1b]8;;\x07")
	before := terminal.Row(0)
	cells := append([]Cell(nil), before.Cells...)
	marks := append([]Mark(nil), before.Marks...)
	terminal.Resize(4, 3) // Cuts through the wide character.
	if terminal.Row(0).Cells[3].Attr&WideHead != 0 {
		t.Fatal("narrow view exposes half a wide character")
	}
	terminal.Resize(16, 3)
	after := terminal.Row(0)
	if !reflect.DeepEqual(after.Cells, cells) || !reflect.DeepEqual(after.Marks, marks) || after.Written != 123 {
		t.Fatalf("restored row lost cells, marks or timestamp: %+v", after)
	}
	link, ok := terminal.LinksAt(BufferPosition{Line: 0, Column: 6})
	if !ok || link.Target != "https://example.test" {
		t.Fatalf("restored OSC 8 link = %+v, %v", link, ok)
	}
}

func TestResizeDoesNotRestoreEditedClippedRows(t *testing.T) {
	for _, test := range []struct {
		name, edit, want string
	}{
		{"overwrite", "\rXY", "XYcd"},
		{"erase", "\r\x1b[2K", ""},
		{"insert", "\r\x1b[@", " abc"},
		{"delete", "\r\x1b[P", "bcd"},
	} {
		t.Run(test.name, func(t *testing.T) {
			terminal := New(8, 3, 20)
			feed(terminal, "abcdEFGH")
			terminal.Resize(4, 3)
			feed(terminal, test.edit)
			terminal.Resize(8, 3)
			expectText(t, terminal, test.want)
		})
	}
}

func TestResizeRetainsClippedRowsWhenTheyScrollIntoHistory(t *testing.T) {
	terminal := New(8, 2, 20)
	feed(terminal, "ABCDEFGH\r\nijklmnop")
	terminal.Resize(4, 2)
	feed(terminal, "\r\nx")
	terminal.Resize(8, 2)
	expectText(t, terminal, "ijklmnop\nx")
	if got := string(terminal.AppendRetainedText(nil)); got != "ABCDEFGH\nijklmnop\nx" {
		t.Fatalf("retained output = %q", got)
	}
	if terminal.histBytes != terminal.historyBytes() {
		t.Fatal("history byte accounting drifted")
	}
}

func TestResizeRestoresClippedRowsPulledFromHistory(t *testing.T) {
	terminal := New(8, 2, 20)
	feed(terminal, "ABCDEFGH\r\nijklmnop\r\nq")
	terminal.Resize(4, 3)
	terminal.Resize(8, 3)
	expectText(t, terminal, "ABCDEFGH\nijklmnop\nq")
	if terminal.Scrollback() != 0 || terminal.histBytes != 0 {
		t.Fatal("pulled history is still counted")
	}
}

func TestResizeRetainsBackgroundMainButNotAlternateScreen(t *testing.T) {
	terminal := New(8, 3, 20)
	feed(terminal, "main-END\x1b[?1049h\x1b[Halt-END")
	terminal.Resize(4, 3)
	terminal.Resize(8, 3)
	expectText(t, terminal, "alt-")
	feed(terminal, "\x1b[?1049l")
	expectText(t, terminal, "main-END")
}

func TestResizeDoesNotRestoreClippedRowsAfterReset(t *testing.T) {
	terminal := New(8, 2, 20)
	feed(terminal, "OLD-TEXT")
	terminal.Resize(4, 2)
	terminal.Reset()
	feed(terminal, "new")
	terminal.Resize(8, 2)
	expectText(t, terminal, "new")
}

func TestResizeClippedWideHistoryViewKeepsWholeCells(t *testing.T) {
	for _, prefix := range []string{"", "\x1b[44m"} {
		t.Run(prefix, func(t *testing.T) {
			terminal := New(8, 2, 20)
			feed(terminal, prefix+"abc\u754c\u0301XYZ\r\nnext")
			terminal.Resize(4, 2)
			feed(terminal, "\r\nx")
			terminal.Scroll(1)
			row := terminal.Row(0)
			for _, cell := range row.Cells {
				if cell.Attr&(WideHead|WideTail) != 0 {
					t.Fatalf("history view exposes a partial wide cell: %+v", row)
				}
			}
			if len(row.Marks) != 0 {
				t.Fatalf("clipped glyph still has visible combining marks: %+v", row.Marks)
			}
			if prefix != "" && (len(row.Cells) != 4 || row.Cells[3] != (Cell{BG: Indexed(4)})) {
				t.Fatalf("clipping lost the boundary background: %+v", row.Cells)
			}
			terminal.Resize(8, 2)
			if row := terminal.Row(0); row.Cells[3].R != '\u754c' || len(row.Marks) != 1 {
				t.Fatalf("history storage lost the complete glyph: %+v", row)
			}
		})
	}
}
