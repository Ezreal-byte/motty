package term

import (
	"strings"
	"unicode"
)

// Link is one explicit OSC 8 or detected HTTP/HTTPS target in the retained
// terminal buffer. End is exclusive. A link may cross rows only where the
// terminal soft-wrapped them.
type Link struct {
	Start  BufferPosition
	End    BufferPosition
	Target string
}

// Links returns explicit OSC 8 links and finds HTTP/HTTPS targets in unlinked
// retained text, reusing dst. At most limit links are retained; truncated
// reports that more existed. Detection is performed only when Links is called.
func (t *Terminal) Links(dst []Link, limit int) ([]Link, bool) {
	dst = dst[:0]
	if limit <= 0 {
		return dst, false
	}

	scanner := linkScanner{dst: dst, limit: limit}
	var explicitID uint32
	var explicitStart, explicitEnd BufferPosition
	finishExplicit := func() bool {
		if explicitID == 0 {
			return false
		}
		target := t.hyperlinkTarget(explicitID)
		explicitID = 0
		if target == "" {
			return false
		}
		if len(scanner.dst) >= scanner.limit {
			return true
		}
		scanner.dst = append(scanner.dst, Link{Start: explicitStart, End: explicitEnd, Target: target})
		return false
	}
	for lineIndex := 0; lineIndex < t.bufferLineCount(); lineIndex++ {
		line := t.bufferLine(lineIndex)
		used := retainedLineUsedCells(line)
		mark := 0
		for column := 0; column < used; column++ {
			cell := line.cells[column]
			if cell.Attr&WideTail != 0 {
				continue
			}
			width := 1
			if cell.Attr&WideHead != 0 {
				width = 2
			}
			value := cell.R
			if value == 0 || value < 0x20 || value >= 0x7f && value < 0xa0 || cell.Attr&Hidden != 0 {
				value = ' '
			}
			start := BufferPosition{Line: lineIndex, Column: column}
			end := BufferPosition{Line: lineIndex, Column: min(column+width, t.cols)}
			hyperlink := line.hyperlinkAt(column)
			if hyperlink != explicitID {
				if finishExplicit() {
					return scanner.dst, true
				}
				if hyperlink != 0 {
					if scanner.breakLine() {
						return scanner.dst, true
					}
					explicitID = hyperlink
					explicitStart = start
				}
			}
			if explicitID != 0 {
				explicitEnd = end
				continue
			}
			if scanner.write(value, start, end) {
				return scanner.dst, true
			}
			for mark < len(line.marks) && int(line.marks[mark].Col) < column {
				mark++
			}
			for mark < len(line.marks) && int(line.marks[mark].Col) == column {
				if scanner.write(line.marks[mark].R, start, end) {
					return scanner.dst, true
				}
				mark++
			}
		}
		if !line.wrapped {
			if finishExplicit() || scanner.breakLine() {
				return scanner.dst, true
			}
		}
	}
	if finishExplicit() {
		return scanner.dst, true
	}
	if scanner.finish() {
		return scanner.dst, true
	}
	return scanner.dst, false
}

type linkScanner struct {
	dst   []Link
	limit int

	previous rune
	start    BufferPosition
	scheme   [8]rune
	matched  int
	secure   bool
	active   bool

	text        strings.Builder
	safeBytes   int
	safeEnd     BufferPosition
	parentheses int
	brackets    int
	braces      int
}

func (s *linkScanner) write(value rune, start, end BufferPosition) bool {
	if s.active {
		if linkTerminator(value) {
			if s.finish() {
				return true
			}
			s.previous = value
			return false
		}
		switch value {
		case ')':
			if s.parentheses == 0 {
				if s.finish() {
					return true
				}
				s.previous = value
				return false
			}
			s.parentheses--
		case ']':
			if s.brackets == 0 {
				if s.finish() {
					return true
				}
				s.previous = value
				return false
			}
			s.brackets--
		case '}':
			if s.braces == 0 {
				if s.finish() {
					return true
				}
				s.previous = value
				return false
			}
			s.braces--
		case '(':
			s.parentheses++
		case '[':
			s.brackets++
		case '{':
			s.braces++
		}

		s.text.WriteRune(value)
		if !linkTrailingPunctuation(value) {
			s.safeBytes = s.text.Len()
			s.safeEnd = end
		}
		s.previous = value
		return false
	}

	if s.matched == 0 {
		if (value == 'h' || value == 'H') && linkSchemeBoundary(s.previous) {
			s.start = start
			s.scheme[0] = value
			s.matched = 1
		}
		s.previous = value
		return false
	}

	if s.matchScheme(value) {
		s.scheme[s.matched] = value
		s.matched++
		if s.schemeComplete() {
			s.active = true
			s.text.Grow(64)
			for index := 0; index < s.matched; index++ {
				s.text.WriteRune(s.scheme[index])
			}
			s.safeBytes = s.text.Len()
			s.safeEnd = end
		}
		s.previous = value
		return false
	}

	s.matched = 0
	s.secure = false
	if (value == 'h' || value == 'H') && linkSchemeBoundary(s.previous) {
		s.start = start
		s.scheme[0] = value
		s.matched = 1
	}
	s.previous = value
	return false
}

