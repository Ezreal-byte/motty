package sshconn

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

// echoServer accepts connections and echoes what it reads.
func echoServer(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			socket, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer socket.Close(); io.Copy(socket, socket) }()
		}
	}()
	return listener.Addr().String()
}

// fakeProxy is a proxy listener whose handler reads the client's request itself.
type fakeProxy struct {
	address string
	seen    chan string // what the handler decoded about each request
}

func startProxy(t *testing.T, handle func(net.Conn, chan<- string)) *fakeProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	proxy := &fakeProxy{address: listener.Addr().String(), seen: make(chan string, 16)}
	go func() {
		for {
			socket, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer socket.Close(); handle(socket, proxy.seen) }()
		}
	}()
	return proxy
}

// socks5Handler speaks the server side of SOCKS5 and tunnels to the real destination.
// wantLogin makes it require user "me" and password "secret"; reply, when not zero,
// is what it answers to the connect request instead of success.
func socks5Handler(wantLogin bool, reply byte) func(net.Conn, chan<- string) {
	return func(client net.Conn, seen chan<- string) {
		var head [2]byte
		if _, err := io.ReadFull(client, head[:]); err != nil || head[0] != 5 {
			return
		}
		methods := make([]byte, head[1])
		io.ReadFull(client, methods)
		if wantLogin {
			client.Write([]byte{5, 2})
			var version, ulen [1]byte
			io.ReadFull(client, version[:])
			io.ReadFull(client, ulen[:])
			user := make([]byte, ulen[0])
			io.ReadFull(client, user)
			var plen [1]byte
			io.ReadFull(client, plen[:])
			password := make([]byte, plen[0])
			io.ReadFull(client, password)
			if string(user) != "me" || string(password) != "secret" {
				client.Write([]byte{1, 1})
				return
			}
			client.Write([]byte{1, 0})
		} else {
			client.Write([]byte{5, 0})
		}
		var request [4]byte
		if _, err := io.ReadFull(client, request[:]); err != nil {
			return
		}
		var host string
		switch request[3] {
		case 1:
			var ip [4]byte
			io.ReadFull(client, ip[:])
			host = net.IP(ip[:]).String()
		case 3:
			var length [1]byte
			io.ReadFull(client, length[:])
			name := make([]byte, length[0])
			io.ReadFull(client, name)
			host = string(name)
		case 4:
			var ip [16]byte
			io.ReadFull(client, ip[:])
			host = net.IP(ip[:]).String()
		}
		var port [2]byte
		io.ReadFull(client, port[:])
		destination := net.JoinHostPort(host, strconv.Itoa(int(port[0])<<8|int(port[1])))
		seen <- "socks5 " + destination
		if reply != 0 {
			client.Write([]byte{5, reply, 0, 1, 0, 0, 0, 0, 0, 0})
			return
		}
		// The test addresses are names the proxy "resolves": send everything to the echo server.
		target, err := net.Dial("tcp", echoTarget)
		if err != nil {
			client.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
			return
		}
		defer target.Close()
		client.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0x12, 0x34})
		go func() { io.Copy(target, client); target.Close() }()
		io.Copy(client, target)
	}
}

var echoTarget string

