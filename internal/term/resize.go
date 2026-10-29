package term

// unclipped exposes a row's full output only while nothing has overwritten or
// erased its visible part. A changed row must never resurrect an old tail.
func (l *line) unclipped() *line {
	if saved := l.clipped; saved != nil {
		if saved.version == l.version && saved.wrapped == l.wrapped {
			return saved
		}
		l.clipped = nil
	}
	return l
}

func (t *Terminal) fitMainCols(l *line) {
	source := l.unclipped()
	oldWidth, oldVersion := len(l.cells), l.version
	if oldWidth == t.cols {
		return
	}
	if source != l {
		l.cells = append(l.cells[:0], source.cells...)
		l.marks = append(l.marks[:0], source.marks...)
		if source.hyperlinks != nil {
			var links []uint32
			if l.hyperlinks != nil {
				links = *l.hyperlinks
			}
			links = append(links[:0], (*source.hyperlinks)...)
			l.hyperlinks = &links
		}
	} else if t.cols < oldWidth && l.hasContentAfter(t.cols) {
		saved := *l
		saved.cells = append([]Cell(nil), l.cells...)
		saved.marks = append([]Mark(nil), l.marks...)
		if l.hyperlinks != nil {
			links := append([]uint32(nil), (*l.hyperlinks)...)
			saved.hyperlinks = &links
		}
		l.clipped = &saved
	}
	t.fitCols(l)
	if l.version == oldVersion {
		l.version++
	}
	if saved := l.clipped; saved != nil {
		if t.cols >= len(saved.cells) {
			l.clipped = nil
		} else {
			saved.version = l.version
		}
	}
}

func (l *line) hasContentAfter(column int) bool {
	for index := column; index < len(l.cells); index++ {
		if l.cells[index] != (Cell{}) || l.hyperlinkAt(index) != 0 {
			return true
		}
	}
	return false
}
