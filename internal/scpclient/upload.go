package scpclient

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"
)

// Sender writes entries to a sink-mode scp. It is valid only inside the
// callback of Upload.
type Sender struct {
	p     *peer
	depth int
	limit Limits
}

// ErrSourceChanged reports a local file that ended before its announced size.
var ErrSourceChanged = errors.New("scp: file is shorter than announced")

// Upload runs `scp -t` on directory and lets send place entries in it. With
// recursive the sink accepts directories. Every entry is created inside
// directory under the name given here, whatever the path of the file locally.
func (c *Client) Upload(directory string, recursive bool, send func(*Sender) error) error {
	quoted, err := Quote(directory)
	if err != nil {
		return err
	}
	command := "scp -t -p -- " + quoted
	if recursive {
		command = "scp -t -r -p -- " + quoted
	}
	ch, err := c.start(command)
	if err != nil {
		return err
	}
	p := newPeer(ch)
	err = p.readStatus() // the sink says it is ready
	if err == nil {
		sender := &Sender{p: p, limit: c.limits}
		err = send(sender)
		for err == nil && sender.depth > 0 {
			err = sender.LeaveDir()
		}
	}
	return finish(ch, err)
}

func checkSendName(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	return nil
}

func (s *Sender) times(mtime time.Time) error {
	if mtime.IsZero() || mtime.Unix() < 0 {
		return nil
	}
	if _, err := fmt.Fprintf(s.p.ch, "T%d 0 %d 0\n", mtime.Unix(), mtime.Unix()); err != nil {
		return err
	}
	return s.p.readStatus()
}

// EnterDir creates a directory in the current one and makes it current.
func (s *Sender) EnterDir(name string, mode fs.FileMode, mtime time.Time) error {
	if err := checkSendName(name); err != nil {
		return err
	}
	if s.depth >= s.limit.MaxDepth {
		return fmt.Errorf("%w: deeper than %d directories", ErrTooLarge, s.limit.MaxDepth)
	}
	if err := s.times(mtime); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.p.ch, "D%04o 0 %s\n", mode.Perm(), name); err != nil {
		return err
	}
	if err := s.p.readStatus(); err != nil {
		return err
	}
	s.depth++
	return nil
}

// LeaveDir goes back to the parent directory.
func (s *Sender) LeaveDir() error {
	if s.depth == 0 {
		return errors.New("scp: no directory to leave")
	}
	if _, err := io.WriteString(s.p.ch, "E\n"); err != nil {
		return err
	}
	s.depth--
	return s.p.readStatus()
}

// File sends size bytes read from r as a file called name in the current
// directory. A reader that ends early aborts the transfer: the protocol has no
// way to take back an announced size.
func (s *Sender) File(name string, mode fs.FileMode, mtime time.Time, size int64, r io.Reader) error {
	if err := checkSendName(name); err != nil {
		return err
	}
	if size < 0 || size > s.limit.MaxFileSize {
		return fmt.Errorf("%w: %d bytes", ErrTooLarge, size)
	}
	if err := s.times(mtime); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.p.ch, "C%04o %d %s\n", mode.Perm(), size, name); err != nil {
		return err
	}
	if err := s.p.readStatus(); err != nil {
		return err
	}
	buffer := copyBuffers.Get().(*[64 * 1024]byte)
	defer copyBuffers.Put(buffer)
	remaining := size
	for remaining > 0 {
		chunk := buffer[:min(int64(len(buffer)), remaining)]
		read, readErr := r.Read(chunk)
		if read > 0 {
			if _, err := s.p.ch.Write(chunk[:read]); err != nil {
				return err
			}
			remaining -= int64(read)
		}
		if readErr != nil && remaining > 0 {
			if errors.Is(readErr, io.EOF) {
				return fmt.Errorf("%w: %q", ErrSourceChanged, name)
			}
			return readErr
		}
	}
	if _, err := s.p.ch.Write([]byte{0}); err != nil {
		return err
	}
	return s.p.readStatus()
}

// Run executes one fixed shell command and returns its standard output, at
// most limit bytes. A command that writes more fails with ErrTooLarge.
func (c *Client) Run(command string, limit int) ([]byte, error) {
	ch, err := c.start(command)
	if err != nil {
		return nil, err
	}
	_ = ch.CloseWrite()
	data, err := io.ReadAll(io.LimitReader(ch, int64(limit)+1))
	if err == nil && len(data) > limit {
		err = ErrTooLarge
	}
	if err != nil {
		_ = ch.Close()
		return nil, err
	}
	if waitErr := ch.Wait(); waitErr != nil {
		_ = ch.Close()
		message := sanitize(ch.Stderr())
		if message == "" {
			message = waitErr.Error()
		}
		return data, &RemoteError{Fatal: true, Message: message}
	}
	return data, ch.Close()
}

// quoteAll quotes each argument, or fails.
func quoteAll(args ...string) ([]string, error) {
	quoted := make([]string, len(args))
	for index, arg := range args {
		value, err := Quote(arg)
		if err != nil {
			return nil, err
		}
		quoted[index] = value
	}
	return quoted, nil
}

// Command builds a shell command from a fixed program text and quoted path
// arguments: each "{}" in format is replaced by the next argument.
func Command(format string, args ...string) (string, error) {
	quoted, err := quoteAll(args...)
	if err != nil {
		return "", err
	}
	if strings.Count(format, "{}") != len(quoted) {
		return "", errors.New("scp: command argument count")
	}
	parts := strings.Split(format, "{}")
	var out strings.Builder
	for index, part := range parts {
		out.WriteString(part)
		if index < len(quoted) {
			out.WriteString(quoted[index])
		}
	}
	return out.String(), nil
}
