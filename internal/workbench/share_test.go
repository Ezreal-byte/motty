package workbench

import (
	"errors"
	"strings"
	"testing"

	"motty/internal/sshconn"
)

// lender is a fake connection that can lend itself, as the real one does.
type lender struct {
	*fakeConn
	world *sharingWorld
}

func (l *lender) Share(func()) (Connection, error) {
	l.fakeConn.mu.Lock()
	connected := l.fakeConn.status.Phase == sshconn.PhaseConnected
	l.fakeConn.mu.Unlock()
	if !connected || l.world.refuse {
		return nil, sshconn.ErrNotShareable
	}
	child := &fakeConn{}
	child.set(func(status *sshconn.Status) { status.Phase = sshconn.PhaseConnected })
	l.world.shared = append(l.world.shared, child)
	return &lender{fakeConn: child, world: l.world}, nil
}

type sharingWorld struct {
	*world
	shared []*fakeConn
	refuse bool
}

// newSharingWorld dials connections that can be shared.
func newSharingWorld(t testing.TB) (*sharingWorld, Options) {
	t.Helper()
	known, options := newWorld(t)
	sharing := &sharingWorld{world: known}
	options.Dial = func(config sshconn.Config) Connection {
		fake := &fakeConn{}
		known.configs = append(known.configs, config)
		known.conns = append(known.conns, fake)
		return &lender{fakeConn: fake, world: sharing}
	}
	return sharing, options
}

func (world *sharingWorld) connectFirst(t testing.TB, state *State) *fakeConn {
	t.Helper()
	openHost(state)
	if len(world.conns) != 1 {
		t.Fatalf("dials = %d", len(world.conns))
	}
	fake := world.conns[0]
	fake.set(func(status *sshconn.Status) {
		status.Phase = sshconn.PhaseConnected
		status.ServerVersion = "SSH-2.0-Test"
	})
	state.Wake()
	return fake
}

func TestASplitOfAConnectedHostSharesItsConnectionInsteadOfDialing(t *testing.T) {
	world, options := newSharingWorld(t)
	state := newTestState(t, options)
	first := world.connectFirst(t, state)

	state.Click(hitPaneSplitHorizontal)
	if len(world.conns) != 1 || len(world.shared) != 1 {
		t.Fatalf("dials %d, shares %d: want the one dial and one share", len(world.conns), len(world.shared))
	}
	split, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || !split.shared || split.phase != sshconn.PhaseConnected {
		t.Fatalf("split session = %+v", split)
	}
	if len(world.shared[0].shells) != 1 {
		t.Fatalf("the shared tab opened %d shells, want its own", len(world.shared[0].shells))
	}
	if len(first.shells) != 1 {
		t.Fatalf("the first tab has %d shells", len(first.shells))
	}
	if text := split.term.screen.Text(); strings.Contains(text, "connection") {
		t.Fatalf("sharing is announced in the terminal: %q", text)
	}
}

func TestANewTabOnAConnectedHostSharesToo(t *testing.T) {
	world, options := newSharingWorld(t)
	state := newTestState(t, options)
	world.connectFirst(t, state)

	state.Click(hitHosts)
	openHost(state)
	if len(world.conns) != 1 || len(world.shared) != 1 {
		t.Fatalf("dials %d, shares %d", len(world.conns), len(world.shared))
	}
	if len(state.sessions) != 2 || !state.sessions[1].shared {
		t.Fatalf("sessions = %+v", state.sessions)
	}
}

func TestAChangedEffectiveProxyPolicyNeverSharesTheOldConnection(t *testing.T) {
	tests := []struct {
		name        string
		initialHost *outboundProxy
		initial     outboundProxy
		nextHost    *outboundProxy
		next        outboundProxy
	}{
		{
			name:     "proxied to direct",
			initial:  outboundProxy{kind: sshconn.ProxySOCKS5, address: "global.proxy:1080"},
			nextHost: &outboundProxy{},
		},
		{
			name:        "direct to dedicated",
			initialHost: &outboundProxy{},
			nextHost:    &outboundProxy{kind: sshconn.ProxyHTTPConnect, address: "host.proxy:3128"},
		},
		{
			name:    "inherited global proxy changed",
			initial: outboundProxy{kind: sshconn.ProxySOCKS5, address: "first.proxy:1080"},
			next:    outboundProxy{kind: sshconn.ProxySOCKS5, address: "second.proxy:1080"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			world, options := newSharingWorld(t)
			state := newTestState(t, options)
			state.proxy = test.initial
			state.hosts[0].Proxy = cloneOutboundProxy(test.initialHost)
			world.connectFirst(t, state)

			state.proxy = test.next
			state.hosts[0].Proxy = cloneOutboundProxy(test.nextHost)
			// Host editing updates open tabs for future reconnects, but it cannot
			// change the transport their live connection already established.
			state.sessions[0].host = state.hosts[0]
			state.Click(hitHosts)
			openHost(state)
			if len(world.conns) != 2 || len(world.shared) != 0 {
				t.Fatalf("dials=%d shares=%d", len(world.conns), len(world.shared))
			}
		})
	}
}

