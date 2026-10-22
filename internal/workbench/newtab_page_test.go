package workbench

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func newTabState(t *testing.T) *State {
	t.Helper()
	state := newTestState(t, testOptions(t))
	openEmptyTab(state)
	return state
}

func TestNewTabPageOffersThisComputerAndTheSavedHosts(t *testing.T) {
	state := newTabState(t)
	var canvas ui.Canvas
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	if _, ok := hitByID(&canvas, hitNewTabLocal); !ok {
		t.Fatal("no row for this computer")
	}
	for index := range min(len(state.hosts), newTabMaxHosts) {
		if _, ok := hitByID(&canvas, hitNewTabHostList.At(index)); !ok {
			t.Errorf("no row for host %d", index)
		}
	}
	if _, ok := hitByID(&canvas, hitPaneChooseHost); !ok {
		t.Fatal("no way to the full host list")
	}
}

func TestNewTabPageConnectsAHostInThisTab(t *testing.T) {
	state := newTabState(t)
	before := len(state.workspaceTabs)
	workspace := state.deck.ActiveWorkspace()
	host := state.hosts[0]
	state.Click(hitNewTabHostList.At(0))
	if len(state.workspaceTabs) != before || state.deck.ActiveWorkspace() != workspace {
		t.Fatalf("tabs %d -> %d: choosing a host must connect in the tab it was chosen in", before, len(state.workspaceTabs))
	}
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || session.HostID != host.ID {
		t.Fatalf("active session = %+v, want host %d", session, host.ID)
	}
	if state.activeSection != sectionTerminal {
		t.Fatalf("section = %v", state.activeSection)
	}
}

func TestNewTabPageOpensALocalShellInThisTab(t *testing.T) {
	state := newTabState(t)
	before := len(state.workspaceTabs)
	state.Click(hitNewTabLocal)
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || !session.isLocal() || len(state.workspaceTabs) != before {
		t.Fatalf("local session ok=%v tabs %d -> %d", ok, before, len(state.workspaceTabs))
	}
}

func TestNewTabPageFitsASmallPane(t *testing.T) {
	state := newTabState(t)
	var canvas ui.Canvas
	state.Paint(&canvas, ui.Size{Width: 700, Height: 420})
	for _, id := range []ui.ID{hitNewTabLocal, hitPaneChooseHost} {
		hit, ok := hitByID(&canvas, id)
		if !ok || hit.Bounds.Y+hit.Bounds.Height > 420 || hit.Bounds.X < 0 || hit.Bounds.X+hit.Bounds.Width > 700 {
			t.Fatalf("control %d outside a 700x420 window: %+v, %v", id, hit.Bounds, ok)
		}
	}
}