func (s *linkScanner) matchScheme(value rune) bool {
	switch s.matched {
	case 1, 2:
		return value == 't' || value == 'T'
	case 3:
		return value == 'p' || value == 'P'
	case 4:
		if value == 's' || value == 'S' {
			s.secure = true
			return true
		}
		return value == ':'
	case 5:
		if s.secure {
			return value == ':'
		}
		return value == '/'
	case 6:
		return value == '/'
	case 7:
		return s.secure && value == '/'
	default:
		return false
	}
}

func (s *linkScanner) schemeComplete() bool {
	return !s.secure && s.matched == 7 || s.secure && s.matched == 8
}

func (s *linkScanner) breakLine() bool {
	truncated := s.finish()
	s.previous = 0
	s.matched = 0
	s.secure = false
	return truncated
}

func (s *linkScanner) finish() bool {
	if !s.active {
		s.matched = 0
		s.secure = false
		return false
	}
	text := s.text.String()[:s.safeBytes]
	if validLinkTarget(text, s.schemeLength()) {
		if len(s.dst) >= s.limit {
			return true
		}
		s.dst = append(s.dst, Link{Start: s.start, End: s.safeEnd, Target: text})
	}
	s.resetCandidate()
	return false
}

func (s *linkScanner) schemeLength() int {
	if s.secure {
		return len("https://")
	}
	return len("http://")
}

func (s *linkScanner) resetCandidate() {
	s.matched = 0
	s.secure = false
	s.active = false
	s.text = strings.Builder{}
	s.safeBytes = 0
	s.safeEnd = BufferPosition{}
	s.parentheses = 0
	s.brackets = 0
	s.braces = 0
}

func validLinkTarget(target string, schemeLength int) bool {
	authority := target[schemeLength:]
	if end := indexLinkAuthorityEnd(authority); end >= 0 {
		authority = authority[:end]
	}
	if at := strings.LastIndexByte(authority, '@'); at >= 0 {
		authority = authority[at+1:]
	}
	if len(authority) == 0 {
		return false
	}
	if authority[0] == '[' {
		close := strings.IndexByte(authority, ']')
		return close > 1
	}
	if colon := strings.LastIndexByte(authority, ':'); colon >= 0 {
		authority = authority[:colon]
	}
	for _, value := range authority {
		if unicode.IsLetter(value) || unicode.IsNumber(value) {
			return true
		}
	}
	return false
}

func indexLinkAuthorityEnd(authority string) int {
	for index, value := range authority {
		if value == '/' || value == '?' || value == '#' {
			return index
		}
	}
	return -1
}

func linkSchemeBoundary(value rune) bool {
	if value > unicode.MaxASCII {
		return true
	}
	return value == 0 || !isASCIIAlphaNumeric(value) && value != '_' && value != '+' && value != '-' && value != '.'
}

func isASCIIAlphaNumeric(value rune) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func linkTerminator(value rune) bool {
	if unicode.IsSpace(value) || unicode.IsControl(value) {
		return true
	}
	switch value {
	case '<', '>', '"', '\'', '`', '\\',
		'，', '。', '；', '：', '！', '？', '、',
		'（', '）', '［', '］', '｛', '｝', '【', '】', '《', '》', '〈', '〉', '“', '”', '‘', '’':
		return true
	default:
		return false
	}
}

func linkTrailingPunctuation(value rune) bool {
	switch value {
	case '.', ',', ';', ':', '!', '?', '#', '&':
		return true
	default:
		return false
	}
}
