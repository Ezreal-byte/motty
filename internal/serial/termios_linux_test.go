//go:build linux

package serial

import "golang.org/x/sys/unix"

type termiosFlag = uint32

func getTermios(fd int) (*unix.Termios, error) { return unix.IoctlGetTermios(fd, unix.TCGETS2) }
