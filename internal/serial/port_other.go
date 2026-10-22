//go:build !linux && !darwin && !windows

package serial

import (
	"errors"
	"io"
)

func open(string, Settings) (io.ReadWriteCloser, error) {
	return nil, errors.New("serial: ports are not supported on this system")
}

// Ports lists nothing here.
func Ports() []Port { return nil }
