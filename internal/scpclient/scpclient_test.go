package scpclient

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"
)

// fakeChannel is a command whose far end is a script running in the test.
type fakeChannel struct {
	toClient   *io.PipeWriter // what the peer sends
	fromClient *io.PipeReader // what the client sent
	clientIn   *io.PipeReader
	clientOut  *io.PipeWriter
	stderr     string
	waitErr    error
	closed     chan struct{}
}

func (c *fakeChannel) Read(p []byte) (int, error)  { return c.clientIn.Read(p) }
func (c *fakeChannel) Write(p []byte) (int, error) { return c.clientOut.Write(p) }
func (c *fakeChannel) CloseWrite() error           { return c.clientOut.Close() }
func (c *fakeChannel) Wait() error                 { return c.waitErr }
func (c *fakeChannel) Stderr() string              { return c.stderr }
func (c *fakeChannel) Close() error {
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
	c.clientOut.CloseWithError(io.ErrClosedPipe)
	c.clientIn.CloseWithError(io.ErrClosedPipe)
	return nil
}

// peer is the script's side of a fakeChannel.
type scriptPeer struct {
	out *io.PipeWriter
	in  *bufio.Reader
	ch  *fakeChannel
}

func (p *scriptPeer) send(text string) { io.WriteString(p.out, text) }
func (p *scriptPeer) ack() byte {
	b, err := p.in.ReadByte()
	if err != nil {
		return 0xff
	}
	return b
}
func (p *scriptPeer) line() string {
	text, _ := p.in.ReadString('\n')
	return strings.TrimSuffix(text, "\n")
}
func (p *scriptPeer) end() { p.out.Close() }

// newClient returns a client whose commands are answered by script. The
// command line the client started is reported on commands.
func newClient(script func(command string, p *scriptPeer)) (*Client, *[]string) {
	var commands []string
	return New(func(command string) (Channel, error) {
		commands = append(commands, command)
		toClientR, toClientW := io.Pipe()
		fromClientR, fromClientW := io.Pipe()
		ch := &fakeChannel{toClient: toClientW, fromClient: fromClientR, clientIn: toClientR, clientOut: fromClientW, closed: make(chan struct{})}
		peer := &scriptPeer{out: toClientW, in: bufio.NewReader(fromClientR), ch: ch}
		go func() {
			script(command, peer)
		}()
		return ch, nil
	}), &commands
}

type memHandler struct {
	dirs  []string
	files map[string]string
	times map[string]time.Time
	modes map[string]fs.FileMode
	order []string
}

type memFile struct {
	h    *memHandler
	name string
	buf  bytes.Buffer
}

func (f *memFile) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *memFile) Commit() error {
	f.h.files[f.name] = f.buf.String()
	return nil
}
func (f *memFile) Abort() {}

func newMem() *memHandler {
	return &memHandler{files: map[string]string{}, times: map[string]time.Time{}, modes: map[string]fs.FileMode{}}
}
func (h *memHandler) Dir(rel string, mode fs.FileMode, mtime time.Time) error {
	h.dirs = append(h.dirs, rel)
	h.order = append(h.order, "D:"+rel)
	h.modes[rel] = mode
	return nil
}
func (h *memHandler) File(rel string, mode fs.FileMode, mtime time.Time, size int64) (FileWriter, error) {
	h.order = append(h.order, "F:"+rel)
	h.times[rel], h.modes[rel] = mtime, mode
	return &memFile{h: h, name: rel}, nil
}

func TestDownloadTree(t *testing.T) {
	client, commands := newClient(func(_ string, p *scriptPeer) {
		defer p.end()
		p.ack() // ready
		p.send("T1700000000 0 1700000001 0\n")
		p.ack()
		p.send("D0755 0 root\n")
		p.ack()
		p.send("T1700000100 0 1700000100 0\n")
		p.ack()
		p.send("C0644 5 a.txt\n")
		p.ack()
		p.send("hello\x00")
		p.ack()
		p.send("D0700 0 sub dir\n")
		p.ack()
		p.send("C0600 3 b\n")
		p.ack()
		p.send("xyz\x00")
		p.ack()
		p.send("E\n")
		p.ack()
		p.send("E\n")
		p.ack()
	})
	handler := newMem()
	stats, err := client.Download("/srv/it's here", true, handler)
	if err != nil {
		t.Fatal(err)
	}
	if want := `scp -f -r -p -- '/srv/it'\''s here'`; (*commands)[0] != want {
		t.Fatalf("command = %q, want %q", (*commands)[0], want)
	}
	if stats.Files != 2 || stats.Bytes != 8 {
		t.Fatalf("stats = %+v", stats)
	}
	if handler.files["a.txt"] != "hello" || handler.files["sub dir/b"] != "xyz" {
		t.Fatalf("files = %v order = %v", handler.files, handler.order)
	}
	if handler.dirs[0] != "." || handler.dirs[1] != "sub dir" {
		t.Fatalf("dirs = %v, want the root as \".\" and its children by their path from it", handler.dirs)
	}
	if got := handler.times["a.txt"]; got.Unix() != 1700000100 {
		t.Fatalf("mtime = %v", got)
	}
}

