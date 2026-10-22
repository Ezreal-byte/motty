//go:build darwin

package serial

import "golang.org/x/sys/unix"

type termiosFlag = uint64

func getTermios(fd int) (*unix.Termios, error) { return unix.IoctlGetTermios(fd, unix.TIOCGETA) }
