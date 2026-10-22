package portforward

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestSOCKS5ConnectsAddressTypesByteByByte(t *testing.T) {
	tests := []struct {
		name        string
		addressType byte
		address     []byte
		port        uint16
		want        string
	}{
		{name: "IPv4", addressType: socks5AddressIPv4, address: []byte{127, 0, 0, 1}, port: 2222, want: "127.0.0.1:2222"},
		{name: "IPv6", addressType: socks5AddressIPv6, address: net.ParseIP("2001:db8::1").To16(), port: 2200, want: "[2001:db8::1]:2200"},
		{name: "domain", addressType: socks5AddressDomain, address: []byte("ssh.internal"), port: 22, want: "ssh.internal:22"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := echoServer(t)
			dialed := make(chan string, 1)
			forward, err := Start(Config{
				ListenAddress: "127.0.0.1:0",
				Mode:          ModeSOCKS5,
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
			writeOneByteAtATime(t, client, []byte{socks5Version, 2, 2, socks5MethodNone})
			readSOCKS5Response(t, client, []byte{socks5Version, socks5MethodNone})
			request := socks5ConnectRequest(test.addressType, test.address, test.port)
			writeOneByteAtATime(t, client, request)
			readSOCKS5Response(t, client, socks5Reply(socks5ReplySucceeded))
			if got := <-dialed; got != test.want {
				t.Fatalf("destination = %q, want %q", got, test.want)
			}

			if _, err := io.WriteString(client, "proxy me"); err != nil {
				t.Fatal(err)
			}
			response := make([]byte, len("PROXY ME"))
			if _, err := io.ReadFull(client, response); err != nil {
				t.Fatal(err)
			}
			if string(response) != "PROXY ME" {
				t.Fatalf("response = %q", response)
			}
			_ = client.Close()
			waitFor(t, func(status Status) bool {
				return status.Active == 0 && status.Accepted == 1 && status.Uploaded == 8 && status.Downloaded == 8
			}, forward)
		})
	}
}

func TestReadSOCKS5RequestRejectsInvalidFrames(t *testing.T) {
	validGreeting := []byte{socks5Version, 1, socks5MethodNone}
	tests := []struct {
		name      string
		frame     []byte
		wantReply []byte
		wantError string
	}{
		{name: "truncated greeting", frame: []byte{socks5Version}, wantError: "greeting"},
		{name: "old version", frame: []byte{4, 1, socks5MethodNone}, wantError: "unsupported version"},
		{name: "zero methods", frame: []byte{socks5Version, 0}, wantReply: []byte{socks5Version, socks5MethodUnavailable}, wantError: "no supported method"},
		{name: "truncated methods", frame: []byte{socks5Version, 2, socks5MethodNone}, wantError: "methods"},
		{name: "unsupported authentication", frame: []byte{socks5Version, 1, 2}, wantReply: []byte{socks5Version, socks5MethodUnavailable}, wantError: "no supported method"},
		{name: "truncated request", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect), wantReply: []byte{socks5Version, socks5MethodNone}, wantError: "request"},
		{name: "malformed request version", frame: appendFrame(validGreeting, 4, socks5CommandConnect, 0, socks5AddressIPv4), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyGeneralFailure)...), wantError: "malformed header"},
		{name: "nonzero reserved byte", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect, 1, socks5AddressIPv4), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyGeneralFailure)...), wantError: "malformed header"},
		{name: "bind command", frame: appendFrame(validGreeting, socks5Version, 2, 0, socks5AddressIPv4), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyCommandUnsupported)...), wantError: "unsupported command"},
		{name: "UDP command", frame: appendFrame(validGreeting, socks5Version, 3, 0, socks5AddressIPv4), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyCommandUnsupported)...), wantError: "unsupported command"},
		{name: "unknown address type", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect, 0, 2), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyAddressTypeUnsupported)...), wantError: "address type unsupported"},
		{name: "truncated IPv4", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect, 0, socks5AddressIPv4, 127), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyGeneralFailure)...), wantError: "IPv4 destination"},
		{name: "truncated IPv6", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect, 0, socks5AddressIPv6, 0), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyGeneralFailure)...), wantError: "IPv6 destination"},
		{name: "empty domain", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect, 0, socks5AddressDomain, 0), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyGeneralFailure)...), wantError: "empty host"},
		{name: "truncated domain", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect, 0, socks5AddressDomain, 3, 'a'), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyGeneralFailure)...), wantError: "domain destination"},
		{name: "truncated port", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect, 0, socks5AddressIPv4, 127, 0, 0, 1, 0), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyGeneralFailure)...), wantError: "destination port"},
		{name: "zero port", frame: appendFrame(validGreeting, socks5Version, socks5CommandConnect, 0, socks5AddressIPv4, 127, 0, 0, 1, 0, 0), wantReply: append([]byte{socks5Version, socks5MethodNone}, socks5Reply(socks5ReplyGeneralFailure)...), wantError: "zero is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var reply bytes.Buffer
			_, err := readSOCKS5Request(bytes.NewReader(test.frame), &reply)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want containing %q", err, test.wantError)
			}
			if !bytes.Equal(reply.Bytes(), test.wantReply) {
				t.Fatalf("reply = %v, want %v", reply.Bytes(), test.wantReply)
			}
		})
	}
}

