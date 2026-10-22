// Package scpclient speaks the classic SCP wire protocol over an SSH exec
// channel, for servers that have no SFTP subsystem. It is a strict
// implementation: whatever the peer announces is validated before it reaches
// the caller, so a hostile server cannot place a file outside the destination
// the caller chose (the CVE-2019-6111 class of attacks), claim a size it does
// not deliver, or make the client allocate or write without bound.
package scpclient

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"
)

// Channel is one running remote command: its standard output is Read, its
// standard input is Written. Close aborts the command.
type Channel interface {
	io.Reader
	io.Writer
	// CloseWrite sends end of input.
	CloseWrite() error
	// Wait returns once the command ended; a non-zero exit status is an error.
	Wait() error
	// Close tears the channel down without waiting.
	Close() error
	// Stderr is the end of what the command wrote to its standard error.
	Stderr() string
}

// Start runs command on the remote shell.
type Start func(command string) (Channel, error)

// Limits bound what a peer can make a transfer do.
type Limits struct {
	MaxFileSize  int64 // one file
	MaxTotalSize int64 // all files of one transfer
	MaxEntries   int   // files and directories of one transfer
	MaxDepth     int   // directory nesting
}

// DefaultLimits are the bounds a Client starts with.
var DefaultLimits = Limits{MaxFileSize: 1 << 40, MaxTotalSize: 1 << 42, MaxEntries: 100_000, MaxDepth: 128}

// Client runs SCP transfers and the few fixed shell commands the protocol
// needs around them.
type Client struct {
	start  Start
	limits Limits
}

// New returns a client that starts commands with start.
func New(start Start) *Client { return &Client{start: start, limits: DefaultLimits} }

// SetLimits replaces the limits; a zero field keeps the default.
func (c *Client) SetLimits(limits Limits) {
	if limits.MaxFileSize <= 0 {
		limits.MaxFileSize = DefaultLimits.MaxFileSize
	}
	if limits.MaxTotalSize <= 0 {
		limits.MaxTotalSize = DefaultLimits.MaxTotalSize
	}
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = DefaultLimits.MaxEntries
	}
	if limits.MaxDepth <= 0 {
		limits.MaxDepth = DefaultLimits.MaxDepth
	}
	c.limits = limits
}

var (
	// ErrProtocol reports a peer that broke the protocol.
	ErrProtocol = errors.New("scp: protocol violation")
	// ErrUnsafeName reports a control message naming something that is not a
	// plain file name.
	ErrUnsafeName = errors.New("scp: unsafe name from server")
	// ErrTooLarge reports a size or count above the limits.
	ErrTooLarge = errors.New("scp: transfer exceeds limits")
)

// RemoteError is a message the peer sent with status 1 (warning) or 2 (fatal).
type RemoteError struct {
	Fatal   bool
	Message string
}

func (err *RemoteError) Error() string { return "scp: remote: " + err.Message }

// Is maps the usual messages onto the os errors callers test for.
func (err *RemoteError) Is(target error) bool {
	lower := strings.ToLower(err.Message)
	switch target {
	case os.ErrNotExist:
		return strings.Contains(lower, "no such file")
	case os.ErrPermission:
		return strings.Contains(lower, "permission denied")
	case os.ErrExist:
		return strings.Contains(lower, "file exists")
	}
	return false
}

