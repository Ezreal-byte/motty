package portforward

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPConnectTunnelsTargetsAndPreservesBufferedPayload(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{name: "domain", target: "echo.internal:443"},
		{name: "IPv4", target: "127.0.0.1:8443"},
		{name: "IPv6", target: "[2001:db8::1]:9443"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := echoServer(t)
			dialed := make(chan string, 1)
			forward, err := Start(Config{
				ListenAddress: "127.0.0.1:0",
				Mode:          ModeHTTPConnect,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					if network != "tcp" {
						t.Errorf("network = %q", network)
					}
					dialed <- address
					return (&net.Dialer{}).DialContext(ctx, "tcp", target.Addr().String())
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer forward.Close()

			client, err := net.Dial("tcp", forward.Status().ListenAddress)
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte("payload after headers")
			request := []byte("CONNECT " + test.target + " HTTP/1.1\r\nHost: " + test.target + "\r\nUser-Agent: motty-test\r\n\r\n")
			request = append(request, payload...)
			if _, err := client.Write(request); err != nil {
				t.Fatal(err)
			}
			reader := bufio.NewReader(client)
			readHTTPConnectResponse(t, reader, "HTTP/1.1 200 Connection Established")
			response := make([]byte, len(payload))
			if _, err := io.ReadFull(reader, response); err != nil {
				t.Fatal(err)
			}
			if string(response) != strings.ToUpper(string(payload)) {
				t.Fatalf("response = %q", response)
			}
			if got := <-dialed; got != test.target {
				t.Fatalf("destination = %q, want %q", got, test.target)
			}
			_ = client.Close()
			waitFor(t, func(status Status) bool {
				return status.Active == 0 && status.Accepted == 1 && status.Uploaded == uint64(len(payload)) && status.Downloaded == uint64(len(payload))
			}, forward)
		})
	}
}

