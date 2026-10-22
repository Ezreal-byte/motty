package sftpclient

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type fakeInfo struct {
	name string
	size int64
	mode os.FileMode
}

func (info fakeInfo) Name() string       { return info.name }
func (info fakeInfo) Size() int64        { return info.size }
func (info fakeInfo) Mode() os.FileMode  { return info.mode }
func (info fakeInfo) ModTime() time.Time { return time.Unix(1_700_000_000, 0) }
func (info fakeInfo) IsDir() bool        { return info.mode.IsDir() }
func (info fakeInfo) Sys() any           { return nil }

type fakeRemote struct {
	cwd     string
	infos   []os.FileInfo
	files   map[string][]byte
	chunk   int
	listErr error
	closed  chan struct{}
	opened  chan struct{}
	block   bool
	once    sync.Once
}

func (remote *fakeRemote) Getwd() (string, error) { return remote.cwd, nil }
func (remote *fakeRemote) ReadDir(string) ([]os.FileInfo, error) {
	return remote.infos, remote.listErr
}
func (remote *fakeRemote) Open(path string) (io.ReadCloser, error) {
	content, ok := remote.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	if remote.block {
		close(remote.opened)
		return blockingReadCloser{closed: remote.closed}, nil
	}
	return io.NopCloser(&chunkReader{content: content, chunk: remote.chunk}), nil
}
func (remote *fakeRemote) Close() error {
	remote.once.Do(func() { close(remote.closed) })
	return nil
}

type chunkReader struct {
	content []byte
	chunk   int
}

type blockingReadCloser struct {
	closed <-chan struct{}
}

func (reader blockingReadCloser) Read([]byte) (int, error) {
	<-reader.closed
	return 0, os.ErrClosed
}

func (blockingReadCloser) Close() error { return nil }

func (reader *chunkReader) Read(destination []byte) (int, error) {
	if len(reader.content) == 0 {
		return 0, io.EOF
	}
	count := min(len(destination), len(reader.content))
	if reader.chunk > 0 {
		count = min(count, reader.chunk)
	}
	copy(destination, reader.content[:count])
	reader.content = reader.content[count:]
	return count, nil
}

func waitResult(t *testing.T, worker *Worker) Result {
	t.Helper()
	select {
	case result := <-worker.Results():
		return result
	case <-time.After(time.Second):
		t.Fatal("worker did not complete")
		return Result{}
	}
}

func TestWorkerListsWorkingDirectoryWithDirectoriesFirst(t *testing.T) {
	remote := &fakeRemote{cwd: "/home/test", closed: make(chan struct{}), infos: []os.FileInfo{
		fakeInfo{name: "z.txt", size: 9},
		fakeInfo{name: "src", mode: os.ModeDir | 0o755},
		fakeInfo{name: "A.txt", size: 3},
	}}
	worker := NewWorker(func() (Remote, error) { return remote, nil }, nil)
	defer worker.Close()
	if !worker.List("") {
		t.Fatal("list was not queued")
	}
	result := waitResult(t, worker)
	if result.Err != nil || result.Path != "/home/test" {
		t.Fatalf("result = %+v", result)
	}
	want := []string{"src", "A.txt", "z.txt"}
	for index, entry := range result.Entries {
		if entry.Name != want[index] {
			t.Fatalf("entries = %+v", result.Entries)
		}
	}
}

func TestWorkerDownloadsEveryChunkBoundary(t *testing.T) {
	content := []byte("alpha\x00beta\ngamma")
	for _, chunk := range []int{1, 2, 3, 7, len(content)} {
		t.Run(string(rune('a'+chunk)), func(t *testing.T) {
			remote := &fakeRemote{closed: make(chan struct{}), files: map[string][]byte{"/remote/data.bin": content}, chunk: chunk}
			worker := NewWorker(func() (Remote, error) { return remote, nil }, nil)
			defer worker.Close()
			target := filepath.Join(t.TempDir(), "data.bin")
			if !worker.Download("/remote/data.bin", target) {
				t.Fatal("download was not queued")
			}
			result := waitResult(t, worker)
			if result.Err != nil || result.Bytes != int64(len(content)) {
				t.Fatalf("result = %+v", result)
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, content) {
				t.Fatalf("download = %q, %v", got, err)
			}
		})
	}
}

func TestWorkerReportsOpenAndListErrors(t *testing.T) {
	want := errors.New("subsystem unavailable")
	worker := NewWorker(func() (Remote, error) { return nil, want }, nil)
	defer worker.Close()
	worker.List("/")
	if result := waitResult(t, worker); !errors.Is(result.Err, want) {
		t.Fatalf("open error = %v", result.Err)
	}

	remote := &fakeRemote{closed: make(chan struct{}), listErr: os.ErrPermission}
	worker.Close()
	<-worker.Done()
	worker = NewWorker(func() (Remote, error) { return remote, nil }, nil)
	defer worker.Close()
	worker.List("/root")
	if result := waitResult(t, worker); !errors.Is(result.Err, os.ErrPermission) {
		t.Fatalf("list error = %v", result.Err)
	}
}

func TestWorkerCancelInterruptsDownloadAndRemovesPartialFile(t *testing.T) {
	remote := &fakeRemote{
		closed: make(chan struct{}), opened: make(chan struct{}), block: true,
		files: map[string][]byte{"/remote/blocked.bin": []byte("unused")},
	}
	worker := NewWorker(func() (Remote, error) { return remote, nil }, nil)
	defer worker.Close()
	target := filepath.Join(t.TempDir(), "blocked.bin")
	if !worker.Download("/remote/blocked.bin", target) {
		t.Fatal("download was not queued")
	}
	select {
	case <-remote.opened:
	case <-time.After(time.Second):
		t.Fatal("download did not start")
	}
	worker.Cancel()
	if result := waitResult(t, worker); !errors.Is(result.Err, ErrCanceled) {
		t.Fatalf("cancel result = %+v", result)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial target remains: %v", err)
	}
}

func BenchmarkEntriesFrom(b *testing.B) {
	infos := make([]os.FileInfo, 128)
	for index := range infos {
		mode := os.FileMode(0o644)
		if index%8 == 0 {
			mode = os.ModeDir | 0o755
		}
		infos[index] = fakeInfo{name: "entry-" + string(rune(index+32)), size: int64(index), mode: mode}
	}
	b.ReportAllocs()
	for b.Loop() {
		if entries := entriesFrom(infos); len(entries) != len(infos) {
			b.Fatal(len(entries))
		}
	}
}
