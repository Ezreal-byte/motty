package sftpclient

import (
	"errors"
	"os"
	"path"
	"testing"
	"time"

	"github.com/pkg/sftp"
)

type nativeDirectoryRemote struct {
	*fakeRemote
	created string
	failure error
}

func (r *nativeDirectoryRemote) MkdirAll(directory string) error {
	r.created = directory
	return r.failure
}

func TestCreateDirectoriesQueuesNativeRecursiveCreation(t *testing.T) {
	for _, failure := range []error{nil, os.ErrPermission} {
		backend := &nativeDirectoryRemote{fakeRemote: &fakeRemote{closed: make(chan struct{})}, failure: failure}
		worker := NewWorker(func() (Remote, error) { return backend, nil }, nil)
		if !worker.CreateDirectories("/srv/new parent/中文") {
			t.Fatal("creation was not queued")
		}
		select {
		case result := <-worker.Results():
			if result.Operation != OperationCreateDirectory || result.Path != "/srv/new parent/中文" || !errors.Is(result.Err, failure) || backend.created != result.Path {
				t.Fatalf("unexpected recursive creation result: %+v", result)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("creation did not finish")
		}
		worker.Close()
		<-worker.Done()
	}
}

type basicDirectoryRemote struct {
	*fakeRemote
	directories map[string]bool
	created     []string
	denied      string
	file        string
	rawStatus   bool
}

func (r *basicDirectoryRemote) ReadDir(directory string) ([]os.FileInfo, error) {
	if directory == r.denied {
		return nil, os.ErrPermission
	}
	if directory == r.file {
		return nil, os.ErrInvalid
	}
	if !r.directories[directory] {
		if r.rawStatus {
			return nil, &sftp.StatusError{Code: uint32(sftp.ErrSSHFxNoSuchFile)}
		}
		return nil, os.ErrNotExist
	}
	return nil, nil
}
func (r *basicDirectoryRemote) Mkdir(directory string) error {
	if !r.directories[path.Dir(directory)] {
		return os.ErrNotExist
	}
	r.created = append(r.created, directory)
	r.directories[directory] = true
	return nil
}

func TestCreateDirectoriesFallbackPreservesParentsAndRejectsFilesAndPermissions(t *testing.T) {
	for _, kind := range []string{"missing", "status", "exists", "file", "permission"} {
		t.Run(kind, func(t *testing.T) {
			r := &basicDirectoryRemote{fakeRemote: &fakeRemote{}, directories: map[string]bool{"/srv": true}}
			switch kind {
			case "status":
				r.rawStatus = true
			case "exists":
				r.directories["/srv/new"] = true
				r.directories["/srv/new/leaf"] = true
			case "file":
				r.file = "/srv/new"
			case "permission":
				r.denied = "/srv/new"
			}
			err := createDirectories(r, "/srv/new/leaf")
			if kind == "file" || kind == "permission" {
				if err == nil || len(r.created) != 0 {
					t.Fatal("a file or permission error was treated as a missing directory")
				}
				return
			}
			if err != nil || !r.directories["/srv/new/leaf"] {
				t.Fatalf("tree creation failed: %v", err)
			}
			if kind == "exists" && len(r.created) != 0 {
				t.Fatal("existing directories were recreated")
			}
			if kind != "exists" && (len(r.created) != 2 || r.created[0] != "/srv/new" || r.created[1] != "/srv/new/leaf") {
				t.Fatal("missing parents were not created in order")
			}
		})
	}
}