func TestDownloadSingleFile(t *testing.T) {
	client, _ := newClient(func(_ string, p *scriptPeer) {
		defer p.end()
		p.ack()
		p.send("C0640 4 whatever\n")
		p.ack()
		p.send("data\x00")
		p.ack()
	})
	handler := newMem()
	if _, err := client.Download("/etc/motd", false, handler); err != nil {
		t.Fatal(err)
	}
	if handler.files["."] != "data" {
		t.Fatalf("files = %v", handler.files)
	}
}

// A malicious server must not be able to name anything but a plain file.
func TestDownloadRefusesUnsafeNames(t *testing.T) {
	for _, name := range []string{"../x", "a/b", "..", ".", "/etc/passwd", "x/../../y"} {
		t.Run(name, func(t *testing.T) {
			client, _ := newClient(func(_ string, p *scriptPeer) {
				defer p.end()
				p.ack()
				p.send("D0755 0 root\n")
				p.ack()
				p.send("C0644 1 " + name + "\n")
				p.ack()
			})
			handler := newMem()
			_, err := client.Download("/srv", true, handler)
			if !errors.Is(err, ErrUnsafeName) {
				t.Fatalf("err = %v, want unsafe name", err)
			}
			if len(handler.files) != 0 {
				t.Fatalf("a file was created: %v", handler.files)
			}
		})
	}
	// The same names as directories.
	client, _ := newClient(func(_ string, p *scriptPeer) {
		defer p.end()
		p.ack()
		p.send("D0755 0 ..\n")
		p.ack()
	})
	if _, err := client.Download("/srv", true, newMem()); !errors.Is(err, ErrUnsafeName) {
		t.Fatalf("directory: err = %v", err)
	}
}

func TestDownloadRefusesBadSizes(t *testing.T) {
	cases := map[string]struct {
		message string
		want    error
	}{
		"huge":       {"C0644 4611686018427387904 f\n", ErrTooLarge},
		"overflow":   {"C0644 99999999999999999999 f\n", ErrProtocol},
		"negative":   {"C0644 -1 f\n", ErrProtocol},
		"over limit": {"C0644 2000000 f\n", ErrTooLarge},
		"bad mode":   {"C0888 1 f\n", ErrProtocol},
		"no name":    {"C0644 1\n", ErrProtocol},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			client, _ := newClient(func(_ string, p *scriptPeer) {
				defer p.end()
				p.ack()
				p.send(tc.message)
				p.ack()
			})
			client.SetLimits(Limits{MaxFileSize: 1 << 20})
			handler := newMem()
			if _, err := client.Download("/f", false, handler); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if len(handler.order) != 0 {
				t.Fatalf("the handler was reached: %v", handler.order)
			}
		})
	}
}

