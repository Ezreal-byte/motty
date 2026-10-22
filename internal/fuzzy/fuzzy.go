// Package fuzzy ranks candidates against a short query by subsequence match.
//
// A candidate matches when the query's characters appear in it in order,
// ignoring case. Its score rewards, from most to least: the match starting at
// the candidate's first character, characters that start a word, runs of
// adjacent characters, and it penalises the gaps between scattered ones. The
// score is only comparable between candidates of the same query.
//
// Scoring allocates nothing: compile the query once with Compile, then call
// Score for every candidate.
package fuzzy

import (
	"unicode"
	"unicode/utf8"
)

const (
	matchBase     = 16 // every matched character
	bonusPrefix   = 24 // the first match is the candidate's first character
	bonusBoundary = 16 // the match follows a separator
	bonusCamel    = 12 // the match is a capital after a lower-case letter
	bonusAdjacent = 12 // the match directly follows the previous one
	bonusWhole    = 40 // the query is a prefix of the candidate
	gapStart      = 3  // skipping characters costs this, plus one for each more
	lengthShift   = 3  // a candidate loses one point per 2^lengthShift bytes
)

// Query is a compiled query.
type Query struct {
	runes []rune // lower-cased
}

// Compile prepares a query. Spaces are dropped: "git  st" asks for the letters
// g, i, t, s, t.
func Compile(query string) Query {
	runes := make([]rune, 0, len(query))
	for _, r := range query {
		if !unicode.IsSpace(r) {
			runes = append(runes, fold(r))
		}
	}
	return Query{runes: runes}
}

func fold(r rune) rune {
	if r < utf8.RuneSelf {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}
	return unicode.ToLower(r)
}

func separator(r rune) bool {
	switch r {
	case ' ', '/', '\\', '-', '_', '.', ':', '=', '|', ',', ';', '@', '"', '\'', '(', '[', '{':
		return true
	}
	return false
}

// Score reports whether the candidate matches and how well. A higher score is
// a better match.
func (q Query) Score(candidate string) (score int, ok bool) {
	if len(q.runes) == 0 {
		return 0, true
	}
	// Forward: the earliest end of a match.
	next := 0
	end := 0
	for index, r := range candidate {
		if fold(r) == q.runes[next] {
			next++
			if next == len(q.runes) {
				end = index + utf8.RuneLen(r)
				break
			}
		}
	}
	if next < len(q.runes) {
		return 0, false
	}
	// Backward from that end: the tightest start, so a match is not stretched
	// across text when a shorter one ends in the same place.
	start := end
	remaining := len(q.runes) - 1
	for position := end; position > 0 && remaining >= 0; {
		r, size := utf8.DecodeLastRuneInString(candidate[:position])
		position -= size
		if fold(r) == q.runes[remaining] {
			remaining--
			if remaining < 0 {
				start = position
			}
		}
	}
	// Score the window greedily from its start.
	var previous rune // the character before the current one
	if start > 0 {
		previous, _ = utf8.DecodeLastRuneInString(candidate[:start])
	}
	next, last, matched := 0, -1, 0
	adjacentAll := true
	position := start
	for _, r := range candidate[start:end] {
		if next < len(q.runes) && fold(r) == q.runes[next] {
			score += matchBase
			switch {
			case position == 0:
				score += bonusPrefix
			case separator(previous):
				score += bonusBoundary
			case unicode.IsUpper(r) && unicode.IsLower(previous):
				score += bonusCamel
			}
			if last >= 0 {
				if gap := matched - last - 1; gap == 0 {
					score += bonusAdjacent
				} else {
					adjacentAll = false
					score -= gapStart + gap - 1
				}
			}
			last = matched
			next++
		}
		matched++
		position += utf8.RuneLen(r)
		previous = r
	}
	if start == 0 && adjacentAll {
		score += bonusWhole
	}
	return score - len(candidate)>>lengthShift, true
}
