package workbench

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sync"

	"motty/internal/localshell"
	"motty/internal/sftpclient"
	"motty/internal/sshconn"
)

// errLocalOnly is what the remote-only capabilities say to a local terminal.
var errLocalOnly = errors.New("workbench: not available in a local terminal")

// localHostID marks a session that runs a shell on this machine. It is not a
// saved host: no catalog entry has it.
const localHostID = ^uint32(0)

// localConn is the Connection of a local terminal: connected from the start, no
// credentials to ask for, and a shell on this machine instead of a remote one.
// Everything that needs a server (files, forwarding, commands) refuses.
type localConn struct {
	mu     sync.Mutex
	status sshconn.Status
	dir    string
}

func newLocalConn() *localConn {
	return &localConn{status: sshconn.Status{Version: 1, Phase: sshconn.PhaseConnected, Method: "local"}}
}

func (conn *localConn) Status() sshconn.Status {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	return conn.status
}

func (conn *localConn) TrustHostKey(bool)          {}
func (conn *localConn) RejectHostKey()             {}
func (conn *localConn) SubmitPassword(string)      {}
func (conn *localConn) SubmitAnswers([]string)     {}
func (conn *localConn) SubmitKeyPassphrase(string) {}
func (conn *localConn) SkipIdentity()              {}
func (conn *localConn) Cancel()                    {}

func (conn *localConn) OpenShell(config sshconn.ShellConfig) (Shell, error) {
	return localshell.Start(localshell.Config{Cols: config.Cols, Rows: config.Rows, Dir: conn.dir, Notify: config.Notify})
}

func (conn *localConn) OpenSFTP() (sftpclient.Remote, error) { return nil, errLocalOnly }

func (conn *localConn) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, errLocalOnly
}

func (conn *localConn) Listen(string, string) (net.Listener, error) { return nil, errLocalOnly }

func (conn *localConn) RunCommand(context.Context, string, int) ([]byte, error) {
	return nil, errLocalOnly
}

func (conn *localConn) StreamCommand(context.Context, string, int, func([]byte) bool) error {
	return errLocalOnly
}

// openLocalTerminal opens a shell on this computer, in the empty pane the user
// is looking at or else in a new tab, like any other host.
func (state *State) openLocalTerminal() *Session {
	host := localHost(state.messages.LocalTerminalTitle)
	return state.createHostSession(host, host.Address)
}

// localSystem is the operating system this program runs on, so the tab wears
// the same badge a remote machine of that kind would.
func localSystem() sshconn.RemoteSystem {
	switch runtime.GOOS {
	case "darwin":
		return sshconn.SystemDarwin
	case "windows":
		return sshconn.SystemWindows
	case "linux":
		return sshconn.SystemLinux
	}
	return sshconn.SystemUnknown
}

// sessionAddress is what a session shows as its address: user, host and port for
// an SSH host, the computer's name for a shell on this one.
func (state *State) sessionAddress(host Host) string {
	switch {
	case host.ID == localHostID:
		return host.Address
	case host.Protocol == protocolSerial:
		return host.Address
	case host.Protocol == protocolTelnet || host.isDesktop():
		return fmt.Sprintf("%s:%d", host.Address, host.Port)
	}
	return fmt.Sprintf("%s@%s:%d", state.login(host), host.Address, host.Port)
}

// isLocal reports whether a session runs on this machine.
func (session *Session) isLocal() bool { return session.HostID == localHostID }

// localHost is the Host a local session carries, so the code that shows or saves
// a session's host has something to read.
func localHost(label string) Host {
	name, _ := os.Hostname()
	host := Host{ID: localHostID, Label: label, Address: name, System: localSystem()}
	host.finish()
	return host
}