func TestDownloadDetectsWrongLength(t *testing.T) {
	t.Run("truncated", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			p.send("C0644 10 f\n")
			p.ack()
			p.send("four")
		})
		if _, err := client.Download("/f", false, newMem()); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("longer than announced", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			p.send("C0644 3 f\n")
			p.ack()
			p.send("abcdef\x00")
			p.ack()
		})
		handler := newMem()
		_, err := client.Download("/f", false, handler)
		if err == nil || handler.files["."] == "abcdef" {
			t.Fatalf("err = %v files = %v", err, handler.files)
		}
	})
	t.Run("no final status", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			p.send("C0644 3 f\n")
			p.ack()
			p.send("abc")
		})
		if _, err := client.Download("/f", false, newMem()); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("truncated directory", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			p.send("D0755 0 d\n")
			p.ack()
		})
		if _, err := client.Download("/d", true, newMem()); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestDownloadRefusesMoreThanRequested(t *testing.T) {
	t.Run("second file", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			p.send("C0644 1 a\n")
			p.ack()
			p.send("x\x00")
			p.ack()
			p.send("C0644 1 b\n")
			p.ack()
			p.send("y\x00")
			p.ack()
		})
		handler := newMem()
		if _, err := client.Download("/a", false, handler); !errors.Is(err, ErrProtocol) {
			t.Fatalf("err = %v", err)
		}
		if _, second := handler.files["b"]; second {
			t.Fatal("the unrequested file was written")
		}
	})
	t.Run("directory for a file", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			p.send("D0755 0 d\n")
			p.ack()
		})
		if _, err := client.Download("/d", false, newMem()); !errors.Is(err, ErrProtocol) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("E without D", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			p.send("E\n")
			p.ack()
		})
		if _, err := client.Download("/d", true, newMem()); !errors.Is(err, ErrProtocol) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("too many entries", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			p.send("D0755 0 d\n")
			p.ack()
			for index := 0; index < 5; index++ {
				p.send(fmt.Sprintf("C0644 0 f%d\n", index))
				p.ack()
				p.send("\x00")
				p.ack()
			}
		})
		client.SetLimits(Limits{MaxEntries: 3})
		if _, err := client.Download("/d", true, newMem()); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("too deep", func(t *testing.T) {
		client, _ := newClient(func(_ string, p *scriptPeer) {
			defer p.end()
			p.ack()
			for index := 0; index < 10; index++ {
				p.send("D0755 0 d\n")
				p.ack()
			}
		})
		client.SetLimits(Limits{MaxDepth: 4})
		if _, err := client.Download("/d", true, newMem()); !errors.Is(err, ErrTooLarge) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestDownloadReportsRemoteMessages(t *testing.T) {
	client, _ := newClient(func(_ string, p *scriptPeer) {
		defer p.end()
		p.ack()
		p.send("\x01scp: /nope: No such file or directory\x1b[31m\n")
	})
	_, err := client.Download("/nope", false, newMem())
	var remote *RemoteError
	if !errors.As(err, &remote) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
	if strings.ContainsRune(remote.Message, 0x1b) {
		t.Fatalf("control character reached the message: %q", remote.Message)
	}
}

func TestDownloadStopsWhenHandlerFails(t *testing.T) {
	client, _ := newClient(func(_ string, p *scriptPeer) {
		defer p.end()
		p.ack()
		p.send("C0644 1 a\n")
		p.ack()
	})
	boom := errors.New("disk full")
	_, err := client.Download("/a", false, failingHandler{boom})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}

type failingHandler struct{ err error }

func (h failingHandler) Dir(string, fs.FileMode, time.Time) error { return h.err }
func (h failingHandler) File(string, fs.FileMode, time.Time, int64) (FileWriter, error) {
	return nil, h.err
}

// runSink plays a well-behaved `scp -t`: it acknowledges every message and file.
func runSink(p *scriptPeer, received *[]string, data *string) {
	defer p.end()
	p.out.Write([]byte{0}) // ready
	for {
		text, err := p.in.ReadString('\n')
		if err != nil {
			return
		}
		text = strings.TrimSuffix(text, "\n")
		if received != nil {
			*received = append(*received, text)
		}
		if strings.HasPrefix(text, "C") {
			var size int
			fmt.Sscanf(text[6:], "%d", &size)
			p.out.Write([]byte{0})
			buf := make([]byte, size+1)
			if _, err := io.ReadFull(p.in, buf); err != nil {
				return
			}
			if data != nil {
				*data += string(buf[:size])
			}
		}
		p.out.Write([]byte{0})
	}
}

func TestUpload(t *testing.T) {
	var received []string
	var data string
	client, commands := newClient(func(_ string, p *scriptPeer) { runSink(p, &received, &data) })
	mtime := time.Unix(1700000000, 0)
	err := client.Upload("/srv/in", true, func(s *Sender) error {
		if err := s.EnterDir("tree", 0o755, mtime); err != nil {
			return err
		}
		if err := s.File("a b.txt", 0o640, mtime, 5, strings.NewReader("hello")); err != nil {
			return err
		}
		return nil // the open directory is closed for us
	})
	if err != nil {
		t.Fatal(err)
	}
	if (*commands)[0] != "scp -t -r -p -- '/srv/in'" {
		t.Fatalf("command = %q", (*commands)[0])
	}
	want := []string{"T1700000000 0 1700000000 0", "D0755 0 tree", "T1700000000 0 1700000000 0", "C0640 5 a b.txt", "E"}
	if strings.Join(received, "|") != strings.Join(want, "|") || data != "hello" {
		t.Fatalf("received %q data %q", received, data)
	}
}

func TestUploadRefusesUnsafeNamesAndShortSources(t *testing.T) {
	ready := func(_ string, p *scriptPeer) { runSink(p, nil, nil) }
	client, _ := newClient(ready)
	err := client.Upload("/d", false, func(s *Sender) error {
		return s.File("../escape", 0o644, time.Time{}, 1, strings.NewReader("x"))
	})
	if !errors.Is(err, ErrUnsafeName) {
		t.Fatalf("err = %v", err)
	}
	client, _ = newClient(ready)
	err = client.Upload("/d", false, func(s *Sender) error {
		return s.File("f", 0o644, time.Time{}, 10, strings.NewReader("short"))
	})
	if !errors.Is(err, ErrSourceChanged) {
		t.Fatalf("err = %v", err)
	}
}

func TestUploadReportsSinkRefusal(t *testing.T) {
	client, _ := newClient(func(_ string, p *scriptPeer) {
		defer p.end()
		p.out.Write([]byte{0})
		p.line()
		p.out.Write([]byte("\x01scp: /d/f: Permission denied\n"))
	})
	err := client.Upload("/d", false, func(s *Sender) error {
		return s.File("f", 0o644, time.Time{}, 1, strings.NewReader("x"))
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("err = %v", err)
	}
}

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"/plain":      `'/plain'`,
		"a b":         `'a b'`,
		"it's":        `'it'\''s'`,
		"$(rm -rf /)": `'$(rm -rf /)'`,
		"-rf":         `'-rf'`,
		"`x`;y":       "'`x`;y'",
	}
	for in, want := range cases {
		got, err := Quote(in)
		if err != nil || got != want {
			t.Errorf("Quote(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "a\nb", "a\x00b", "a\rb"} {
		if _, err := Quote(bad); err == nil {
			t.Errorf("Quote(%q) accepted", bad)
		}
	}
	if _, err := Command("rm -- {}", "a\nb"); err == nil {
		t.Error("Command accepted a newline")
	}
}

func TestParseListing(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	gnu := `total 12
drwxr-xr-x 2 1000 1000 4096 1700000000 sub dir
-rw-r--r-- 1 0 0 1234 1700000001 file with  spaces.txt
lrwxrwxrwx 1 0 0 4 1700000002 link -> target dir
-rwsr-xr-x 1 0 0 10 1700000003 su
crw-rw-rw- 1 0 0 1, 3 1700000004 null
-rw-r--r-- 1 0 0 1 1700000005 ..
-rw-r--r-- 1 0 0 1 1700000005 a/b
garbage
`
	items, err := ParseListing([]byte(gnu), now)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, item := range items {
		names = append(names, item.Name)
	}
	if got := strings.Join(names, "|"); got != "sub dir|file with  spaces.txt|link|su|null" {
		t.Fatalf("names = %q", got)
	}
	if !items[0].Mode.IsDir() || items[1].Size != 1234 || items[2].LinkTarget != "target dir" || items[2].Mode&os.ModeSymlink == 0 {
		t.Fatalf("items = %+v", items)
	}
	if items[3].Mode&os.ModeSetuid == 0 || items[0].UID != 1000 {
		t.Fatalf("items = %+v", items)
	}
	busybox := "-rw-r--r--    1 0        0              123 Oct  3 08:15 recent\n-rw-r--r--    1 0        0              123 Dec 25  2019 old\n-rw-r--r-- 1 0 0 5 Dec 30 10:00 future-month\n"
	items, _ = ParseListing([]byte(busybox), now)
	if len(items) != 3 || items[0].ModTime.Month() != 10 || items[1].ModTime.Year() != 2019 || items[2].ModTime.Year() != 2025 {
		t.Fatalf("items = %+v", items)
	}
}

func TestRunBoundsOutput(t *testing.T) {
	client, _ := newClient(func(_ string, p *scriptPeer) {
		defer p.end()
		p.send(strings.Repeat("x", 100))
	})
	if _, err := client.Run("yes", 10); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v", err)
	}
}

func FuzzParseControl(f *testing.F) {
	for _, seed := range []string{"C0644 5 a", "D0755 0 d", "E", "T1 0 2 0", "C0644 -1 x", "C9 9 ..", "D0755 0 a/b", "Cxxxx 1 f"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, line []byte) {
		message, err := parseControl(line)
		if err != nil {
			return
		}
		switch message.kind {
		case 'C', 'D':
			if checkName(message.name) != nil || message.size < 0 || message.mode&^(fs.ModeDir|0o777) != 0 {
				t.Fatalf("accepted %q as %+v", line, message)
			}
		}
	})
}

func FuzzParseListing(f *testing.F) {
	f.Add([]byte("-rw-r--r-- 1 0 0 5 1700000000 name\n"))
	f.Add([]byte("crw-rw-rw- 1 0 0 1, 3 Jan  1 00:00 null"))
	f.Fuzz(func(t *testing.T, out []byte) {
		items, _ := ParseListing(out, time.Unix(1700000000, 0))
		for _, item := range items {
			if checkName(item.Name) != nil || item.Size < 0 {
				t.Fatalf("accepted %+v", item)
			}
		}
	})
}
