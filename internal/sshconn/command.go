package sshconn

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrCommandOutputTooLarge reports that a remote command wrote more stdout
// than its caller agreed to retain.
var ErrCommandOutputTooLarge = errors.New("sshconn: command output too large")

// RunCommand runs one command on the authenticated server and returns its
// stdout. maxOutputBytes bounds retained output; extra bytes are discarded and
// reported as ErrCommandOutputTooLarge when the command ends. Canceling ctx
// closes only this command session, not the SSH connection.
func (conn *Conn) RunCommand(ctx context.Context, command string, maxOutputBytes int) ([]byte, error) {
	if command == "" {
		return nil, errors.New("sshconn: empty command")
	}
	if maxOutputBytes <= 0 {
		return nil, errors.New("sshconn: command output limit must be positive")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if conn.ctx != nil {
		select {
		case <-conn.ctx.Done():
			return nil, ErrCanceled
		default:
		}
	}
	client := conn.Client()
	if client == nil {
		return nil, errors.New("sshconn: not connected")
	}
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	output := commandOutput{limit: maxOutputBytes}
	session.Stdout = &output
	if err := session.Start(command); err != nil {
		return nil, err
	}
	waited := make(chan error, 1)
	go func() { waited <- session.Wait() }()

	select {
	case err = <-waited:
	case <-ctx.Done():
		_ = session.Close()
		<-waited
		return nil, ctx.Err()
	case <-conn.ctx.Done():
		_ = session.Close()
		<-waited
		return nil, ErrCanceled
	}
	data, exceeded := output.result()
	if exceeded {
		return data, fmt.Errorf("%w: limit %d bytes", ErrCommandOutputTooLarge, maxOutputBytes)
	}
	return data, err
}

// commandOutput remains bounded even when a hostile command ignores its
// documented output contract. Session may copy stdout from its own goroutine.
type commandOutput struct {
	mu       sync.Mutex
	data     []byte
	limit    int
	exceeded bool
}

func (output *commandOutput) Write(data []byte) (int, error) {
	written := len(data)
	output.mu.Lock()
	remaining := output.limit - len(output.data)
	if remaining > 0 {
		output.data = append(output.data, data[:min(remaining, len(data))]...)
	}
	if len(data) > remaining {
		output.exceeded = true
	}
	output.mu.Unlock()
	return written, nil
}

func (output *commandOutput) result() ([]byte, bool) {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.data, output.exceeded
}

// StreamCommand runs one long-lived command on the authenticated server and
// calls line with each stdout line, newline included, as it arrives. The slice
// is valid only during the call. It returns nil once line reports false, which
// ends the command's session, and ErrCommandOutputTooLarge for a line longer
// than maxLineBytes. When the command ends by itself the result is its exit
// status. Canceling ctx closes only this command session.
func (conn *Conn) StreamCommand(ctx context.Context, command string, maxLineBytes int, line func([]byte) bool) error {
	if command == "" {
		return errors.New("sshconn: empty command")
	}
	if maxLineBytes <= 0 {
		return errors.New("sshconn: command line limit must be positive")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var connDone <-chan struct{}
	if conn.ctx != nil {
		connDone = conn.ctx.Done()
		select {
		case <-connDone:
			return ErrCanceled
		default:
		}
	}
	client := conn.Client()
	if client == nil {
		return errors.New("sshconn: not connected")
	}
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	if err := session.Start(command); err != nil {
		return err
	}
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ctx.Done():
		case <-connDone:
		case <-finished:
			return
		}
		_ = session.Close()
	}()

	reader := bufio.NewReaderSize(stdout, max(maxLineBytes, 16))
	for {
		data, err := reader.ReadSlice('\n')
		if err == nil {
			if !line(data) {
				return nil
			}
			continue
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			return fmt.Errorf("%w: limit %d bytes", ErrCommandOutputTooLarge, maxLineBytes)
		}
		break
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-connDone:
		return ErrCanceled
	default:
	}
	return session.Wait()
}
