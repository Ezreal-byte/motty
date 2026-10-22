package workbench

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func rightClickHost(state *State, host uint32) {
	pointer(state, ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, Target: hitHostList.At(int(host)), X: 400, Y: 300})
}

func TestRightClickOnAHostOpensItsMenuWithoutChangingWhatIsChecked(t *testing.T) {
	world, options := newWorld(t)
	state := newTestState(t, options)
	first, second := state.hosts[0].ID, state.hosts[1].ID
	state.Click(hitHostCheckList.At(int(first)))

	rightClickHost(state, second)
	if !state.hostMenu.open || state.hostMenu.host != second {
		t.Fatalf("menu = %+v", state.hostMenu)
	}
	if len(state.hostSelections) != 1 || state.hostSelections[0] != first || state.selectedHostID != second {
		t.Fatalf("selection = %v selected %d, want the original check and the pointed host highlighted", state.hostSelections, state.selectedHostID)
	}
	canvas, _ := paintedTexts(state)
	for _, id := range []ui.ID{hitHostMenuConnect, hitHostMenuEdit, hitHostMenuCopy, hitHostMenuMove, hitHostMenuDelete} {
		if hit, ok := hitByID(canvas, id); !ok || hit.Disabled {
			t.Errorf("menu item %d: %+v found=%v", id, hit, ok)
		}
	}
	state.Click(hitHostMenuConnect)
	if state.hostMenu.open || len(world.conns) != 1 {
		t.Fatalf("Connect: open=%v dialed=%d", state.hostMenu.open, len(world.conns))
	}
}

func TestHostMenuEditsCopiesAndDeletesTheHostPointedAt(t *testing.T) {
	state := newTestState(t, testOptions(t))
	target := state.hosts[1]
	checked := state.hosts[0].ID
	state.Click(hitHostCheckList.At(int(checked)))

	rightClickHost(state, target.ID)
	state.Click(hitHostMenuEdit)
	if state.modal != modalNewHost || state.hostDialogMode != hostDialogEdit || state.hostDialogID != target.ID {
		t.Fatalf("Edit: modal %v mode %v id %d", state.modal, state.hostDialogMode, state.hostDialogID)
	}
	state.Click(hitDialogCancel)

	rightClickHost(state, target.ID)
	state.Click(hitHostMenuCopy)
	if state.modal != modalNewHost || state.hostDialogMode != hostDialogCopy {
		t.Fatalf("Copy: modal %v mode %v", state.modal, state.hostDialogMode)
	}
	state.Click(hitDialogCancel)

	rightClickHost(state, target.ID)
	state.Click(hitHostMenuDelete)
	if state.modal != modalDeleteHost || len(state.hostActionIDs) != 1 || state.hostActionIDs[0] != target.ID {
		t.Fatalf("Delete: modal %v ids %v", state.modal, state.hostActionIDs)
	}
	if len(state.hostSelections) != 1 || state.hostSelections[0] != checked {
		t.Fatalf("context actions changed the checked hosts to %v", state.hostSelections)
	}
}

func TestHostMenuMovesOnlyTheUncheckedHostPointedAt(t *testing.T) {
	state := newTestState(t, testOptions(t))
	checked, target := state.hosts[0].ID, state.hosts[1].ID
	state.Click(hitHostCheckList.At(int(checked)))

	rightClickHost(state, target)
	state.Click(hitHostMenuMove)
	if !state.hostGroupMenu || len(state.hostActionIDs) != 1 || state.hostActionIDs[0] != target {
		t.Fatalf("Move: open=%v ids=%v", state.hostGroupMenu, state.hostActionIDs)
	}
	if len(state.hostSelections) != 1 || state.hostSelections[0] != checked {
		t.Fatalf("Move changed the checked hosts to %v", state.hostSelections)
	}
}

func TestRightClickInsideSeveralCheckedHostsKeepsThemAndDisablesTheSingleOnes(t *testing.T) {
	state := newTestState(t, testOptions(t))
	a, b := state.hosts[0].ID, state.hosts[1].ID
	state.Click(hitHostCheckList.At(int(a)))
	state.Click(hitHostCheckList.At(int(b)))

	rightClickHost(state, b)
	if len(state.hostSelections) != 2 {
		t.Fatalf("selection = %v, want both kept", state.hostSelections)
	}
	canvas, _ := paintedTexts(state)
	for id, disabled := range map[ui.ID]bool{hitHostMenuConnect: true, hitHostMenuEdit: true, hitHostMenuCopy: true, hitHostMenuMove: false, hitHostMenuDelete: false} {
		if hit, ok := hitByID(canvas, id); !ok || hit.Disabled != disabled {
			t.Errorf("item %d: found=%v disabled=%v, want disabled=%v", id, ok, hit.Disabled, disabled)
		}
	}
	state.Click(hitHostMenuDelete)
	if len(state.hostActionIDs) != 2 {
		t.Fatalf("Delete acts on %v, want both", state.hostActionIDs)
	}
}