// Quote returns path as one word for a POSIX shell. NUL and newline cannot be
// carried safely and are refused.
func Quote(path string) (string, error) {
	if path == "" {
		return "", errors.New("scp: empty path")
	}
	if strings.ContainsAny(path, "\x00\n\r") {
		return "", errors.New("scp: path has a control character")
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'", nil
}

// maxControlLine bounds one control message.
const maxControlLine = 4096

// control is a parsed C, D, T or E message.
type control struct {
	kind  byte // 'C', 'D', 'E' or 'T'
	mode  fs.FileMode
	size  int64
	name  string
	mtime time.Time
}

// parseControl parses one control message without its newline.
func parseControl(line []byte) (control, error) {
	if len(line) == 0 {
		return control{}, fmt.Errorf("%w: empty message", ErrProtocol)
	}
	switch line[0] {
	case 'E':
		if len(line) != 1 {
			return control{}, fmt.Errorf("%w: junk after E", ErrProtocol)
		}
		return control{kind: 'E'}, nil
	case 'T':
		fields := strings.Split(string(line[1:]), " ")
		if len(fields) != 4 {
			return control{}, fmt.Errorf("%w: bad T message", ErrProtocol)
		}
		mtime, err := parseDecimal(fields[0], 1<<40)
		if err != nil {
			return control{}, fmt.Errorf("%w: bad time", ErrProtocol)
		}
		if _, err := parseDecimal(fields[1], 999_999); err != nil {
			return control{}, fmt.Errorf("%w: bad time", ErrProtocol)
		}
		if _, err := parseDecimal(fields[2], 1<<40); err != nil {
			return control{}, fmt.Errorf("%w: bad time", ErrProtocol)
		}
		if _, err := parseDecimal(fields[3], 999_999); err != nil {
			return control{}, fmt.Errorf("%w: bad time", ErrProtocol)
		}
		return control{kind: 'T', mtime: time.Unix(mtime, 0)}, nil
	case 'C', 'D':
		// Cmmmm size name
		rest := string(line[1:])
		modeText, rest, ok := strings.Cut(rest, " ")
		if !ok || len(modeText) < 3 || len(modeText) > 4 {
			return control{}, fmt.Errorf("%w: bad mode", ErrProtocol)
		}
		var mode int64
		for _, r := range modeText {
			if r < '0' || r > '7' {
				return control{}, fmt.Errorf("%w: bad mode", ErrProtocol)
			}
			mode = mode<<3 | int64(r-'0')
		}
		sizeText, name, ok := strings.Cut(rest, " ")
		if !ok {
			return control{}, fmt.Errorf("%w: no name", ErrProtocol)
		}
		size, err := parseDecimal(sizeText, 1<<62)
		if err != nil {
			return control{}, fmt.Errorf("%w: bad size", ErrProtocol)
		}
		if err := checkName(name); err != nil {
			return control{}, err
		}
		result := control{kind: line[0], mode: fs.FileMode(mode & 0o777), size: size, name: name}
		if line[0] == 'D' {
			result.mode |= fs.ModeDir
		}
		return result, nil
	}
	return control{}, fmt.Errorf("%w: unknown message %q", ErrProtocol, line[0])
}

// parseDecimal reads digits only, no sign, no space, at most limit.
func parseDecimal(text string, limit int64) (int64, error) {
	if text == "" || len(text) > 19 {
		return 0, ErrProtocol
	}
	var value int64
	for _, r := range text {
		if r < '0' || r > '9' {
			return 0, ErrProtocol
		}
		value = value*10 + int64(r-'0')
		if value > limit {
			return 0, ErrTooLarge
		}
	}
	return value, nil
}

// checkName accepts a single path component.
func checkName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00\n") {
		return fmt.Errorf("%w: %q", ErrUnsafeName, name)
	}
	return nil
}

// peer wraps a channel with the framing helpers both directions use.
type peer struct {
	ch     Channel
	reader *bufio.Reader
}

func newPeer(ch Channel) *peer { return &peer{ch: ch, reader: bufio.NewReaderSize(ch, 32*1024)} }

func (p *peer) ack() error {
	_, err := p.ch.Write([]byte{0})
	return err
}

// readLine reads the rest of a control line, bounded.
func (p *peer) readLine() ([]byte, error) {
	var line []byte
	for {
		b, err := p.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
		if b == '\n' {
			return line, nil
		}
		if len(line) >= maxControlLine {
			return nil, fmt.Errorf("%w: control message too long", ErrProtocol)
		}
		line = append(line, b)
	}
}

// remoteError reads the message that follows a status byte 1 or 2.
func (p *peer) remoteError(fatal bool) error {
	line, err := p.readLine()
	if err != nil {
		return err
	}
	return &RemoteError{Fatal: fatal, Message: sanitize(string(line))}
}

// sanitize keeps a peer's text printable and short before it reaches a UI.
func sanitize(text string) string {
	var out strings.Builder
	for _, r := range text {
		if out.Len() >= 300 {
			out.WriteString("…")
			break
		}
		if r < ' ' || r == 0x7f {
			r = ' '
		}
		out.WriteRune(r)
	}
	return strings.TrimSpace(out.String())
}

// readStatus reads the acknowledgement of a message or of a file's data.
func (p *peer) readStatus() error {
	b, err := p.reader.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	switch b {
	case 0:
		return nil
	case 1, 2:
		return p.remoteError(b == 2)
	}
	return fmt.Errorf("%w: unexpected byte %#x", ErrProtocol, b)
}

var copyBuffers = sync.Pool{New: func() any { return new([64 * 1024]byte) }}

// finish ends a transfer: on success it waits for the command's exit status; on
// failure it aborts the command.
func finish(ch Channel, err error) error {
	if err != nil {
		_ = ch.Close()
		return err
	}
	_ = ch.CloseWrite()
	if waitErr := ch.Wait(); waitErr != nil {
		if tail := sanitize(ch.Stderr()); tail != "" {
			return &RemoteError{Fatal: true, Message: tail}
		}
		return fmt.Errorf("scp: %w", waitErr)
	}
	return ch.Close()
}
