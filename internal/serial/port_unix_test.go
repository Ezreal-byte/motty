//go:build linux || darwin

package serial

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// ptyDevice makes a pseudo-terminal pair and returns the master and the path of
// the slave, which stands in for a serial device.
func ptyDevice(t *testing.T) (*os.File, string) {
	t.Helper()
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("no pseudo-terminal: %v", err)
	}
	t.Cleanup(func() { master.Close() })
	path := slave.Name()
	// Keep the slave open so the master does not see a hangup before the test
	// opens it itself.
	t.Cleanup(func() { slave.Close() })
	return master, path
}

func TestOpenPutsTheLineInRawMode(t *testing.T) {
	_, path := ptyDevice(t)
	device, err := open(path, Settings{Baud: 57600, DataBits: 8, StopBits: 1, Flow: FlowSoftware})
	if err != nil {
		t.Fatal(err)
	}
	defer device.Close()
	termios, err := getTermios(device.(*port).fd)
	if err != nil {
		t.Fatal(err)
	}
	// A pseudo-terminal keeps its own frame (8 bits, no parity) whatever it is
	// told, so only the mode is checked here; the frame is checked on the flags.
	if termios.Lflag&(unix.ICANON|unix.ECHO|unix.ISIG) != 0 || termios.Oflag&unix.OPOST != 0 || termios.Iflag&(unix.ICRNL|unix.INLCR) != 0 {
		t.Errorf("not raw: %+v", termios)
	}
	if termios.Iflag&(unix.IXON|unix.IXOFF) != unix.IXON|unix.IXOFF {
		t.Errorf("software flow control is off: iflag %#x", termios.Iflag)
	}
}

func TestTheFrameBecomesTheRightTermiosFlags(t *testing.T) {
	for _, test := range []struct {
		settings Settings
		size     uint64
		parity   uint64
		stop     bool
	}{
		{Settings{DataBits: 8, StopBits: 1}, unix.CS8, 0, false},
		{Settings{DataBits: 7, StopBits: 2, Parity: ParityEven}, unix.CS7, unix.PARENB, true},
		{Settings{DataBits: 6, StopBits: 1, Parity: ParityOdd}, unix.CS6, unix.PARENB | unix.PARODD, false},
		{Settings{DataBits: 5, StopBits: 1}, unix.CS5, 0, false},
	} {
		var termios unix.Termios
		// Start from the worst case: everything set.
		termios.Cflag = ^termiosFlag(0)
		if err := makeRaw(&termios, test.settings); err != nil {
			t.Fatal(err)
		}
		cflag := uint64(termios.Cflag)
		if cflag&unix.CSIZE != test.size || cflag&(unix.PARENB|unix.PARODD) != test.parity || (cflag&unix.CSTOPB != 0) != test.stop {
			t.Errorf("%+v became cflag %#x", test.settings, cflag)
		}
	}
}

func TestBytesPassUntouchedBothWays(t *testing.T) {
	master, path := ptyDevice(t)
	session, err := Start(Config{Device: path, Settings: Default, Notify: func() {}})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	payload := []byte{0x00, 0x01, '\r', '\n', 0x03, 0x1a, 0xff, 0x7f, '\r', 0x11, 0x13}
	if _, err := master.Write(payload); err != nil {
		t.Fatal(err)
	}
	var got []byte
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && len(got) < len(payload) {
		got = session.Drain(got)
		time.Sleep(time.Millisecond)
	}
	if string(got) != string(payload) {
		t.Fatalf("read %v, want %v", got, payload)
	}
	if !session.Write(payload) {
		t.Fatal("write refused")
	}
	back := make([]byte, len(payload))
	master.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(master, back); err != nil {
		t.Fatal(err)
	}
	if string(back) != string(payload) {
		t.Fatalf("device received %v, want %v", back, payload)
	}
}

func TestCloseEndsABlockedReadAtOnce(t *testing.T) {
	_, path := ptyDevice(t)
	session, err := Start(Config{Device: path, Settings: Default})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond) // let the reader block
	started := time.Now()
	session.Close()
	select {
	case <-session.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("a blocked read was not ended by Close")
	}
	if time.Since(started) > time.Second || session.Err() != nil {
		t.Fatalf("close took %v, err %v", time.Since(started), session.Err())
	}
}

func TestADeviceThatGoesAwayEndsTheSessionWithAnError(t *testing.T) {
	master, path := ptyDevice(t)
	session, err := Start(Config{Device: path, Settings: Default})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	master.Close() // the adapter is unplugged
	select {
	case <-session.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("the session outlived its device")
	}
}

func TestOpeningAMissingDeviceFails(t *testing.T) {
	if _, err := Start(Config{Device: "/dev/does-not-exist-serial", Settings: Default}); err == nil {
		t.Fatal("opened a device that is not there")
	}
}