func httpConnectHandler(wantLogin bool, status string, extra string) func(net.Conn, chan<- string) {
	return func(client net.Conn, seen chan<- string) {
		reader := bufio.NewReader(client)
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		authorization := ""
		for {
			header, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if header == "\r\n" {
				break
			}
			if strings.HasPrefix(header, "Proxy-Authorization: ") {
				authorization = strings.TrimSpace(strings.TrimPrefix(header, "Proxy-Authorization: "))
			}
		}
		seen <- strings.TrimSpace(line) + " | " + authorization
		if wantLogin && authorization != "Basic "+base64.StdEncoding.EncodeToString([]byte("me:secret")) {
			client.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\n\r\n"))
			return
		}
		if status != "200 Connection established" {
			client.Write([]byte("HTTP/1.1 " + status + "\r\n\r\n"))
			return
		}
		target, err := net.Dial("tcp", echoTarget)
		if err != nil {
			client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer target.Close()
		// The first tunnel bytes ride in the same segment as the answer.
		client.Write([]byte("HTTP/1.1 " + status + "\r\n" + extra + "\r\n" + "early"))
		go func() { io.Copy(target, reader); target.Close() }()
		io.Copy(client, target)
	}
}

func roundTrip(t *testing.T, socket net.Conn, message string) string {
	t.Helper()
	socket.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := socket.Write([]byte(message)); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, len(message))
	if _, err := io.ReadFull(socket, buffer); err != nil {
		t.Fatalf("read back: %v", err)
	}
	return string(buffer)
}

func TestSOCKS5CarriesADomainNameAndAnAddressAndAuthenticates(t *testing.T) {
	echoTarget = echoServer(t)
	open := startProxy(t, socks5Handler(false, 0))
	login := startProxy(t, socks5Handler(true, 0))

	socket, err := (&Proxy{Kind: ProxySOCKS5, Address: open.address}).Dial(context.Background(), time.Second, "server.internal:22")
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	if got := roundTrip(t, socket, "hello"); got != "hello" {
		t.Fatalf("echo = %q", got)
	}
	if seen := <-open.seen; seen != "socks5 server.internal:22" {
		t.Fatalf("the proxy was asked for %q: a name must reach it unresolved", seen)
	}

	socket, err = (&Proxy{Kind: ProxySOCKS5, Address: login.address, Username: "me", Password: "secret"}).Dial(context.Background(), time.Second, "10.1.2.3:2222")
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	roundTrip(t, socket, "x")
	if seen := <-login.seen; seen != "socks5 10.1.2.3:2222" {
		t.Fatalf("seen %q", seen)
	}

	if _, err := (&Proxy{Kind: ProxySOCKS5, Address: login.address, Username: "me", Password: "wrong"}).Dial(context.Background(), time.Second, "h:1"); !errors.Is(err, ErrProxyRefused) {
		t.Fatalf("a wrong password gave %v", err)
	}
	if _, err := (&Proxy{Kind: ProxySOCKS5, Address: login.address}).Dial(context.Background(), time.Second, "h:1"); !errors.Is(err, ErrProxyRefused) {
		t.Fatalf("no login against a proxy that wants one gave %v", err)
	}
}

func TestSOCKS5RefusalsAreReportedInWords(t *testing.T) {
	echoTarget = echoServer(t)
	refusing := startProxy(t, socks5Handler(false, 5))
	_, err := (&Proxy{Kind: ProxySOCKS5, Address: refusing.address}).Dial(context.Background(), time.Second, "h:22")
	if !errors.Is(err, ErrProxyRefused) || !strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("err = %v", err)
	}
	if _, err := (&Proxy{Kind: ProxySOCKS5, Address: "127.0.0.1:1"}).Dial(context.Background(), time.Second, "h:22"); err == nil || !strings.Contains(err.Error(), "connect to the proxy") {
		t.Fatalf("an unreachable proxy gave %v", err)
	}
	notSOCKS := startProxy(t, func(client net.Conn, _ chan<- string) { client.Write([]byte("HTTP/1.1 400 Bad\r\n\r\n")) })
	if _, err := (&Proxy{Kind: ProxySOCKS5, Address: notSOCKS.address}).Dial(context.Background(), time.Second, "h:22"); err == nil {
		t.Fatal("an HTTP server was taken for a SOCKS5 proxy")
	}
}