func TestHostMenuClosesOnAnOutsidePressAndIgnoresOtherRightClicks(t *testing.T) {
	state := newTestState(t, testOptions(t))
	rightClickHost(state, state.hosts[0].ID)
	state.Click(hitHostMenuDismiss)
	if state.hostMenu.open {
		t.Fatal("an outside press left the menu open")
	}
	pointer(state, ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, Target: hitSearch, X: 50, Y: 50})
	if state.hostMenu.open {
		t.Fatal("a right click away from a host opened the menu")
	}
	rightClickHost(state, state.hosts[0].ID)
	if result := pointer(state, ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonSecondary, Target: hitHostMenuDismiss}); !result.Handled || state.hostMenu.open {
		t.Fatalf("a secondary press outside: %+v open=%v", result, state.hostMenu.open)
	}
}

func TestHostMenuIsAsWideAsItsLabelsAskInBothLanguages(t *testing.T) {
	for _, chinese := range []bool{false, true} {
		state := newTestState(t, testOptions(t))
		if chinese {
			state.Click(hitLanguage)
		}
		rightClickHost(state, state.hosts[0].ID)
		canvas, _ := paintedTexts(state)
		hit, ok := hitByID(canvas, hitHostMenuMove)
		if !ok {
			t.Fatal("no menu")
		}
		widest := canvas.TextWidth(state.messages.MoveHostGroup, 13)
		// A context menu is never narrower than Vega's 9rem, whatever its labels.
		if hit.Bounds.Width > max(widest+80, 144) || hit.Bounds.Width < widest+8 {
			t.Errorf("chinese=%v: a row is %.0f wide for a %.0f wide label", chinese, hit.Bounds.Width, widest)
		}
	}
}

// Typing a row's first letters highlights it, and typing the same letter again
// cycles to the next row with that initial, as Base UI menus do.
func TestHostMenuTypeaheadHighlightsRowsByName(t *testing.T) {
	state := newTestState(t, testOptions(t))
	rightClickHost(state, state.hosts[0].ID)
	paintAt(state, ui.Size{Width: 900, Height: 600}) // list the menu's rows for the keyboard

	key := func(text string) ui.ID {
		t.Helper()
		result := state.Key(ui.KeyEvent{Key: ui.KeyUnknown, Text: text})
		if !result.Handled || result.Focus.IsZero() {
			t.Fatalf("%q: %+v", text, result)
		}
		return result.Focus
	}
	if got := key("c"); got != hitHostMenuConnect {
		t.Fatalf("c focused %v, want Connect", got)
	}
	if got := key("c"); got != hitHostMenuCopy {
		t.Fatalf("second c focused %v, want Copy Host", got)
	}
}

// Choosing Move group hands the menu over to the group picker: the picker takes
// the menu's place, at the point the menu was opened at, rather than opening in
// the window's top left corner.
func TestMoveGroupFromAHostsMenuOpensThePickerWhereTheMenuWas(t *testing.T) {
	state := newTestState(t, testOptions(t))
	rightClickHost(state, state.hosts[1].ID)
	paintedTexts(state)
	menu := state.popups.surface
	if menu.X <= 0 || menu.Y <= 0 {
		t.Fatalf("the host menu was not opened at the pointer: %+v", menu)
	}
	state.Click(hitHostMenuMove)
	paintedTexts(state)
	if picker := state.popups.surface; picker.X != menu.X || picker.Y != menu.Y {
		t.Fatalf("the group picker opened at %+v, want the menu's place %+v", picker, menu)
	}
}

// A context menu keeps Tab inside itself, as Base UI's modal context menu does;
// only dropdowns and selects hand Tab back to the page.
func TestTabStaysInsideAContextMenu(t *testing.T) {
	state := newTestState(t, testOptions(t))
	rightClickHost(state, state.hosts[0].ID)
	paintAt(state, ui.Size{Width: 900, Height: 600})
	state.Key(ui.KeyEvent{Key: ui.KeyTab, Target: hitHostMenuConnect})
	if !state.hostMenu.open {
		t.Fatal("Tab left the context menu")
	}
}
