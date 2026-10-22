package scpclient

import (
	"io"
	"sync"

	"golang.org/x/crypto/ssh"
)

// SSH starts commands as exec sessions of an authenticated client.
func SSH(client *ssh.Client) Start {
	return func(command string) (Channel, error) {
		session, err := client.NewSession()
		if err != nil {
			return nil, err
		}
		stdin, err := session.StdinPipe()
		if err != nil {
			_ = session.Close()
			return nil, err
		}
		stdout, err := session.StdoutPipe()
		if err != nil {
			_ = session.Close()
			return nil, err
		}
		channel := &sshChannel{session: session, stdin: stdin, stdout: stdout}
		session.Stderr = &channel.stderr
		if err := session.Start(command); err != nil {
			_ = session.Close()
			return nil, err
		}
		return channel, nil
	}
}

type sshChannel struct {
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  tail
}

func (c *sshChannel) Read(p []byte) (int, error)  { return c.stdout.Read(p) }
func (c *sshChannel) Write(p []byte) (int, error) { return c.stdin.Write(p) }
func (c *sshChannel) CloseWrite() error           { return c.stdin.Close() }
func (c *sshChannel) Wait() error                 { return c.session.Wait() }
func (c *sshChannel) Close() error {
	err := c.session.Close()
	if err == io.EOF {
		return nil
	}
	return err
}
func (c *sshChannel) Stderr() string { return c.stderr.String() }

// tail keeps the last bytes written to it.
type tail struct {
	mu   sync.Mutex
	data []byte
}

const tailBytes = 2048

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.data = append(t.data, p...)
	if len(t.data) > tailBytes {
		t.data = append(t.data[:0], t.data[len(t.data)-tailBytes:]...)
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.data)
}
