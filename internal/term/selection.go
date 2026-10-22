package term

import (
	"strings"
	"unicode"
)

// Position identifies one visible terminal cell.
type Position struct {
	Row, Column int
}

// SelectedText returns the cells between two inclusive visible positions.
// Soft-wrapped rows are joined; hard line endings remain newlines.
func (t *Terminal) SelectedText(anchor, focus Position) string {
	anchor = t.clampPosition(anchor)
	focus = t.clampPosition(focus)
	if positionLess(focus, anchor) {
		anchor, focus = focus, anchor
	}

	var result strings.Builder
	result.Grow((focus.Row - anchor.Row + 1) * (t.cols + 1))
	for rowIndex := anchor.Row; rowIndex <= focus.Row; rowIndex++ {
		from, to := 0, t.cols-1
		if rowIndex == anchor.Row {
			from = anchor.Column
		}
		if rowIndex == focus.Row {
			to = focus.Column
		}
		row := t.Row(rowIndex)
		writeSelectedCells(&result, row, from, to)
		if rowIndex < focus.Row && !row.Wrapped {
			result.WriteByte('\n')
		}
	}
	return result.String()
}

// WordBounds returns the inclusive run of like cells around position. Letters,
// numbers, and underscores form words; spaces and punctuation form their own runs.
func (t *Terminal) WordBounds(position Position) (Position, Position) {
	position = t.clampPosition(position)
	row := t.Row(position.Row)
	column := position.Column
	if column < len(row.Cells) && row.Cells[column].Attr&WideTail != 0 && column > 0 {
		column--
	}
	kind := selectedCellKind(row.Cells, column)
	start := column
	for start > 0 {
		previous := start - 1
		if previous < len(row.Cells) && row.Cells[previous].Attr&WideTail != 0 && previous > 0 {
			previous--
		}
		if selectedCellKind(row.Cells, previous) != kind {
			break
		}
		start = previous
	}
	end := column
	for {
		next := end + selectedCellWidth(row.Cells, end)
		if next >= t.cols || selectedCellKind(row.Cells, next) != kind {
			break
		}
		end = next
	}
	end += selectedCellWidth(row.Cells, end) - 1
	return Position{Row: position.Row, Column: start}, Position{Row: position.Row, Column: min(end, t.cols-1)}
}

func (t *Terminal) clampPosition(position Position) Position {
	position.Row = clampInt(position.Row, 0, t.rows-1)
	position.Column = clampInt(position.Column, 0, t.cols-1)
	return position
}

func positionLess(left, right Position) bool {
	return left.Row < right.Row || left.Row == right.Row && left.Column < right.Column
}

func writeSelectedCells(result *strings.Builder, row Row, from, to int) {
	to = selectedRowEnd(row, from, to)
	mark := 0
	for column := from; column <= to; column++ {
		for mark < len(row.Marks) && int(row.Marks[mark].Col) < column {
			mark++
		}
		if column >= len(row.Cells) {
			result.WriteByte(' ')
			continue
		}
		cell := row.Cells[column]
		if cell.Attr&WideTail != 0 {
			continue
		}
		character := cell.R
		if character == 0 {
			character = ' '
		}
		result.WriteRune(character)
		for mark < len(row.Marks) && int(row.Marks[mark].Col) == column {
			result.WriteRune(row.Marks[mark].R)
			mark++
		}
	}
}

func selectedRowEnd(row Row, from, to int) int {
	for to >= from {
		if to >= len(row.Cells) {
			to--
			continue
		}
		cell := row.Cells[to]
		if cell.Attr&WideTail != 0 {
			if to > from && selectedCellContent(row, to-1) {
				return to
			}
			to--
			continue
		}
		if selectedCellContent(row, to) {
			return to
		}
		to--
	}
	return from - 1
}

func selectedCellContent(row Row, column int) bool {
	if column >= 0 && column < len(row.Cells) {
		character := row.Cells[column].R
		if character != 0 && character != ' ' {
			return true
		}
	}
	for _, mark := range row.Marks {
		if int(mark.Col) == column {
			return true
		}
		if int(mark.Col) > column {
			break
		}
	}
	return false
}

type cellKind uint8

const (
	cellBlank cellKind = iota
	cellWord
	cellPunctuation
)

func selectedCellKind(cells []Cell, column int) cellKind {
	if column < 0 || column >= len(cells) || cells[column].R == 0 || unicode.IsSpace(cells[column].R) {
		return cellBlank
	}
	if cells[column].R == '_' || unicode.IsLetter(cells[column].R) || unicode.IsNumber(cells[column].R) {
		return cellWord
	}
	return cellPunctuation
}

func selectedCellWidth(cells []Cell, column int) int {
	if column >= 0 && column < len(cells) && cells[column].Attr&WideHead != 0 {
		return 2
	}
	return 1
}

// TokenAt returns the run of non-blank text around a visible cell, without the
// brackets, quotes and sentence punctuation that usually wrap it: what a person
// would call "that address" or "that path" when pointing at it. It is empty on a
// blank cell. The token stays on its row, so a soft-wrapped path is cut where the
// wrap fell.
func (t *Terminal) TokenAt(position Position) string {
	position = t.clampPosition(position)
	row := t.Row(position.Row)
	blank := func(column int) bool {
		if column >= len(row.Cells) {
			return true
		}
		cell := row.Cells[column]
		return cell.Attr&WideTail == 0 && (cell.R == 0 || unicode.IsSpace(cell.R))
	}
	if blank(position.Column) {
		return ""
	}
	start, end := position.Column, position.Column
	for start > 0 && !blank(start-1) && position.Column-start < 512 {
		start--
	}
	for end+1 < t.cols && !blank(end+1) && end-position.Column < 512 {
		end++
	}
	text := t.SelectedText(Position{Row: position.Row, Column: start}, Position{Row: position.Row, Column: end})
	text = strings.TrimLeft(text, "([{<\"'`")
	return strings.TrimRight(text, ")]}>\"'`,;:!?.")
}
