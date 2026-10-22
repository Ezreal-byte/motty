package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestX11RelayTurnsALocalConnectionIntoAnX11ChannelToTheClient(t *testing.T) {
	t.Setenv("E2E_X11", "1")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(private)
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	relayReady := make(chan *x11Relay, 1)
	go func() {
		socket, err := listener.Accept()
		if err != nil {
			return
		}
		conn, channels, requests, err := ssh.NewServerConn(socket, config)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(requests)
		go func() {
			for range channels {
			}
		}()
		relayReady <- newX11Relay(conn)
	}()
	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User: "u", HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	x11Channels := client.HandleChannelOpen("x11")
	relay := <-relayReady
	defer relay.Close()
	payload := ssh.Marshal(struct {
		Single   bool
		Protocol string
		Cookie   string
		Screen   uint32
	}{false, "MIT-MAGIC-COOKIE-1", "00112233445566778899aabbccddeeff", 0})
	if !relay.Request(payload) {
		t.Fatal("x11-req refused")
	}
	env := strings.Join(relay.Env(), " ")
	if !strings.Contains(env, "E2E_X11_COOKIE=00112233445566778899aabbccddeeff") || !strings.Contains(env, "DISPLAY=127.0.0.1:") {
		t.Fatalf("env %q", env)
	}
	go func() {
		for request := range x11Channels {
			channel, _, _ := request.Accept()
			io.Copy(channel, channel) // echo
		}
	}()
	var port string
	for _, entry := range relay.Env() {
		if rest, ok := strings.CutPrefix(entry, "DISPLAY=127.0.0.1:"); ok {
			number := strings.TrimSuffix(rest, ".0")
			port = "60" + number
		}
	}
	program, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatal(err)
	}
	defer program.Close()
	program.Write([]byte("setup"))
	reply := make([]byte, 5)
	if _, err := io.ReadFull(program, reply); err != nil || string(reply) != "setup" {
		t.Fatalf("reply %q err=%v", reply, err)
	}
}

func TestX11RelayIsOffUnlessEnabled(t *testing.T) {
	t.Setenv("E2E_X11", "")
	if newX11Relay(nil).Request(nil) {
		t.Fatal("accepted without E2E_X11=1")
	}
}
