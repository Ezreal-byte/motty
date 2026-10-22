//go:build darwin

package serial

import (
	"errors"

	"golang.org/x/sys/unix"
)

// flush drops what the line still holds in either direction.
func flush(fd int) { _ = unix.IoctlSetPointerInt(fd, unix.TIOCFLUSH, 3) }

// configure puts the line in raw mode at the settings. macOS keeps the speed in
// the termios structure itself, whatever the rate.
func configure(fd int, settings Settings) error {
	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}
	if err := makeRaw(termios, settings); err != nil {
		return err
	}
	termios.Cflag &^= unix.CRTSCTS
	if settings.Flow == FlowHardware {
		termios.Cflag |= unix.CRTSCTS
	}
	termios.Ispeed, termios.Ospeed = uint64(settings.Baud), uint64(settings.Baud)
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, termios)
}

func markSpaceParity(*unix.Termios, Parity) error {
	return errors.New("serial: mark and space parity are not available on macOS")
}
