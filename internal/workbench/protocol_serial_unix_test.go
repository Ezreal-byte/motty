//go:build !windows

package workbench

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"motty/internal/serial"
)

func TestSerialHostTalksToARealTTY(t *testing.T) {
	master, slave, err := pty.Open()
	if err != nil {
		t.Skipf("no pseudo-terminal: %v", err)
	}
	defer master.Close()
	defer slave.Close()

	_, options := newWorld(t)
	state := newTestState(t, options)
	state.options.Invalidate = func() {}
	addStreamHost(state, Host{Label: "Console", Address: slave.Name(), Protocol: protocolSerial, Line: serial.Default})
	openHost(state)
	session, _ := state.sessionByIDPointer(state.activeSessionID)
	waitUntil(t, state, "the shell", func() bool { return session.term != nil && session.term.shell != nil })

	master.Write([]byte("U-Boot 2024.01\r\nautoboot: "))
	waitUntil(t, state, "the boot banner", func() bool { return strings.Contains(screenText(state, 0), "autoboot:") })
	session.term.send([]byte("\r"))
	got := make([]byte, 1)
	master.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := io.ReadFull(master, got); err != nil || got[0] != '\r' {
		t.Fatalf("the device received %q, %v: bytes must pass untouched", got, err)
	}

	// Unplugging the adapter ends the tab with a reason.
	master.Close()
	waitUntil(t, state, "the end", func() bool { return session.term.shell == nil && session.term.ended })
}
