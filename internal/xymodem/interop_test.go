//go:build !windows

package xymodem

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// These run the engines against lrzsz's sx, rx, sb and rb. They are skipped
// where lrzsz is not installed.

type proc struct {
	in     io.WriteCloser
	chunks chan []byte
	wait   chan struct{}
}

func start(t *testing.T, dir, name string, args ...string) *proc {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is not installed", name)
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &proc{in: in, chunks: make(chan []byte, 64), wait: make(chan struct{})}
	go func() {
		defer close(p.chunks)
		buffer := make([]byte, 4096)
		for {
			n, err := out.Read(buffer)
			if n > 0 {
				p.chunks <- append([]byte(nil), buffer[:n]...)
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { cmd.Wait(); close(p.wait) }()
	t.Cleanup(func() { cmd.Process.Kill() })
	return p
}

func (p *proc) pump(t *testing.T, e engine) {
	t.Helper()
	deadline := time.After(60 * time.Second)
	idle := time.NewTimer(2 * time.Second)
	defer idle.Stop()
	flush := func() {
		if out := e.TakeOutput(nil); len(out) > 0 {
			p.in.Write(out)
		}
	}
	flush()
	for !e.Finished() {
		select {
		case chunk, ok := <-p.chunks:
			if !ok {
				if !e.Finished() {
					t.Fatalf("the program ended first (engine err %v)", e.Err())
				}
				return
			}
			e.Feed(chunk)
			flush()
			idle.Reset(2 * time.Second)
		case <-idle.C:
			e.Timeout()
			flush()
			idle.Reset(2 * time.Second)
		case <-deadline:
			t.Fatalf("timed out; err %v", e.Err())
		}
	}
}

func TestReceivingFromLrzszSxAndSb(t *testing.T) {
	random := rand.New(rand.NewSource(11))
	for _, test := range []struct {
		program string
		args    []string
		mode    Mode
	}{
		{"sx", []string{"--quiet"}, XMODEM},
		{"sx", []string{"--quiet", "--1k"}, XMODEM1K},
		{"sb", []string{"--quiet"}, YMODEM},
	} {
		t.Run(test.program+describe(test.mode), func(t *testing.T) {
			dir := t.TempDir()
			data := make([]byte, 33333)
			random.Read(data)
			if test.mode != YMODEM {
				// The padding of an XMODEM file cannot be told from the file, so the
				// data must not end in SUB.
				data[len(data)-1] = 'x'
			}
			file := filepath.Join(dir, "in.bin")
			os.WriteFile(file, data, 0o644)
			target := &collect{}
			receiver := NewReceiver(test.mode, "got.bin", target)
			p := start(t, dir, test.program, append(test.args, file)...)
			p.pump(t, receiver)
			if receiver.Err() != nil {
				t.Fatalf("receiver: %v", receiver.Err())
			}
			name := "got.bin"
			if test.mode == YMODEM {
				name = "in.bin"
			}
			if got := target.files[name]; got == nil || !bytes.Equal(got.Bytes(), data) {
				t.Fatalf("%s differs (%v)", name, target.order)
			}
		})
	}
}

func TestSendingToLrzszRxAndRb(t *testing.T) {
	random := rand.New(rand.NewSource(12))
	for _, test := range []struct {
		program string
		args    []string
		mode    Mode
	}{
		{"rx", []string{"--quiet", "--overwrite", "got.bin"}, XMODEM},
		{"rx", []string{"--quiet", "--overwrite", "got.bin"}, XMODEM1K},
		{"rb", []string{"--quiet", "--overwrite"}, YMODEM},
	} {
		t.Run(test.program+describe(test.mode), func(t *testing.T) {
			dir := t.TempDir()
			list := files(random, 20000)
			list[0].Name = "got.bin"
			p := start(t, dir, test.program, test.args...)
			sender := NewSender(test.mode, list)
			p.pump(t, sender)
			if sender.Err() != nil {
				t.Fatalf("sender: %v", sender.Err())
			}
			select {
			case <-p.wait:
			case <-time.After(5 * time.Second):
				t.Fatal("the program did not exit")
			}
			got, err := os.ReadFile(filepath.Join(dir, "got.bin"))
			want := content(list[0])
			if test.mode != YMODEM {
				// XMODEM pads the last block; the receiver cannot know the length.
				got = bytes.TrimRight(got, "\x1a")
				want = bytes.TrimRight(want, "\x1a")
			}
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("file is %d bytes (%v), want %d", len(got), err, len(want))
			}
		})
	}
}
