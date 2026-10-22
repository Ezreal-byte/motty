package sftpclient

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// ErrUnsupported reports a Remote that cannot do a write operation.
var ErrUnsupported = errors.New("sftp: operation not supported by this connection")

// ErrExists reports that WriteFile would replace a file it was not allowed to.
var ErrExists = errors.New("sftp: file already exists")

// ReadAll reads at most limit bytes of a remote file and reports whether the
// file was longer.
func ReadAll(r Remote, path string, limit int) (data []byte, truncated bool, err error) {
	file, err := r.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	data, err = io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > limit {
		return data[:limit], true, nil
	}
	return data, false, nil
}

// WriteFile writes content to a remote file. Without overwrite an existing file
// is left alone and ErrExists is returned.
func WriteFile(r Remote, path string, content []byte, overwrite bool) error {
	t, ok := r.(transferRemote)
	if !ok {
		return ErrUnsupported
	}
	flags := os.O_WRONLY | os.O_CREATE
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := t.OpenTransferFile(path, flags)
	if err != nil {
		if !overwrite {
			if _, statErr := t.Lstat(path); statErr == nil {
				return fmt.Errorf("%w: %s", ErrExists, path)
			}
		}
		return err
	}
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// Delete removes a remote file or empty directory.
func Delete(r Remote, path string) error {
	t, ok := r.(transferRemote)
	if !ok {
		return ErrUnsupported
	}
	return t.Remove(path)
}
