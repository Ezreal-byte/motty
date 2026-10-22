// Package diag builds the report a user can attach to a bug report. It knows
// nothing of the application: callers hand it text, and it keeps what the user
// would not want to publish out of it.
package diag

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Log keeps the last lines written to it, for a report to show what the program
// said just before something went wrong. It is an io.Writer so it can sit beside
// the standard logger's own output.
type Log struct {
	mu       sync.Mutex
	lines    []string
	partial  []byte
	bytes    int
	maxLines int
	maxBytes int
}

// NewLog returns a Log that keeps at most maxLines lines and about maxBytes bytes.
func NewLog(maxLines, maxBytes int) *Log {
	return &Log{maxLines: maxLines, maxBytes: maxBytes}
}

// Write records p. It never fails, so it is safe in an io.MultiWriter.
func (log *Log) Write(p []byte) (int, error) {
	log.mu.Lock()
	defer log.mu.Unlock()
	data := append(log.partial, p...)
	for {
		end := bytes.IndexByte(data, '\n')
		if end < 0 {
			break
		}
		log.add(strings.TrimRight(string(data[:end]), "\r"))
		data = data[end+1:]
	}
	// A line that never ends is cut, not kept without bound.
	if len(data) > 4096 {
		data = data[:4096]
	}
	log.partial = append(log.partial[:0], data...)
	return len(p), nil
}

func (log *Log) add(line string) {
	if len(line) > 2048 {
		line = line[:2048] + "…"
	}
	log.lines = append(log.lines, line)
	log.bytes += len(line)
	for len(log.lines) > log.maxLines || log.bytes > log.maxBytes && len(log.lines) > 1 {
		log.bytes -= len(log.lines[0])
		log.lines = log.lines[1:]
	}
}

// Lines returns the kept lines, oldest first.
func (log *Log) Lines() []string {
	if log == nil {
		return nil
	}
	log.mu.Lock()
	defer log.mu.Unlock()
	return append([]string(nil), log.lines...)
}

// Redactor replaces what identifies the user's machines and accounts with
// stand-ins, in free text such as an error message.
type Redactor struct {
	pairs []pair
}

type pair struct{ secret, stand string }

// Hide makes Apply replace secret with stand. Short secrets are ignored: replacing
// "a" or "22" would wreck the text without hiding anything.
func (r *Redactor) Hide(secret, stand string) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 3 {
		return
	}
	for _, p := range r.pairs {
		if p.secret == secret {
			return
		}
	}
	r.pairs = append(r.pairs, pair{secret, stand})
	// Longest first, so "db.example.com" goes before "example.com".
	sort.SliceStable(r.pairs, func(i, j int) bool { return len(r.pairs[i].secret) > len(r.pairs[j].secret) })
}

var (
	ipv4 = regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	// Only the full form and the "::" form: a bare "12:30:45" is a time.
	ipv6    = regexp.MustCompile(`(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?::(?:[0-9a-fA-F]{1,4}(?::[0-9a-fA-F]{1,4}){0,6})?`)
	email   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	userAt  = regexp.MustCompile(`\b[A-Za-z0-9._-]+@(\[?[A-Za-z0-9.:-]+\]?)`)
	keyLike = regexp.MustCompile(`(?i)\b(password|passwd|passphrase|token|secret|api[_-]?key)\s*[=:]\s*\S+`)
)

// Apply returns text with the hidden values and anything that looks like an
// address, an account or a credential replaced.
func (r *Redactor) Apply(text string) string {
	for _, p := range r.pairs {
		text = strings.ReplaceAll(text, p.secret, p.stand)
	}
	if home, err := os.UserHomeDir(); err == nil && len(home) > 3 {
		text = strings.ReplaceAll(text, home, "~")
	}
	text = keyLike.ReplaceAllString(text, "$1=<hidden>")
	text = email.ReplaceAllString(text, "<account>")
	text = userAt.ReplaceAllString(text, "<user>@$1")
	text = ipv4.ReplaceAllString(text, "<ip>")
	text = ipv6.ReplaceAllString(text, "<ip>")
	return text
}

// Report is a plain-text document of titled sections. The user can read it all
// before sending it, which is why it is text and not an archive.
type Report struct {
	text bytes.Buffer
	r    *Redactor
}

// NewReport starts a report whose free text passes through r.
func NewReport(r *Redactor) *Report { return &Report{r: r} }

// Section starts a titled section.
func (report *Report) Section(title string) {
	if report.text.Len() > 0 {
		report.text.WriteByte('\n')
	}
	fmt.Fprintf(&report.text, "== %s ==\n", title)
}

// Field writes "key: value" with the value passed through the redactor.
func (report *Report) Field(key, value string) {
	fmt.Fprintf(&report.text, "%s: %s\n", key, report.r.Apply(value))
}

// Fact writes "key: value" for a value that cannot identify anyone (a number, a
// setting's name), skipping the redactor so nothing in it is mangled.
func (report *Report) Fact(key string, value any) {
	fmt.Fprintf(&report.text, "%s: %v\n", key, value)
}

// Line writes one line of free text through the redactor.
func (report *Report) Line(text string) {
	report.text.WriteString(report.r.Apply(text))
	report.text.WriteByte('\n')
}

// String is the finished report.
func (report *Report) String() string { return report.text.String() }
