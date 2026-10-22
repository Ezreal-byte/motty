package workbench

import (
	"strconv"
	"time"

	"github.com/vivaui/viva/ui"
)

// terminalGutter is what the strip at the left of the terminal shows for each
// line: its number, the time it was written, both, or nothing.
//
// The gutter takes its width from the terminal's columns. The grid, the pty size
// the remote is told, selection, links, find and mouse coordinates all start at
// geometry.left, so moving that edge keeps every one of them consistent with
// no overlay and no second coordinate system. The columns stay reserved on the
// alternate screen too, where the strip is left empty: a full-screen program
// then owns every cell it was told about, and entering or leaving it never
// changes the size the remote sees.
type terminalGutter uint8

const (
	gutterOff terminalGutter = iota
	gutterNumbers
	gutterTime
	gutterBoth
)

const (
	gutterNumberChars = 6 // up to 999999, then the low digits
	gutterTimeChars   = 8 // HH:MM:SS
	gutterRuleGap     = float32(10)
)

func (gutter terminalGutter) String() string {
	switch gutter {
	case gutterNumbers:
		return "numbers"
	case gutterTime:
		return "time"
	case gutterBoth:
		return "both"
	}
	return "off"
}

func terminalGutterFromString(value string) terminalGutter {
	switch value {
	case "numbers":
		return gutterNumbers
	case "time":
		return gutterTime
	case "both":
		return gutterBoth
	}
	return gutterOff
}

func (gutter terminalGutter) numbers() bool { return gutter == gutterNumbers || gutter == gutterBoth }
func (gutter terminalGutter) times() bool   { return gutter == gutterTime || gutter == gutterBoth }

// chars is the gutter's text width in cells.
func (gutter terminalGutter) chars() int {
	n := 0
	if gutter.numbers() {
		n += gutterNumberChars
	}
	if gutter.times() {
		n += gutterTimeChars
	}
	if n == gutterNumberChars+gutterTimeChars {
		n++ // a cell between the two
	}
	return n
}

// gutterWidth is the pixels the gutter takes from the left of the grid.
func (state *State) gutterWidth() float32 {
	chars := state.terminalGutter.chars()
	if chars == 0 || state.cellWidth <= 0 {
		return 0
	}
	return snap(float32(chars)*state.cellWidth) + gutterRuleGap
}

func (state *State) wallSeconds() int64 {
	if state.wallClock != nil {
		return state.wallClock()
	}
	return time.Now().Unix()
}

// stampTerminal turns line timestamps on or off for a terminal. Off, the
// emulator records nothing.
func (state *State) stampTerminal(t *terminal) {
	if state.terminalGutter.times() {
		t.screen.SetClock(state.wallSeconds)
	} else {
		t.screen.SetClock(nil)
	}
}

func (state *State) setTerminalGutter(gutter terminalGutter) bool {
	if state.terminalGutter == gutter {
		return false
	}
	state.terminalGutter = gutter
	for index := range state.sessions {
		if t := state.sessions[index].term; t != nil {
			state.stampTerminal(t)
		}
	}
	state.fitTerminals()
	return true
}

// gutterEntry caches the text of one row's gutter so a repaint of unchanged
// rows formats nothing.
type gutterEntry struct {
	number uint64
	at     int64
	mode   terminalGutter
	text   string
}

// paintGutter draws the strip for row. entry is the row's cache entry.
func (state *State) paintGutter(canvas *ui.Canvas, entry *rowEntry, t *terminal, geometry terminalGeometry, row int, y float32, style ui.TextStyle) {
	mode := state.terminalGutter
	line := t.screen.Row(row)
	number := t.screen.LineNumber(row)
	cached := &entry.gutter
	if cached.text == "" || cached.number != number || cached.at != line.Written || cached.mode != mode {
		cached.number, cached.at, cached.mode = number, line.Written, mode
		cached.text = formatGutter(mode, number, line.Written)
	}
	if cached.text == "" {
		return
	}
	width := float32(mode.chars()) * state.cellWidth
	canvas.Label(cached.text, ui.Rect{X: geometry.left - state.gutterWidth(), Y: y, Width: width + 1, Height: state.cellHeight}, style)
}

// formatGutter is the text of a gutter: the number right-aligned, then the time.
func formatGutter(mode terminalGutter, number uint64, written int64) string {
	var buffer [gutterNumberChars + 1 + gutterTimeChars]byte
	text := buffer[:0]
	if mode.numbers() {
		var scratch [gutterNumberChars]byte
		digits := strconv.AppendUint(scratch[:0], number%1000000, 10)
		for pad := gutterNumberChars - len(digits); pad > 0; pad-- {
			text = append(text, ' ')
		}
		text = append(text, digits...)
	}
	if mode.times() {
		if mode.numbers() {
			text = append(text, ' ')
		}
		if written == 0 {
			text = append(text, "        "...)
		} else {
			text = time.Unix(written, 0).AppendFormat(text, "15:04:05")
		}
	}
	return string(text)
}
