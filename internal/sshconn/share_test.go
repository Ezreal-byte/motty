package sshconn

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// shareNotifier counts the status changes a shared Conn reports.
type shareNotifier struct {
	mu    sync.Mutex
	count int
}

func (notifier *shareNotifier) notify() {
	notifier.mu.Lock()
	notifier.count++
	notifier.mu.Unlock()
}

func (notifier *shareNotifier) seen() int {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	return notifier.count
}

func waitFor(t *testing.T, description string, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ready() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", description)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// share is Share, with the share let go when the test ends: a connection lives
// until every tab on it has, and the test server waits for it to end.
func share(t *testing.T, conn *Conn, notify func()) *Conn {
	t.Helper()
	child, err := conn.Share(notify)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(child.Close)
	return child
}

func openSharedShell(t *testing.T, conn *Conn) *shellWatcher {
	t.Helper()
	return openShell(t, &harness{t: t, conn: conn}, 80, 24)
}

func TestSharedTabsUseOneAuthenticatedConnection(t *testing.T) {
	server := passwordServer(t, echoShell)
	owner := connect(t, server)
	var notifier shareNotifier
	child := share(t, owner.conn, notifier.notify)
	status := child.Status()
	if status.Phase != PhaseConnected || status.Method != "password" || status.ServerVersion != "SSH-2.0-GopTest" || child.Client() != owner.conn.Client() {
		t.Fatalf("shared status = %+v", status)
	}
	first := openSharedShell(t, owner.conn)
	second := openSharedShell(t, child)
	first.until("the greeting", func(output string) bool { return strings.Contains(output, "welcome") })
	second.until("the greeting", func(output string) bool { return strings.Contains(output, "welcome") })
	second.shell.Write([]byte("hi\r"))
	second.until("the echo", func(output string) bool { return strings.HasSuffix(output, "HI\r") })
	if got := server.activeConnections(); got != 1 {
		t.Fatalf("the server holds %d connections, want the one", got)
	}
	if !child.Shared() || !owner.conn.Shared() {
		t.Fatal("Shared did not report the sharing")
	}
	if len(server.offered()) != 1 {
		t.Fatalf("the password was asked for %d times, want once", len(server.offered()))
	}
}

func TestTheConnectionOutlivesTheTabThatDialedItUntilTheLastShareLetsGo(t *testing.T) {
	server := passwordServer(t, echoShell)
	owner := connect(t, server)
	first := share(t, owner.conn, nil)
	second := share(t, first, nil) // a share can be shared again

	owner.conn.Close() // the tab that dialed goes first
	if owner.conn.Status().Phase != PhaseClosed || owner.conn.Status().Err != nil {
		t.Fatalf("the owner's tab = %+v", owner.conn.Status())
	}
	select {
	case <-owner.conn.Done():
	default:
		t.Fatal("the owner's tab did not end")
	}
	w := openSharedShell(t, first)
	w.until("the greeting", func(output string) bool { return strings.Contains(output, "welcome") })
	if got := server.activeConnections(); got != 1 {
		t.Fatalf("the connection ended with the tab that dialed it: %d connections", got)
	}
	// A tab can still be opened from what is left.
	third := share(t, second, nil) // still possible with the owner's tab gone

	first.Close()
	second.Close()
	if server.activeConnections() != 1 {
		t.Fatal("the connection ended while a tab still used it")
	}
	third.Close()
	waitFor(t, "the connection to close", func() bool { return server.activeConnections() == 0 })
	if _, err := third.Share(nil); !errors.Is(err, ErrNotShareable) {
		t.Fatalf("Share of an ended connection = %v", err)
	}
}

func TestLettingGoOfAShareLeavesTheOwnerRunning(t *testing.T) {
	server := passwordServer(t, echoShell)
	owner := connect(t, server)
	var notifier shareNotifier
	child := share(t, owner.conn, notifier.notify)
	child.Cancel()
	child.Cancel() // a second let-go changes nothing
	if status := child.Status(); status.Phase != PhaseClosed || status.Err != nil {
		t.Fatalf("the share = %+v", status)
	}
	if notifier.seen() != 1 {
		t.Fatalf("the share reported %d changes, want one", notifier.seen())
	}
	if owner.conn.Status().Phase != PhaseConnected {
		t.Fatalf("the owner = %+v", owner.conn.Status())
	}
	w := openSharedShell(t, owner.conn)
	w.until("the greeting", func(output string) bool { return strings.Contains(output, "welcome") })
}

func TestADroppedConnectionEndsEveryShareWithItsReason(t *testing.T) {
	var serverSide *ssh.ServerConn
	var mu sync.Mutex
	server := startServer(t, func(s *testServer) {
		s.password = "secret"
		s.shell = echoShell
		s.connected = func(conn *ssh.ServerConn) {
			mu.Lock()
			serverSide = conn
			mu.Unlock()
		}
	})
	owner := connect(t, server)
	var one, two shareNotifier
	first := share(t, owner.conn, one.notify)
	second := share(t, first, two.notify)
	waitFor(t, "the server side", func() bool { mu.Lock(); defer mu.Unlock(); return serverSide != nil })
	mu.Lock()
	serverSide.Close()
	mu.Unlock()

	for index, share := range []*Conn{first, second} {
		select {
		case <-share.Done():
		case <-time.After(10 * time.Second):
			t.Fatalf("share %d did not end with the connection", index+1)
		}
		if status := share.Status(); status.Phase != PhaseClosed || status.Err == nil {
			t.Fatalf("share %d = %+v, want closed with the reason", index+1, status)
		}
	}
	if one.seen() == 0 || two.seen() == 0 {
		t.Fatal("a share was not told that the connection ended")
	}
	if _, err := second.Share(nil); !errors.Is(err, ErrNotShareable) {
		t.Fatalf("Share after the drop = %v", err)
	}
}

func TestAConnectionThatIsNotEstablishedCannotBeShared(t *testing.T) {
	server := passwordServer(t, echoShell)
	h := dial(t, server, nil)
	h.phase(PhaseHostKeyPrompt)
	if _, err := h.conn.Share(nil); !errors.Is(err, ErrNotShareable) {
		t.Fatalf("Share while connecting = %v", err)
	}
	h.conn.Cancel()
	<-h.conn.Done()
	if _, err := h.conn.Share(nil); !errors.Is(err, ErrNotShareable) {
		t.Fatalf("Share after cancel = %v", err)
	}
	if h.conn.Shared() {
		t.Fatal("a connection nobody shares reports sharing")
	}
}
