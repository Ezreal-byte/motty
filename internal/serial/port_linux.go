//go:build linux

package serial

import "golang.org/x/sys/unix"

// flush drops what the line still holds in either direction.
func flush(fd int) { _ = unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIOFLUSH) }

// configure puts the line in raw mode at the settings. Linux takes any baud
// rate through termios2's BOTHER, so there is no table of standard rates.
func configure(fd int, settings Settings) error {
	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS2)
	if err != nil {
		return err
	}
	if err := makeRaw(termios, settings); err != nil {
		return err
	}
	termios.Cflag &^= unix.CBAUD | unix.CRTSCTS
	termios.Cflag |= unix.BOTHER
	if settings.Flow == FlowHardware {
		termios.Cflag |= unix.CRTSCTS
	}
	termios.Ispeed, termios.Ospeed = uint32(settings.Baud), uint32(settings.Baud)
	return unix.IoctlSetTermios(fd, unix.TCSETS2, termios)
}

func markSpaceParity(termios *unix.Termios, parity Parity) error {
	const cmspar = 0x40000000
	termios.Cflag |= unix.PARENB | cmspar
	if parity == ParityMark {
		termios.Cflag |= unix.PARODD
	}
	return nil
}