func TestAnotherHostIsDialedNotShared(t *testing.T) {
	world, options := newSharingWorld(t)
	state := newTestState(t, options)
	world.connectFirst(t, state)

	state.Click(hitHosts)
	state.Click(hitHostList.At(2)) // Staging
	openHost(state)
	if len(world.conns) != 2 || len(world.shared) != 0 {
		t.Fatalf("dials %d, shares %d: a different host must be its own connection", len(world.conns), len(world.shared))
	}
}

func TestSharingCanBeTurnedOffAndTheChoiceIsKept(t *testing.T) {
	world, options := newSharingWorld(t)
	state := newTestState(t, options)
	world.connectFirst(t, state)
	state.Click(hitSettings)
	state.Click(hitSettingsShareToggle)
	if state.shareConnections != shareConnectionsOff {
		t.Fatal("the setting did not change")
	}

	state.Click(hitHosts)
	openHost(state)
	if len(world.conns) != 2 || len(world.shared) != 0 {
		t.Fatalf("dials %d, shares %d: with sharing off a tab dials", len(world.conns), len(world.shared))
	}

	again := newTestState(t, options)
	if again.shareConnections != shareConnectionsOff {
		t.Fatal("the setting was not saved")
	}
	again.Click(hitSettings)
	again.Click(hitSettingsShareToggle)
	if again.shareConnections != shareConnectionsOn || again.snapshot().ShareConnections != "on" {
		t.Fatal("turning it back on did not stick")
	}
}

func TestAConnectionThatIsNotConnectedYetIsNotLent(t *testing.T) {
	world, options := newSharingWorld(t)
	state := newTestState(t, options)
	openHost(state) // still connecting
	state.Click(hitPaneSplitHorizontal)
	if len(world.conns) != 2 || len(world.shared) != 0 {
		t.Fatalf("dials %d, shares %d: a connection that is not up cannot be lent", len(world.conns), len(world.shared))
	}
}

func TestAServerThatRefusesAnotherShellOnASharedConnectionGetsASeparateOne(t *testing.T) {
	world, options := newSharingWorld(t)
	state := newTestState(t, options)
	world.connectFirst(t, state)

	// The borrowed connection is full: its shell cannot be opened.
	state.Click(hitPaneSplitHorizontal)
	split, _ := state.sessionByIDPointer(state.activeSessionID)
	if !split.shared {
		t.Fatal("the split was not shared")
	}
	world.shared[0].mu.Lock()
	world.shared[0].shellErr = errors.New("ssh: rejected: administratively prohibited (open failed)")
	world.shared[0].mu.Unlock()
	split.term.shell = nil
	split.term.ended = false
	split.avoidShare = false
	state.openShell(split)

	if len(world.conns) != 2 {
		t.Fatalf("dials = %d, want a separate connection after the refusal", len(world.conns))
	}
	if split.shared || split.err != nil || split.term.ended {
		t.Fatalf("the tab did not recover: shared %t, err %v, ended %t", split.shared, split.err, split.term.ended)
	}
}

func TestClosingTheTabThatDialedLeavesTheSharedTabAlive(t *testing.T) {
	world, options := newSharingWorld(t)
	state := newTestState(t, options)
	first := world.connectFirst(t, state)
	state.Click(hitPaneSplitHorizontal)
	shared := world.shared[0]
	firstID := state.sessions[0].ID
	state.closeSession(firstID)

	if first.canceled != 1 {
		t.Fatalf("the first connection was let go %d times", first.canceled)
	}
	if shared.canceled != 0 {
		t.Fatal("closing one tab let go of the other")
	}
	if len(state.sessions) != 1 || !state.sessions[0].shared {
		t.Fatalf("sessions = %+v", state.sessions)
	}
}
