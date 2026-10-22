package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/crypto/ssh"
)

// The agent-forwarding fixture, enabled by E2E_AGENTFWD=1. A session's
// "auth-agent-req@openssh.com" is accepted, and once the shell runs the server
// queries the forwarded agent through its "auth-agent@openssh.com" channel —
// as a remote ssh-add -l would — and echoes what it listed into the terminal:

//	AGENTFWD <n> keys: <comment>, <comment>

// A headless run therefore proves the local agent is reachable from the remote
// side, key comments and all.

// acceptAgentForward answers the session request.
func acceptAgentForward() bool {
	return agentForwardEnabled()
}

func agentForwardEnabled() bool { return envFlag("E2E_AGENTFWD") }

func envFlag(name string) bool { return os.Getenv(name) == "1" }

// queryForwardedAgent lists the client agent's keys and reports them into the
// session channel. It runs after the shell request, when the client's channel
// handler is long registered.
func queryForwardedAgent(conn ssh.Conn, channel ssh.Channel) {
	if !agentForwardEnabled() {
		return
	}
	go func() {
		agentChannel, requests, err := conn.OpenChannel("auth-agent@openssh.com", nil)
		if err != nil {
			log.Printf("auth-agent channel refused: %v", err)
			fmt.Fprintf(channel, "\r\nAGENTFWD refused: %v\r\n", err)
			return
		}
		go ssh.DiscardRequests(requests)
		defer agentChannel.Close()
		comments, err := listIdentities(agentChannel)
		if err != nil {
			log.Printf("agent list failed: %v", err)
			fmt.Fprintf(channel, "\r\nAGENTFWD error: %v\r\n", err)
			return
		}
		log.Printf("AGENTFWD %d keys: %v", len(comments), comments)
		fmt.Fprintf(channel, "\r\nAGENTFWD %d keys: %s\r\n", len(comments), joinNonEmpty(comments))
	}()
}

// listIdentities speaks one agent REQUEST_IDENTITIES round over rw.
func listIdentities(rw io.ReadWriter) ([]string, error) {
	request := []byte{11} // REQUEST_IDENTITIES
	framed := make([]byte, 4+len(request))
	binary.BigEndian.PutUint32(framed, uint32(len(request)))
	copy(framed[4:], request)
	if _, err := rw.Write(framed); err != nil {
		return nil, err
	}
	header := make([]byte, 4)
	if _, err := io.ReadFull(rw, header); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(header)
	if length == 0 || length > 1<<20 {
		return nil, fmt.Errorf("agent reply length %d", length)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(rw, body); err != nil {
		return nil, err
	}
	if body[0] != 12 { // AGENT_IDENTITIES_ANSWER
		return nil, fmt.Errorf("agent reply type %d", body[0])
	}
	rest := body[1:]
	if len(rest) < 4 {
		return nil, fmt.Errorf("short agent reply")
	}
	count := binary.BigEndian.Uint32(rest[:4])
	rest = rest[4:]
	if count > 1024 {
		return nil, fmt.Errorf("agent lists %d keys", count)
	}
	var comments []string
	for i := uint32(0); i < count; i++ {
		_, remainder, err := takeString(rest)
		if err != nil {
			return nil, err
		}
		comment, remainder, err := takeString(remainder)
		if err != nil {
			return nil, err
		}
		rest = remainder
		comments = append(comments, string(comment))
	}
	return comments, nil
}

func takeString(data []byte) (value, rest []byte, err error) {
	if len(data) < 4 {
		return nil, nil, fmt.Errorf("agent reply is short")
	}
	length := binary.BigEndian.Uint32(data[:4])
	if uint64(length) > uint64(len(data[4:])) {
		return nil, nil, fmt.Errorf("agent string overruns the reply")
	}
	return data[4 : 4+length], data[4+length:], nil
}

func joinNonEmpty(parts []string) string {
	out := ""
	for index, part := range parts {
		if part == "" {
			part = "(no comment)"
		}
		if index > 0 {
			out += ", "
		}
		out += part
	}
	return out
}