func TestHTTPConnectTunnelsKeepsEarlyBytesAndAuthenticates(t *testing.T) {
	echoTarget = echoServer(t)
	open := startProxy(t, httpConnectHandler(false, "200 Connection established", "Via: test\r\n"))
	socket, err := (&Proxy{Kind: ProxyHTTPConnect, Address: open.address}).Dial(context.Background(), time.Second, "[::1]:22")
	if err != nil {
		t.Fatal(err)
	}
	defer socket.Close()
	early := make([]byte, 5)
	socket.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := io.ReadFull(socket, early); err != nil || string(early) != "early" {
		t.Fatalf("bytes sent with the answer were lost: %q %v", early, err)
	}
	if got := roundTrip(t, socket, "ping"); got != "ping" {
		t.Fatalf("echo = %q", got)
	}
	if seen := <-open.seen; !strings.HasPrefix(seen, "CONNECT [::1]:22 HTTP/1.1 | ") || strings.HasSuffix(seen, "| ") == false {
		t.Fatalf("request = %q", seen)
	}

	login := startProxy(t, httpConnectHandler(true, "200 Connection established", ""))
	socket, err = (&Proxy{Kind: ProxyHTTPConnect, Address: login.address, Username: "me", Password: "secret"}).Dial(context.Background(), time.Second, "host.example:443")
	if err != nil {
		t.Fatal(err)
	}
	socket.Close()
	if seen := <-login.seen; !strings.Contains(seen, "CONNECT host.example:443") || !strings.Contains(seen, "Basic "+base64.StdEncoding.EncodeToString([]byte("me:secret"))) {
		t.Fatalf("request = %q", seen)
	}
	if _, err := (&Proxy{Kind: ProxyHTTPConnect, Address: login.address, Username: "me", Password: "no"}).Dial(context.Background(), time.Second, "h:1"); !errors.Is(err, ErrProxyRefused) || !strings.Contains(err.Error(), "authentication required") {
		t.Fatalf("a wrong login gave %v", err)
	}
	denied := startProxy(t, httpConnectHandler(false, "403 Forbidden", ""))
	if _, err := (&Proxy{Kind: ProxyHTTPConnect, Address: denied.address}).Dial(context.Background(), time.Second, "h:1"); !errors.Is(err, ErrProxyRefused) || !strings.Contains(err.Error(), "403") {
		t.Fatalf("a refusal gave %v", err)
	}
}

func TestAProxyThatNeverAnswersOrAnswersEndlesslyDoesNotHangTheDial(t *testing.T) {
	silent := startProxy(t, func(client net.Conn, _ chan<- string) { io.Copy(io.Discard, client) })
	started := time.Now()
	if _, err := (&Proxy{Kind: ProxySOCKS5, Address: silent.address}).Dial(context.Background(), 300*time.Millisecond, "h:1"); err == nil || time.Since(started) > 3*time.Second {
		t.Fatalf("silent proxy: %v after %v", err, time.Since(started))
	}
	cancelled, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := (&Proxy{Kind: ProxyHTTPConnect, Address: silent.address}).Dial(cancelled, 0, "h:1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context deadline gave %v", err)
	}
	endless := startProxy(t, func(client net.Conn, _ chan<- string) {
		client.Write([]byte("HTTP/1.1 200 OK\r\n"))
		for {
			if _, err := client.Write([]byte("X-Header: " + strings.Repeat("a", 500) + "\r\n")); err != nil {
				return
			}
		}
	})
	if _, err := (&Proxy{Kind: ProxyHTTPConnect, Address: endless.address}).Dial(context.Background(), time.Second, "h:1"); err == nil || !strings.Contains(err.Error(), "too long") && !strings.Contains(err.Error(), "ended inside") {
		t.Fatalf("endless headers gave %v", err)
	}
}