func TestHTTPConnectAcceptsFragmentedRequest(t *testing.T) {
	target := echoServer(t)
	forward, err := Start(Config{
		ListenAddress: "127.0.0.1:0",
		Mode:          ModeHTTPConnect,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", target.Addr().String())
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer forward.Close()
	client, err := net.Dial("tcp", forward.Status().ListenAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	writeOneByteAtATime(t, client, []byte("CONNECT fragmented.internal:22 HTTP/1.0\r\n\r\n"))
	readHTTPConnectResponse(t, bufio.NewReader(client), "HTTP/1.1 200 Connection Established")
}

func TestReadHTTPConnectRequestRejectsInvalidFrames(t *testing.T) {
	tests := []struct {
		name       string
		frame      []byte
		wantStatus string
		wantError  string
	}{
		{name: "truncated", frame: []byte("CONNECT host:22 HTTP/1.1\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "request header"},
		{name: "bare line feeds", frame: []byte("CONNECT host:22 HTTP/1.1\n\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "line endings"},
		{name: "empty request line", frame: []byte("\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "request line"},
		{name: "GET method", frame: []byte("GET host:22 HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 405 Method Not Allowed", wantError: "unsupported"},
		{name: "lowercase method", frame: []byte("connect host:22 HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 405 Method Not Allowed", wantError: "unsupported"},
		{name: "HTTP 2", frame: []byte("CONNECT host:22 HTTP/2\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "version"},
		{name: "missing target", frame: []byte("CONNECT  HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "request line"},
		{name: "extra request token", frame: []byte("CONNECT host:22 HTTP/1.1 extra\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "request line"},
		{name: "missing port", frame: []byte("CONNECT host HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "host:port"},
		{name: "zero port", frame: []byte("CONNECT host:0 HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "1-65535"},
		{name: "large port", frame: []byte("CONNECT host:65536 HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "1-65535"},
		{name: "userinfo", frame: []byte("CONNECT user@host:22 HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "host is invalid"},
		{name: "zone", frame: []byte("CONNECT [fe80::1%lo0]:22 HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "host is invalid"},
		{name: "NUL header", frame: []byte("CONNECT host:22 HTTP/1.1\r\nX-Test: a\x00b\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "line endings"},
		{name: "bare carriage return", frame: []byte("CONNECT host:22 HTTP/1.1\r\nX-Test: a\rb\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "line endings"},
		{name: "missing header colon", frame: []byte("CONNECT host:22 HTTP/1.1\r\nHost host:22\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "header field"},
		{name: "invalid header name", frame: []byte("CONNECT host:22 HTTP/1.1\r\nBad Header: value\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "header field"},
		{name: "control header value", frame: []byte("CONNECT host:22 HTTP/1.1\r\nX-Test: a\x01b\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "header field"},
		{name: "DEL header value", frame: []byte("CONNECT host:22 HTTP/1.1\r\nX-Test: a\x7fb\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "header field"},
		{name: "non-ASCII header value", frame: []byte("CONNECT host:22 HTTP/1.1\r\nX-Test: \xc3\xa9\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "header field"},
		{name: "control target", frame: []byte("CONNECT ho\x1fst:22 HTTP/1.1\r\n\r\n"), wantStatus: "HTTP/1.1 400 Bad Request", wantError: "host is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var reply bytes.Buffer
			_, err := readHTTPConnectRequest(bufio.NewReaderSize(bytes.NewReader(test.frame), httpConnectBufferSize), &reply)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want containing %q", err, test.wantError)
			}
			if !strings.HasPrefix(reply.String(), test.wantStatus+"\r\n") {
				t.Fatalf("reply = %q, want status %q", reply.String(), test.wantStatus)
			}
		})
	}
}

func TestHTTPConnectRejectsOversizedHeader(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
	}{
		{name: "single field", frame: []byte("CONNECT host:22 HTTP/1.1\r\nX-Large: " + strings.Repeat("a", httpConnectBufferSize) + "\r\n\r\n")},
		{name: "total", frame: []byte("CONNECT host:22 HTTP/1.1\r\n" + strings.Repeat("X-Pad: 1234567890\r\n", 500) + "\r\n")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var reply bytes.Buffer
			_, err := readHTTPConnectRequest(bufio.NewReaderSize(bytes.NewReader(test.frame), httpConnectBufferSize), &reply)
			if !errors.Is(err, errHTTPConnectHeaderTooLarge) {
				t.Fatalf("error = %v", err)
			}
			if !strings.HasPrefix(reply.String(), "HTTP/1.1 431 Request Header Fields Too Large\r\n") {
				t.Fatalf("reply = %q", reply.String())
			}
		})
	}
}

func TestHTTPConnectReportsDialFailureAndKeepsListening(t *testing.T) {
	target := echoServer(t)
	var attempts atomic.Int64
	forward, err := Start(Config{
		ListenAddress: "127.0.0.1:0",
		Mode:          ModeHTTPConnect,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			if attempts.Add(1) == 1 {
				return nil, errors.New("remote refused")
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", target.Addr().String())
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer forward.Close()

	failed := connectHTTPProxy(t, forward.Status().ListenAddress, "failed.internal:443")
	readHTTPConnectResponse(t, bufio.NewReader(failed), "HTTP/1.1 502 Bad Gateway")
	_ = failed.Close()
	waitFor(t, func(status Status) bool {
		return status.Active == 0 && status.Accepted == 1 && status.LastError != nil
	}, forward)

	recovered := connectHTTPProxy(t, forward.Status().ListenAddress, "working.internal:443")
	readHTTPConnectResponse(t, bufio.NewReader(recovered), "HTTP/1.1 200 Connection Established")
	waitFor(t, func(status Status) bool {
		return status.Accepted == 2 && status.LastError == nil
	}, forward)
	_ = recovered.Close()
}

func TestHTTPConnectListenerSurvivesRejectedClient(t *testing.T) {
	target := echoServer(t)
	forward, err := Start(Config{
		ListenAddress: "127.0.0.1:0",
		Mode:          ModeHTTPConnect,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", target.Addr().String())
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer forward.Close()

	rejected, err := net.Dial("tcp", forward.Status().ListenAddress)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(rejected, "GET host:22 HTTP/1.1\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	readHTTPConnectResponse(t, bufio.NewReader(rejected), "HTTP/1.1 405 Method Not Allowed")
	_ = rejected.Close()
	waitFor(t, func(status Status) bool { return status.Active == 0 && status.LastError != nil }, forward)

	connected := connectHTTPProxy(t, forward.Status().ListenAddress, "working.internal:443")
	readHTTPConnectResponse(t, bufio.NewReader(connected), "HTTP/1.1 200 Connection Established")
	_ = connected.Close()
}

func TestCloseInterruptsHTTPConnectRequest(t *testing.T) {
	forward, err := Start(Config{
		ListenAddress: "127.0.0.1:0",
		Mode:          ModeHTTPConnect,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("dial called before request completed")
			return nil, errors.New("unexpected dial")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := net.Dial("tcp", forward.Status().ListenAddress)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := io.WriteString(connection, "CONNECT host:22 HTTP/1.1\r\n"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func(status Status) bool { return status.Active == 1 }, forward)

	closed := make(chan struct{})
	go func() {
		forward.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not interrupt the HTTP CONNECT request")
	}
}

func TestHTTPConnectReplyForError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want httpConnectReply
	}{
		{name: "deadline", err: fmt.Errorf("wrapped: %w", os.ErrDeadlineExceeded), want: httpConnectReplyGatewayTimeout},
		{name: "network timeout", err: timeoutError{}, want: httpConnectReplyGatewayTimeout},
		{name: "other", err: errors.New("transport rejected"), want: httpConnectReplyBadGateway},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := httpConnectReplyForError(test.err); got != test.want {
				t.Fatalf("reply = %d, want %d", got, test.want)
			}
		})
	}
}

func TestWriteHTTPConnectReplyHandlesShortWrites(t *testing.T) {
	writer := &shortWriter{limit: 1}
	if err := writeHTTPConnectReply(writer, httpConnectReplySuccess); err != nil {
		t.Fatal(err)
	}
	if string(writer.data) != "HTTP/1.1 200 Connection Established\r\n\r\n" {
		t.Fatalf("reply = %q", writer.data)
	}
}

func FuzzReadHTTPConnectRequest(f *testing.F) {
	f.Add([]byte("CONNECT ssh.internal:22 HTTP/1.1\r\nHost: ssh.internal:22\r\n\r\n"))
	f.Add([]byte("CONNECT [2001:db8::1]:443 HTTP/1.0\r\n\r\n"))
	f.Add([]byte("GET / HTTP/1.1\r\n\r\n"))
	f.Fuzz(func(t *testing.T, frame []byte) {
		var reply bytes.Buffer
		_, _ = readHTTPConnectRequest(bufio.NewReaderSize(bytes.NewReader(frame), httpConnectBufferSize), &reply)
		if reply.Len() > 128 {
			t.Fatalf("reply length = %d", reply.Len())
		}
	})
}

func BenchmarkHTTPConnectHandshakeDomain(b *testing.B) {
	frame := []byte("CONNECT ssh.internal:22 HTTP/1.1\r\nHost: ssh.internal:22\r\n\r\n")
	b.ReportAllocs()
	for b.Loop() {
		if destination, err := readHTTPConnectRequest(bufio.NewReaderSize(bytes.NewReader(frame), httpConnectBufferSize), io.Discard); err != nil || destination != "ssh.internal:22" {
			b.Fatalf("destination=%q error=%v", destination, err)
		}
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func connectHTTPProxy(t testing.TB, address, destination string) net.Conn {
	t.Helper()
	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	writeOneByteAtATime(t, connection, []byte("CONNECT "+destination+" HTTP/1.1\r\nHost: "+destination+"\r\n\r\n"))
	return connection
}

func readHTTPConnectResponse(t testing.TB, reader *bufio.Reader, wantStatus string) {
	t.Helper()
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimRight(status, "\r\n") != wantStatus {
		t.Fatalf("status = %q, want %q", status, wantStatus)
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line == "\r\n" {
			return
		}
	}
}
