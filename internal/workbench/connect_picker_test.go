package workbench

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestNewTabButtonAsksWhereToConnectAndMakesNoTabYet(t *testing.T) {
	state := newTestState(t, testOptions(t))
	before := len(state.deck.Workspaces(nil))
	state.Click(hitNewTab)
	if state.modal != modalCommandPalette || !state.commandPalette.connect {
		t.Fatalf("modal = %d connect = %v", state.modal, state.commandPalette.connect)
	}
	if got := len(state.deck.Workspaces(nil)); got != before {
		t.Fatalf("the tab was created before a choice: %d -> %d", before, got)
	}
	var canvas ui.Canvas
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	if _, ok := hitByID(&canvas, hitCommandPaletteItems.At(0)); !ok {
		t.Fatal("the picker lists nothing")
	}
	// Closing it creates nothing either.
	state.Click(hitCommandPaletteClose)
	if state.modal != modalNone || len(state.deck.Workspaces(nil)) != before {
		t.Fatalf("after closing: modal %d tabs %d", state.modal, len(state.deck.Workspaces(nil)))
	}
	if state.commandPalette.connect {
		t.Fatal("the next palette would open as the picker")
	}
}

func TestPickingAHostCreatesANewTabForIt(t *testing.T) {
	state := newTestState(t, testOptions(t))
	before := len(state.deck.Workspaces(nil))
	host := state.hosts[0]
	state.Click(hitNewTab)
	// Local terminal is first, then the saved hosts in order.
	state.Click(hitCommandPaletteItems.At(1))
	if state.modal != modalNone || len(state.deck.Workspaces(nil)) != before+1 {
		t.Fatalf("modal %d, tabs %d -> %d", state.modal, before, len(state.deck.Workspaces(nil)))
	}
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || session.HostID != host.ID || state.activeSection != sectionTerminal {
		t.Fatalf("active session %+v ok=%v section=%v", session, ok, state.activeSection)
	}
}

func TestPickingThisComputerOpensALocalShellInANewTab(t *testing.T) {
	state := newTestState(t, testOptions(t))
	before := len(state.deck.Workspaces(nil))
	state.Click(hitNewTab)
	state.Click(hitCommandPaletteItems.At(0))
	session, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || !session.isLocal() || len(state.deck.Workspaces(nil)) != before+1 {
		t.Fatalf("local ok=%v tabs %d -> %d", ok, before, len(state.deck.Workspaces(nil)))
	}
}

func TestPickerSearchMatchesHostNamesAndAddresses(t *testing.T) {
	state := newTestState(t, testOptions(t))
	state.Click(hitNewTab)
	state.commandPalette.search.Insert(state.hosts[0].Address)
	state.refreshCommandPalette()
	palette := &state.commandPalette
	if palette.visibleCount == 0 {
		t.Fatal("searching a host's address finds nothing")
	}
	if got := palette.entries[palette.visible[0]].target; got != hitConnectPickList.At(int(state.hosts[0].ID)) {
		t.Fatalf("first match target = %d", got)
	}
	state.commandPalette.search.Insert("zzzz-no-such-host")
	state.refreshCommandPalette()
	if palette.visibleCount != 0 {
		t.Fatalf("%d matches for a nonsense query", palette.visibleCount)
	}
}

func TestTheCommandPaletteIsStillTheCommandPalette(t *testing.T) {
	state := newTestState(t, testOptions(t))
	state.Click(hitNewTab)
	state.Click(hitCommandPaletteClose)
	state.openCommandPalette()
	if state.commandPalette.connect {
		t.Fatal("the palette opened as the picker")
	}
	found := false
	for index := uint8(0); index < state.commandPalette.entryCount; index++ {
		found = found || state.commandPalette.entries[index].target == hitSettings
	}
	if !found {
		t.Fatal("the commands are missing from the palette")
	}
}
