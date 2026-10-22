package cmdhistory

import (
	"strings"

	"motty/internal/term"
)

// Screen is what the recorder reads of a terminal: the cursor and the text of
// rows. Columns are cell columns, so a double-width character counts twice.
type Screen interface {
	// Cursor is the cursor's column and its row in the visible screen.
	Cursor() (x, y int)
	// Alt reports the alternate screen, where full-screen programs run.
	Alt() bool
	// Scrolled reports a view that is not the live screen.
	Scrolled() bool
	// Wrapped reports that row y continues on row y+1 (the line was too long).
	Wrapped(y int) bool
	// Span is the text of columns [from, to) of row y; to < 0 means the end of
	// the row. With trim, blanks and the grey suggestion text some shells draw
	// after the cursor are dropped from its end.
	Span(y, from, to int, trim bool) string
}

// TermScreen reads the emulator the application draws.
type TermScreen struct{ Term *term.Terminal }

func (s TermScreen) Cursor() (x, y int) {
	cursor := s.Term.Cursor()
	return cursor.X, cursor.Y
}

func (s TermScreen) Alt() bool      { return s.Term.Modes().AltScreen }
func (s TermScreen) Scrolled() bool { return s.Term.ViewOffset() != 0 }

func (s TermScreen) Wrapped(y int) bool {
	if _, rows := s.Term.Size(); y < 0 || y >= rows {
		return false
	}
	return s.Term.Row(y).Wrapped
}

func (s TermScreen) Span(y, from, to int, trim bool) string {
	if _, rows := s.Term.Size(); y < 0 || y >= rows {
		return ""
	}
	row := s.Term.Row(y)
	cells := row.Cells
	if to < 0 || to > len(cells) {
		to = len(cells)
	}
	from = max(from, 0)
	if trim {
		for to > from && (cells[to-1].R == 0 || cells[to-1].R == ' ' || suggestion(cells[to-1])) {
			to--
		}
	}
	if from >= to {
		return ""
	}
	var out strings.Builder
	marks := row.Marks
	for column := from; column < to; column++ {
		cell := cells[column]
		if cell.Attr&term.WideTail != 0 {
			continue
		}
		switch {
		case cell.R == 0 || cell.Attr&term.Hidden != 0:
			out.WriteByte(' ')
		default:
			out.WriteRune(cell.R)
		}
		for _, mark := range marks {
			if int(mark.Col) == column && cell.Attr&term.Hidden == 0 {
				out.WriteRune(mark.R)
			}
		}
	}
	return out.String()
}

// suggestion reports a cell drawn the way shells draw a suggestion for the rest
// of the line (fish, zsh-autosuggestions): dim, bright black or a plain grey.
// Such text is not on the command line yet, so it is never part of a command.
func suggestion(cell term.Cell) bool {
	if cell.Attr&term.Dim != 0 {
		return true
	}
	switch cell.FG.Kind() {
	case term.PaletteColor:
		index := cell.FG.Index()
		return index == 8 || index >= 232
	case term.TrueColor:
		red, green, blue := cell.FG.Components()
		return red == green && green == blue && red > 0x30 && red < 0xb0
	}
	return false
}
