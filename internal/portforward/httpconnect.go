package portforward

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	httpConnectBufferSize = 4 << 10
	httpConnectMaxHeader  = 8 << 10
)

type httpConnectReply uint8

const (
	httpConnectReplySuccess httpConnectReply = iota
	httpConnectReplyBadRequest
	httpConnectReplyMethodNotAllowed
	httpConnectReplyHeaderTooLarge
	httpConnectReplyBadGateway
	httpConnectReplyGatewayTimeout
)

var errHTTPConnectHeaderTooLarge = errors.New("HTTP CONNECT header limits exceeded")

func readHTTPConnectRequest(reader *bufio.Reader, writer io.Writer) (string, error) {
	total := 0
	requestLine, err := readHTTPConnectLine(reader, &total)
	if err != nil {
		reply := httpConnectReplyBadRequest
		if errors.Is(err, errHTTPConnectHeaderTooLarge) {
			reply = httpConnectReplyHeaderTooLarge
		}
		return "", rejectHTTPConnect(writer, reply, err)
	}
	destination, reply, err := parseHTTPConnectRequestLine(requestLine[:len(requestLine)-2])
	if err != nil {
		return "", rejectHTTPConnect(writer, reply, err)
	}
	for {
		line, err := readHTTPConnectLine(reader, &total)
		if err != nil {
			reply := httpConnectReplyBadRequest
			if errors.Is(err, errHTTPConnectHeaderTooLarge) {
				reply = httpConnectReplyHeaderTooLarge
			}
			return "", rejectHTTPConnect(writer, reply, err)
		}
		if len(line) == 2 {
			break
		}
		if !validHTTPConnectField(line[:len(line)-2]) {
			return "", rejectHTTPConnect(writer, httpConnectReplyBadRequest, errors.New("HTTP CONNECT header field is malformed"))
		}
	}
	return destination, nil
}

func readHTTPConnectLine(reader *bufio.Reader, total *int) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	*total += len(line)
	if *total > httpConnectMaxHeader || errors.Is(err, bufio.ErrBufferFull) {
		return nil, errHTTPConnectHeaderTooLarge
	}
	if err != nil {
		return nil, fmt.Errorf("HTTP CONNECT request header: %w", err)
	}
	if len(line) < 2 || line[len(line)-2] != '\r' || bytes.IndexByte(line[:len(line)-2], '\r') >= 0 || bytes.IndexByte(line[:len(line)-2], '\n') >= 0 || bytes.IndexByte(line, 0) >= 0 {
		return nil, errors.New("HTTP CONNECT request uses malformed line endings")
	}
	return line, nil
}

func parseHTTPConnectRequestLine(line []byte) (string, httpConnectReply, error) {
	method, target, version, ok := splitHTTPConnectRequestLine(line)
	if !ok {
		return "", httpConnectReplyBadRequest, errors.New("HTTP CONNECT request line is malformed")
	}
	if !bytes.Equal(method, []byte("CONNECT")) {
		return "", httpConnectReplyMethodNotAllowed, fmt.Errorf("HTTP CONNECT method %q is unsupported", method)
	}
	if !bytes.Equal(version, []byte("HTTP/1.0")) && !bytes.Equal(version, []byte("HTTP/1.1")) {
		return "", httpConnectReplyBadRequest, fmt.Errorf("HTTP CONNECT version %q is unsupported", version)
	}
	destination, err := normalizeHTTPConnectTarget(string(target))
	if err != nil {
		return "", httpConnectReplyBadRequest, err
	}
	return destination, httpConnectReplySuccess, nil
}

func validHTTPConnectField(line []byte) bool {
	colon := bytes.IndexByte(line, ':')
	if colon <= 0 {
		return false
	}
	for _, value := range line[:colon] {
		if !isHTTPTokenByte(value) {
			return false
		}
	}
	for _, value := range line[colon+1:] {
		if value != '\t' && (value < ' ' || value > '~') {
			return false
		}
	}
	return true
}

func isHTTPTokenByte(value byte) bool {
	if value >= '0' && value <= '9' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' {
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", rune(value))
}

func splitHTTPConnectRequestLine(line []byte) (method, target, version []byte, ok bool) {
	first := bytes.IndexByte(line, ' ')
	if first <= 0 {
		return nil, nil, nil, false
	}
	secondRelative := bytes.IndexByte(line[first+1:], ' ')
	if secondRelative <= 0 {
		return nil, nil, nil, false
	}
	second := first + 1 + secondRelative
	if second+1 >= len(line) || bytes.IndexByte(line[second+1:], ' ') >= 0 {
		return nil, nil, nil, false
	}
	return line[:first], line[first+1 : second], line[second+1:], true
}

func normalizeHTTPConnectTarget(target string) (string, error) {
	host, portText, err := net.SplitHostPort(target)
	if err != nil || host == "" {
		return "", errors.New("HTTP CONNECT target must be host:port")
	}
	for index := range host {
		if host[index] <= ' ' || host[index] >= 0x7f || strings.ContainsRune("/@%?#", rune(host[index])) {
			return "", errors.New("HTTP CONNECT target host is invalid")
		}
	}
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return "", errors.New("HTTP CONNECT target host is invalid")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return "", errors.New("HTTP CONNECT target port must be 1-65535")
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), nil
}

func rejectHTTPConnect(writer io.Writer, reply httpConnectReply, requestErr error) error {
	if err := writeHTTPConnectReply(writer, reply); err != nil {
		return errors.Join(requestErr, fmt.Errorf("HTTP CONNECT reply: %w", err))
	}
	return requestErr
}

func writeHTTPConnectReply(writer io.Writer, reply httpConnectReply) error {
	var response string
	switch reply {
	case httpConnectReplySuccess:
		response = "HTTP/1.1 200 Connection Established\r\n\r\n"
	case httpConnectReplyBadRequest:
		response = "HTTP/1.1 400 Bad Request\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	case httpConnectReplyMethodNotAllowed:
		response = "HTTP/1.1 405 Method Not Allowed\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	case httpConnectReplyHeaderTooLarge:
		response = "HTTP/1.1 431 Request Header Fields Too Large\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	case httpConnectReplyGatewayTimeout:
		response = "HTTP/1.1 504 Gateway Timeout\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	default:
		response = "HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\nContent-Length: 0\r\n\r\n"
	}
	return writeMessage(writer, []byte(response))
}

func httpConnectReplyForError(err error) httpConnectReply {
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return httpConnectReplyGatewayTimeout
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		return httpConnectReplyGatewayTimeout
	}
	return httpConnectReplyBadGateway
}
