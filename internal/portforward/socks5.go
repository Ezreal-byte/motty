package portforward

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"syscall"
)

const (
	socks5Version           = 5
	socks5MethodNone        = 0
	socks5MethodUnavailable = 0xff
	socks5CommandConnect    = 1
	socks5AddressIPv4       = 1
	socks5AddressDomain     = 3
	socks5AddressIPv6       = 4

	socks5ReplySucceeded              = 0
	socks5ReplyGeneralFailure         = 1
	socks5ReplyNetworkUnreachable     = 3
	socks5ReplyHostUnreachable        = 4
	socks5ReplyConnectionRefused      = 5
	socks5ReplyCommandUnsupported     = 7
	socks5ReplyAddressTypeUnsupported = 8
)

func readSOCKS5Request(reader io.Reader, writer io.Writer) (string, error) {
	var greeting [2]byte
	if _, err := io.ReadFull(reader, greeting[:]); err != nil {
		return "", fmt.Errorf("SOCKS5 greeting: %w", err)
	}
	if greeting[0] != socks5Version {
		return "", fmt.Errorf("SOCKS5 greeting: unsupported version %d", greeting[0])
	}
	methods := make([]byte, int(greeting[1]))
	if _, err := io.ReadFull(reader, methods); err != nil {
		return "", fmt.Errorf("SOCKS5 methods: %w", err)
	}
	if !hasSOCKS5Method(methods, socks5MethodNone) {
		_ = writeSOCKS5Method(writer, socks5MethodUnavailable)
		return "", errors.New("SOCKS5 authentication: no supported method")
	}
	if err := writeSOCKS5Method(writer, socks5MethodNone); err != nil {
		return "", fmt.Errorf("SOCKS5 method reply: %w", err)
	}

	var request [4]byte
	if _, err := io.ReadFull(reader, request[:]); err != nil {
		return "", fmt.Errorf("SOCKS5 request: %w", err)
	}
	if request[0] != socks5Version || request[2] != 0 {
		_ = writeSOCKS5Reply(writer, socks5ReplyGeneralFailure)
		return "", errors.New("SOCKS5 request: malformed header")
	}
	if request[1] != socks5CommandConnect {
		_ = writeSOCKS5Reply(writer, socks5ReplyCommandUnsupported)
		return "", fmt.Errorf("SOCKS5 request: unsupported command %d", request[1])
	}
	host, err := readSOCKS5Host(reader, request[3])
	if err != nil {
		if errors.Is(err, errSOCKS5AddressType) {
			_ = writeSOCKS5Reply(writer, socks5ReplyAddressTypeUnsupported)
		} else {
			_ = writeSOCKS5Reply(writer, socks5ReplyGeneralFailure)
		}
		return "", err
	}
	var portBytes [2]byte
	if _, err := io.ReadFull(reader, portBytes[:]); err != nil {
		_ = writeSOCKS5Reply(writer, socks5ReplyGeneralFailure)
		return "", fmt.Errorf("SOCKS5 destination port: %w", err)
	}
	port := binary.BigEndian.Uint16(portBytes[:])
	if port == 0 {
		_ = writeSOCKS5Reply(writer, socks5ReplyGeneralFailure)
		return "", errors.New("SOCKS5 destination port: zero is invalid")
	}
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func hasSOCKS5Method(methods []byte, method byte) bool {
	for _, candidate := range methods {
		if candidate == method {
			return true
		}
	}
	return false
}

var errSOCKS5AddressType = errors.New("SOCKS5 address type unsupported")

func readSOCKS5Host(reader io.Reader, addressType byte) (string, error) {
	switch addressType {
	case socks5AddressIPv4:
		var address [net.IPv4len]byte
		if _, err := io.ReadFull(reader, address[:]); err != nil {
			return "", fmt.Errorf("SOCKS5 IPv4 destination: %w", err)
		}
		return net.IP(address[:]).String(), nil
	case socks5AddressIPv6:
		var address [net.IPv6len]byte
		if _, err := io.ReadFull(reader, address[:]); err != nil {
			return "", fmt.Errorf("SOCKS5 IPv6 destination: %w", err)
		}
		return net.IP(address[:]).String(), nil
	case socks5AddressDomain:
		var length [1]byte
		if _, err := io.ReadFull(reader, length[:]); err != nil {
			return "", fmt.Errorf("SOCKS5 domain length: %w", err)
		}
		if length[0] == 0 {
			return "", errors.New("SOCKS5 domain destination: empty host")
		}
		host := make([]byte, int(length[0]))
		if _, err := io.ReadFull(reader, host); err != nil {
			return "", fmt.Errorf("SOCKS5 domain destination: %w", err)
		}
		return string(host), nil
	default:
		return "", fmt.Errorf("%w: %d", errSOCKS5AddressType, addressType)
	}
}

func writeSOCKS5Method(writer io.Writer, method byte) error {
	message := [2]byte{socks5Version, method}
	return writeMessage(writer, message[:])
}

func writeSOCKS5Reply(writer io.Writer, reply byte) error {
	message := [10]byte{socks5Version, reply, 0, socks5AddressIPv4}
	return writeMessage(writer, message[:])
}

func writeMessage(writer io.Writer, message []byte) error {
	for len(message) > 0 {
		count, err := writer.Write(message)
		if err != nil {
			return err
		}
		if count == 0 {
			return io.ErrShortWrite
		}
		message = message[count:]
	}
	return nil
}

func socks5ReplyForError(err error) byte {
	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return socks5ReplyConnectionRefused
	case errors.Is(err, syscall.ENETUNREACH):
		return socks5ReplyNetworkUnreachable
	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, os.ErrDeadlineExceeded):
		return socks5ReplyHostUnreachable
	default:
		return socks5ReplyGeneralFailure
	}
}
