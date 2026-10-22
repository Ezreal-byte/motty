//go:build !windows

package zmodem

import (
	"bytes"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// These tests run the engines against lrzsz's sz and rz, the programs that sit at
// the other end of a real transfer. They are skipped where lrzsz is not installed.

// pipeProc runs a program whose standard input and output are the transfer.
type pipeProc struct {
	cmd    *exec.Cmd
	in     io.WriteCloser
	chunks chan []byte
	wait   chan struct{}
}

func startProc(t *testing.T, dir string, name string, args ...string) *pipeProc {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Skipf("%s is not installed", name)
	}
	cmd := exec.Command(path, args...)
	cmd.Dir = dir
	in, _ := cmd.StdinPipe()
	out, _ := cmd.StdoutPipe()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	proc := &pipeProc{cmd: cmd, in: in, chunks: make(chan []byte, 64), wait: make(chan struct{})}
	go func() {
		defer close(proc.chunks)
		buffer := make([]byte, 4096)
		for {
			n, err := out.Read(buffer)
			if n > 0 {
				proc.chunks <- append([]byte(nil), buffer[:n]...)
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { cmd.Wait(); close(proc.wait) }()
	t.Cleanup(func() { cmd.Process.Kill() })
	return proc
}

// pump moves bytes between an engine and the program until the engine settles.
func (proc *pipeProc) pump(t *testing.T, engine Engine) {
	t.Helper()
	var mu sync.Mutex
	deadline := time.After(60 * time.Second)
	idle := time.NewTimer(3 * time.Second)
	defer idle.Stop()
	flush := func() {
		mu.Lock()
		defer mu.Unlock()
		if out := engine.TakeOutput(nil); len(out) > 0 {
			proc.in.Write(out)
		}
	}
	flush()
	for !engine.Finished() || !engine.Settled() {
		select {
		case chunk, ok := <-proc.chunks:
			if !ok {
				if !engine.Finished() {
					t.Fatalf("the program ended before the transfer did (engine err %v)", engine.Err())
				}
				return
			}
			engine.Feed(chunk)
			flush()
			idle.Reset(3 * time.Second)
		case <-idle.C:
			if engine.Finished() {
				return
			}
			engine.Timeout()
			flush()
			idle.Reset(3 * time.Second)
		case <-deadline:
			t.Fatalf("timed out; engine finished=%v err=%v", engine.Finished(), engine.Err())
		}
	}
}

func TestReceivingFromLrzszSz(t *testing.T) {
	dir := t.TempDir()
	random := rand.New(rand.NewSource(7))
	want := map[string][]byte{}
	var names []string
	for _, size := range []int{0, 1, 4095, 100000, 700000} {
		data := make([]byte, size)
		random.Read(data)
		name := filepath.Join(dir, "f"+string(rune('a'+len(names)))+".bin")
		os.WriteFile(name, data, 0o644)
		want[filepath.Base(name)] = data
		names = append(names, name)
	}
	target := &memTarget{}
	receiver := NewReceiver(target)
	proc := startProc(t, dir, "sz", append([]string{"--binary", "--zmodem", "--quiet"}, names...)...)
	proc.pump(t, receiver)
	if receiver.Err() != nil {
		t.Fatalf("receiver: %v", receiver.Err())
	}
	for name, data := range want {
		got := target.files[name]
		if got == nil || !bytes.Equal(got.Bytes(), data) {
			t.Errorf("%s differs from what sz sent", name)
		}
	}
}

func TestSendingToLrzszRz(t *testing.T) {
	dir := t.TempDir()
	random := rand.New(rand.NewSource(8))
	files := testFiles(random, 0, 1, 5000, 123456, 600000)
	receiver := startProc(t, dir, "rz", "--binary", "--zmodem", "--quiet", "--overwrite")
	sender := NewSender(files, SendOptions{})
	// rz speaks first (its ZRINIT), which pump delivers as the first chunk.
	receiver.pump(t, sender)
	if sender.Err() != nil {
		t.Fatalf("sender: %v", sender.Err())
	}
	select {
	case <-receiver.wait:
	case <-time.After(5 * time.Second):
		t.Fatal("rz did not exit")
	}
	for _, file := range files {
		got, err := os.ReadFile(filepath.Join(dir, file.Name))
		if err != nil || !bytes.Equal(got, contentOf(file)) {
			t.Errorf("%s: rz wrote %d bytes (%v), want %d", file.Name, len(got), err, file.Size)
		}
	}
}
