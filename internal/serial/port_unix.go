//go:build linux || darwin

package serial

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/sys/unix"
)

// port is an open tty. Reads wait in select(2) on the descriptor and on a pipe
// that Close writes to, so an idle port costs no wakeups and closing it ends a
// blocked read at once. select rather than poll: poll does not work on
// character devices on macOS.
type port struct {
	fd        int
	wakeRead  int
	wakeWrite int

	mu     sync.Mutex
	closed bool
	users  sync.WaitGroup
}

// fdSetSize is how many descriptors select can watch.
const fdSetSize = 1024

func open(device string, settings Settings) (io.ReadWriteCloser, error) {
	fd, err := unix.Open(device, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: device, Err: err}
	}
	var pipe [2]int
	if err := unix.Pipe(pipe[:]); err != nil {
		unix.Close(fd)
		return nil, err
	}
	for _, end := range pipe {
		unix.CloseOnExec(end)
		unix.SetNonblock(end, true)
	}
	if fd >= fdSetSize || pipe[0] >= fdSetSize {
		unix.Close(fd)
		unix.Close(pipe[0])
		unix.Close(pipe[1])
		return nil, errors.New("serial: too many open files to wait on a port")
	}
	if err := configure(fd, settings); err != nil {
		unix.Close(fd)
		unix.Close(pipe[0])
		unix.Close(pipe[1])
		return nil, fmt.Errorf("serial: configuring %s: %w", device, err)
	}
	// Lines can be left in whatever state the last user had them; start clean.
	flush(fd)
	return &port{fd: fd, wakeRead: pipe[0], wakeWrite: pipe[1]}, nil
}

func (p *port) Read(buffer []byte) (int, error) {
	if !p.enter() {
		return 0, io.EOF
	}
	defer p.users.Done()
	for {
		var set unix.FdSet
		set.Set(p.fd)
		set.Set(p.wakeRead)
		if _, err := unix.Select(max(p.fd, p.wakeRead)+1, &set, nil, nil, nil); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return 0, err
		}
		if set.IsSet(p.wakeRead) {
			return 0, io.EOF
		}
		n, err := unix.Read(p.fd, buffer)
		switch {
		case n > 0:
			return n, nil
		case err == nil:
			// A device that was unplugged reads as end of file.
			return 0, io.ErrUnexpectedEOF
		case errors.Is(err, unix.EAGAIN), errors.Is(err, unix.EINTR):
			continue
		default:
			return 0, err
		}
	}
}

func (p *port) Write(data []byte) (int, error) {
	if !p.enter() {
		return 0, io.ErrClosedPipe
	}
	defer p.users.Done()
	written := 0
	for written < len(data) {
		n, err := unix.Write(p.fd, data[written:])
		if n > 0 {
			written += n
		}
		if err == nil || errors.Is(err, unix.EINTR) {
			continue
		}
		if !errors.Is(err, unix.EAGAIN) {
			return written, err
		}
		// The device is not taking more (flow control, a full buffer): wait.
		var read, write unix.FdSet
		read.Set(p.wakeRead)
		write.Set(p.fd)
		if _, err := unix.Select(max(p.fd, p.wakeRead)+1, &read, &write, nil, nil); err != nil && !errors.Is(err, unix.EINTR) {
			return written, err
		}
		if read.IsSet(p.wakeRead) {
			return written, io.ErrClosedPipe
		}
	}
	return written, nil
}

// enter registers a reader or writer, or reports that the port is closed.
func (p *port) enter() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	p.users.Add(1)
	return true
}

func (p *port) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()
	// Wake whoever waits, then free the descriptors once nobody uses them: a
	// descriptor closed under a reader could be reused by an unrelated open.
	_, _ = unix.Write(p.wakeWrite, []byte{0})
	p.users.Wait()
	unix.Close(p.wakeWrite)
	unix.Close(p.wakeRead)
	return unix.Close(p.fd)
}

// makeRaw sets what every platform needs: no translation of bytes, no echo, no
// signals, 8-bit clean, and the frame and flow control of the settings. The
// platform file adds the baud rate and whatever else its termios needs.
func makeRaw(termios *unix.Termios, settings Settings) error {
	termios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON | unix.IXOFF | unix.IXANY
	termios.Oflag &^= unix.OPOST
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	termios.Cflag &^= unix.CSIZE | unix.PARENB | unix.PARODD | unix.CSTOPB
	termios.Cflag |= unix.CREAD | unix.CLOCAL
	switch settings.DataBits {
	case 5:
		termios.Cflag |= unix.CS5
	case 6:
		termios.Cflag |= unix.CS6
	case 7:
		termios.Cflag |= unix.CS7
	default:
		termios.Cflag |= unix.CS8
	}
	if settings.StopBits == 2 {
		termios.Cflag |= unix.CSTOPB
	}
	switch settings.Parity {
	case ParityNone:
	case ParityOdd:
		termios.Cflag |= unix.PARENB | unix.PARODD
	case ParityEven:
		termios.Cflag |= unix.PARENB
	default:
		if err := markSpaceParity(termios, settings.Parity); err != nil {
			return err
		}
	}
	if settings.Flow == FlowSoftware {
		termios.Iflag |= unix.IXON | unix.IXOFF
	}
	termios.Cc[unix.VMIN] = 1
	termios.Cc[unix.VTIME] = 0
	return nil
}