func TestWhichDestinationsGoThroughTheProxy(t *testing.T) {
	proxy := &Proxy{Kind: ProxySOCKS5, Address: "p:1", Bypass: []string{"localhost", "127.0.0.0/8", "::1", "*.corp.example", ".lab", "10.9.8.7", "Exact.Host"}}
	for address, through := range map[string]bool{
		"example.com:22":      true,
		"localhost:22":        false,
		"127.5.5.5:22":        false,
		"[::1]:22":            false,
		"::1":                 false,
		"db.corp.example:22":  false,
		"corp.example:22":     false,
		"notcorp.example:22":  true,
		"box.lab:22":          false,
		"10.9.8.7:22":         false,
		"10.9.8.8:22":         true,
		"exact.host:22":       false,
		"192.168.1.10:22":     true,
		"[2001:db8::1]:22":    true,
		"::ffff:127.0.0.1:22": true, // not a valid host:port; falls to the bare-host rule
	} {
		if got := proxy.Uses(address); got != through {
			t.Errorf("Uses(%q) = %v, want %v", address, got, through)
		}
	}
	for _, none := range []*Proxy{nil, {}, {Kind: ProxySOCKS5}, {Kind: ProxyNone, Address: "p:1"}} {
		if none.Uses("example.com:22") {
			t.Errorf("%+v uses a proxy", none)
		}
	}
}

func TestASessionConnectsThroughAProxyToAHostOnlyTheProxyKnows(t *testing.T) {
	server := startServer(t, func(server *testServer) { server.password = "pw" })
	host, port := server.host()
	for name, handler := range map[string]func(net.Conn, chan<- string){
		"socks5": func(client net.Conn, seen chan<- string) {
			var head [2]byte
			io.ReadFull(client, head[:])
			io.CopyN(io.Discard, client, int64(head[1]))
			client.Write([]byte{5, 0})
			var request [4]byte
			io.ReadFull(client, request[:])
			var length [1]byte
			io.ReadFull(client, length[:])
			destination := make([]byte, int(length[0])+2)
			io.ReadFull(client, destination)
			seen <- string(destination[:length[0]])
			target, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(port))))
			if err != nil {
				return
			}
			defer target.Close()
			client.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
			go func() { io.Copy(target, client); target.Close() }()
			io.Copy(client, target)
		},
		"http": func(client net.Conn, seen chan<- string) {
			reader := bufio.NewReader(client)
			line, _ := reader.ReadString('\n')
			for {
				header, err := reader.ReadString('\n')
				if err != nil || header == "\r\n" {
					break
				}
			}
			seen <- strings.Fields(line)[1]
			target, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(int(port))))
			if err != nil {
				return
			}
			defer target.Close()
			client.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			go func() { io.Copy(target, reader); target.Close() }()
			io.Copy(client, target)
		},
	} {
		t.Run(name, func(t *testing.T) {
			proxy := startProxy(t, handler)
			kind := ProxySOCKS5
			if name == "http" {
				kind = ProxyHTTPConnect
			}
			harness := dial(t, server, func(config *Config) {
				config.Host, config.Port = "ssh.internal.example", 2222
				config.Proxy = &Proxy{Kind: kind, Address: proxy.address}
			})
			harness.phase(PhaseHostKeyPrompt)
			harness.conn.TrustHostKey(false)
			harness.phase(PhasePasswordPrompt)
			harness.conn.SubmitPassword("pw")
			harness.phase(PhaseConnected)
			want := "ssh.internal.example"
			if name == "http" {
				want = "ssh.internal.example:2222"
			}
			if seen := <-proxy.seen; seen != want {
				t.Fatalf("the proxy was asked for %q, want %q", seen, want)
			}
		})
	}
}

func TestAProxyThatCannotBeReachedFailsTheSessionWithItsReason(t *testing.T) {
	server := startServer(t, func(server *testServer) { server.password = "pw" })
	harness := dial(t, server, func(config *Config) {
		config.Proxy = &Proxy{Kind: ProxySOCKS5, Address: "127.0.0.1:1"}
	})
	status := harness.until("failure", func(status Status) bool { return status.Phase == PhaseFailed })
	if status.Err == nil || !strings.Contains(status.Err.Error(), "proxy") {
		t.Fatalf("err = %v", status.Err)
	}
}
