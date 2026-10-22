package workbench

import (
	"bytes"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// telnetServer is a loopback Telnet server that negotiates like a real one and
// logs a user in when it is given a name.
type telnetServer struct {
	listener net.Listener
	mu       sync.Mutex
	received []byte
}

func startTelnetServer(t *testing.T) *telnetServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &telnetServer{listener: listener}
	t.Cleanup(func() { listener.Close() })
	go func() {
		socket, err := listener.Accept()
		if err != nil {
			return
		}
		defer socket.Close()
		// WILL ECHO, WILL SGA, DO NAWS, DO TTYPE, then the login prompt.
		socket.Write([]byte{255, 251, 1, 255, 251, 3, 255, 253, 31, 255, 253, 24})
		socket.Write([]byte{255, 250, 24, 1, 255, 240}) // SB TTYPE SEND
		socket.Write([]byte("login: "))
		buffer := make([]byte, 256)
		for {
			n, err := socket.Read(buffer)
			if n > 0 {
				server.mu.Lock()
				server.received = append(server.received, buffer[:n]...)
				seen := bytes.Contains(server.received, []byte("admin\r\x00"))
				server.mu.Unlock()
				if seen {
					socket.Write([]byte("Welcome admin\r\n"))
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return server
}

func (server *telnetServer) got() []byte {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]byte(nil), server.received...)
}

func TestTelnetLoginAgainstALoopbackServer(t *testing.T) {
	server := startTelnetServer(t)
	_, port, _ := net.SplitHostPort(server.listener.Addr().String())
	number, _ := strconv.Atoi(port)

	_, options := newWorld(t)
	state := newTestState(t, options)
	state.options.Invalidate = func() {}
	addStreamHost(state, Host{Label: "Loopback", Address: "127.0.0.1", Protocol: protocolTelnet, Port: uint16(number)})
	openHost(state)
	session, _ := state.sessionByIDPointer(state.activeSessionID)
	waitUntil(t, state, "the login prompt", func() bool { return strings.Contains(screenText(state, 0), "login:") })
	session.term.send([]byte("admin\r"))
	waitUntil(t, state, "the welcome", func() bool { return strings.Contains(screenText(state, 0), "Welcome admin") })

	got := server.got()
	if !bytes.Contains(got, []byte("admin\r\x00")) {
		t.Fatalf("server received %q: a carriage return must go as CR NUL", got)
	}
	if !bytes.Contains(got, []byte{255, 250, 31}) {
		t.Fatalf("server received %q: no window size", got)
	}
	if !bytes.Contains(got, []byte{255, 250, 24, 0}) {
		t.Fatalf("server received %q: no terminal type", got)
	}
	if strings.Contains(screenText(state, 0), "\x1b") || strings.Contains(screenText(state, 0), "ÿ") {
		t.Fatalf("negotiation bytes reached the screen: %q", screenText(state, 0))
	}
}

func TestTelnetToAPortNobodyListensOnFailsWithTheReason(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listener.Close()
	number, _ := strconv.Atoi(port)

	_, options := newWorld(t)
	state := newTestState(t, options)
	state.options.Invalidate = func() {}
	state.connectionTimeout = 2 * time.Second
	addStreamHost(state, Host{Label: "Nobody", Address: "127.0.0.1", Protocol: protocolTelnet, Port: uint16(number)})
	openHost(state)
	session, _ := state.sessionByIDPointer(state.activeSessionID)
	waitUntil(t, state, "the failure", func() bool { return session.failed() })
	if !strings.Contains(session.failure, "refused") && !strings.Contains(session.failure, "connect") {
		t.Fatalf("failure = %q", session.failure)
	}
	_ = io.EOF
}
