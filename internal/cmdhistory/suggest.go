package cmdhistory

import "strings"

// Suggest is the rest of the command that the line typed so far most likely
// continues into: the most recently used entry that starts with it and is
// longer. entries are newest first, as Store.List returns them. A line that
// reads like a secret, or ends in a newline, gets nothing.
func Suggest(entries []Entry, line string) (rest string, ok bool) {
	if line == "" || strings.ContainsAny(line, "\r\n") || Secret(line) {
		return "", false
	}
	for _, entry := range entries {
		if len(entry.Command) > len(line) && strings.HasPrefix(entry.Command, line) {
			return entry.Command[len(line):], true
		}
	}
	return "", false
}
