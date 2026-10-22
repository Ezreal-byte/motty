package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"sync"

	"golang.org/x/crypto/ssh"
)

// The X11 fixture, enabled by E2E_X11=1. Like sshd it answers a session's
// "x11-req", listens on 127.0.0.1:6000+N for X clients of that session (the
// shell gets DISPLAY=127.0.0.1:N.0 and E2E_X11_COOKIE, the hex of the fake
// cookie the client sent) and turns every connection into an "x11" channel to
// the SSH client. It is how a headless run proves the client swaps the cookie.

type x11Relay struct {
	conn ssh.Conn

	mu       sync.Mutex
	listener net.Listener
	display  int
	cookie   string
}

func newX11Relay(conn ssh.Conn) *x11Relay { return &x11Relay{conn: conn} }

// Request handles an x11-req payload and reports whether it was accepted.
func (relay *x11Relay) Request(payload []byte) bool {
	if os.Getenv("E2E_X11") != "1" {
		return false
	}
	var request struct {
		SingleConnection bool
		AuthProtocol     string
		AuthCookie       string
		Screen           uint32
	}
	if ssh.Unmarshal(payload, &request) != nil || request.AuthProtocol != "MIT-MAGIC-COOKIE-1" {
		return false
	}
	relay.mu.Lock()
	defer relay.mu.Unlock()
	if relay.listener != nil {
		return false
	}
	for display := 10; display < 60; display++ {
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(6000+display)))
		if err != nil {
			continue
		}
		relay.listener, relay.display, relay.cookie = listener, display, request.AuthCookie
		go relay.accept(listener)
		log.Printf("x11-req cookie=%s screen=%d display=%d", request.AuthCookie, request.Screen, display)
		return true
	}
	return false
}

// Env is the environment sshd would give the shell.
func (relay *x11Relay) Env() []string {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	if relay.listener == nil {
		return nil
	}
	return []string{fmt.Sprintf("DISPLAY=127.0.0.1:%d.0", relay.display), "E2E_X11_COOKIE=" + relay.cookie}
}

func (relay *x11Relay) accept(listener net.Listener) {
	for {
		client, err := listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer client.Close()
			channel, requests, err := relay.conn.OpenChannel("x11", ssh.Marshal(struct {
				Host string
				Port uint32
			}{"127.0.0.1", uint32(client.RemoteAddr().(*net.TCPAddr).Port)}))
			if err != nil {
				log.Printf("x11 channel refused: %v", err)
				return
			}
			go ssh.DiscardRequests(requests)
			defer channel.Close()
			done := make(chan struct{})
			go func() {
				io.Copy(client, channel)
				close(done)
			}()
			io.Copy(channel, client)
			channel.CloseWrite()
			<-done
		}()
	}
}

func (relay *x11Relay) Close() {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	if relay.listener != nil {
		relay.listener.Close()
	}
}