func TestSOCKS5ReportsDialFailureAndKeepsListening(t *testing.T) {
	target := echoServer(t)
	var attempts atomic.Int64
	forward, err := Start(Config{
		ListenAddress: "127.0.0.1:0",
		Mode:          ModeSOCKS5,
		Dial: func(ctx context.Context, _ string, _ string) (net.Conn, error) {
			if attempts.Add(1) == 1 {
				return nil, fmt.Errorf("remote refused: %w", syscall.ECONNREFUSED)
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", target.Addr().String())
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer forward.Close()

	failed := connectSOCKS5(t, forward.Status().ListenAddress, socks5ConnectRequest(socks5AddressDomain, []byte("failed.internal"), 22))
	readSOCKS5Response(t, failed, socks5Reply(socks5ReplyConnectionRefused))
	_ = failed.Close()
	waitFor(t, func(status Status) bool {
		return status.Active == 0 && status.Accepted == 1 && status.LastError != nil
	}, forward)
	if !strings.Contains(forward.Status().LastError.Error(), "remote refused") {
		t.Fatalf("last error = %v", forward.Status().LastError)
	}

	recovered := connectSOCKS5(t, forward.Status().ListenAddress, socks5ConnectRequest(socks5AddressDomain, []byte("working.internal"), 22))
	readSOCKS5Response(t, recovered, socks5Reply(socks5ReplySucceeded))
	waitFor(t, func(status Status) bool {
		return status.Accepted == 2 && status.LastError == nil
	}, forward)
	_ = recovered.Close()
}

func TestSOCKS5ListenerSurvivesRejectedClient(t *testing.T) {
	target := echoServer(t)
	forward, err := Start(Config{
		ListenAddress: "127.0.0.1:0",
		Mode:          ModeSOCKS5,
		Dial: func(ctx context.Context, _ string, _ string) (net.Conn, error) {
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
	writeOneByteAtATime(t, rejected, []byte{socks5Version, 1, 2})
	readSOCKS5Response(t, rejected, []byte{socks5Version, socks5MethodUnavailable})
	_ = rejected.Close()
	waitFor(t, func(status Status) bool { return status.Active == 0 && status.LastError != nil }, forward)

	connected := connectSOCKS5(t, forward.Status().ListenAddress, socks5ConnectRequest(socks5AddressIPv4, []byte{127, 0, 0, 1}, 22))
	readSOCKS5Response(t, connected, socks5Reply(socks5ReplySucceeded))
	_ = connected.Close()
}

func TestCloseInterruptsSOCKS5Handshake(t *testing.T) {
	forward, err := Start(Config{
		ListenAddress: "127.0.0.1:0",
		Mode:          ModeSOCKS5,
		Dial: func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("dial called before handshake completed")
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
	waitFor(t, func(status Status) bool { return status.Active == 1 }, forward)

	closed := make(chan struct{})
	go func() {
		forward.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("Close did not interrupt the SOCKS5 handshake")
	}
}

func TestWriteSOCKS5MessageHandlesShortWrites(t *testing.T) {
	writer := &shortWriter{limit: 1}
	if err := writeSOCKS5Reply(writer, socks5ReplySucceeded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(writer.data, socks5Reply(socks5ReplySucceeded)) {
		t.Fatalf("reply = %v", writer.data)
	}
}

func TestSOCKS5ReplyForError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want byte
	}{
		{name: "connection refused", err: fmt.Errorf("wrapped: %w", syscall.ECONNREFUSED), want: socks5ReplyConnectionRefused},
		{name: "network unreachable", err: fmt.Errorf("wrapped: %w", syscall.ENETUNREACH), want: socks5ReplyNetworkUnreachable},
		{name: "host unreachable", err: fmt.Errorf("wrapped: %w", syscall.EHOSTUNREACH), want: socks5ReplyHostUnreachable},
		{name: "timeout", err: fmt.Errorf("wrapped: %w", os.ErrDeadlineExceeded), want: socks5ReplyHostUnreachable},
		{name: "other", err: errors.New("transport rejected"), want: socks5ReplyGeneralFailure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := socks5ReplyForError(test.err); got != test.want {
				t.Fatalf("reply = %d, want %d", got, test.want)
			}
		})
	}
}

func FuzzReadSOCKS5Request(f *testing.F) {
	f.Add([]byte{socks5Version, 1, socks5MethodNone, socks5Version, socks5CommandConnect, 0, socks5AddressIPv4, 127, 0, 0, 1, 0, 22})
	f.Add(appendFrame([]byte{socks5Version, 1, socks5MethodNone}, socks5Version, socks5CommandConnect, 0, socks5AddressDomain, 3, 's', 's', 'h', 0, 22))
	f.Add([]byte{socks5Version, 1, 2})
	f.Fuzz(func(t *testing.T, frame []byte) {
		var reply bytes.Buffer
		_, _ = readSOCKS5Request(bytes.NewReader(frame), &reply)
		if reply.Len() > 12 {
			t.Fatalf("reply length = %d", reply.Len())
		}
	})
}

func BenchmarkSOCKS5HandshakeDomain(b *testing.B) {
	frame := appendFrame([]byte{socks5Version, 1, socks5MethodNone}, socks5Version, socks5CommandConnect, 0, socks5AddressDomain, 12)
	frame = append(frame, []byte("ssh.internal")...)
	frame = append(frame, 0, 22)
	b.ReportAllocs()
	for b.Loop() {
		if destination, err := readSOCKS5Request(bytes.NewReader(frame), io.Discard); err != nil || destination != "ssh.internal:22" {
			b.Fatalf("destination=%q error=%v", destination, err)
		}
	}
}

type shortWriter struct {
	data  []byte
	limit int
}

func (writer *shortWriter) Write(data []byte) (int, error) {
	if len(data) > writer.limit {
		data = data[:writer.limit]
	}
	writer.data = append(writer.data, data...)
	return len(data), nil
}

func connectSOCKS5(t testing.TB, address string, request []byte) net.Conn {
	t.Helper()
	connection, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	writeOneByteAtATime(t, connection, []byte{socks5Version, 1, socks5MethodNone})
	readSOCKS5Response(t, connection, []byte{socks5Version, socks5MethodNone})
	writeOneByteAtATime(t, connection, request)
	return connection
}

func socks5ConnectRequest(addressType byte, address []byte, port uint16) []byte {
	request := []byte{socks5Version, socks5CommandConnect, 0, addressType}
	if addressType == socks5AddressDomain {
		request = append(request, byte(len(address)))
	}
	request = append(request, address...)
	request = binary.BigEndian.AppendUint16(request, port)
	return request
}

func socks5Reply(code byte) []byte {
	return []byte{socks5Version, code, 0, socks5AddressIPv4, 0, 0, 0, 0, 0, 0}
}

func appendFrame(prefix []byte, suffix ...byte) []byte {
	frame := append([]byte(nil), prefix...)
	return append(frame, suffix...)
}

func writeOneByteAtATime(t testing.TB, writer io.Writer, data []byte) {
	t.Helper()
	for _, value := range data {
		if _, err := writer.Write([]byte{value}); err != nil {
			t.Fatal(err)
		}
	}
}

func readSOCKS5Response(t testing.TB, reader io.Reader, want []byte) {
	t.Helper()
	got := make([]byte, len(want))
	if _, err := io.ReadFull(reader, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("response = %v, want %v", got, want)
	}
}
