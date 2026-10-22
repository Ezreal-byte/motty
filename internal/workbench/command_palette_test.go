package workbench

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestCommandPaletteIdentitiesAreItsOwn(t *testing.T) {
	// The palette's controls and its entries are names in a place of its own, so
	// nothing else answers to them, and an entry the palette cannot list has none.
	for _, id := range []ui.ID{hitCommandPalette, hitCommandPaletteDismiss, hitCommandPaletteClose, hitCommandPaletteSearch, hitCommandPaletteItems.At(0), hitCommandPaletteItems.At(maxCommandPaletteEntries - 1)} {
		if !tree.Within(id.Scope(), commandPaletteHits) {
			t.Fatalf("identity %s is outside the palette's place", tree.Text(id))
		}
	}
}

func TestCommandPaletteFiltersAndExecutesCommands(t *testing.T) {
	state := newTestState(t, testOptions(t))
	state.Click(hitCommandPalette)
	if state.modal != modalCommandPalette || state.EditorFor(hitCommandPaletteSearch) == nil {
		t.Fatal("command palette did not open with a native text editor")
	}
	app.TextEvent{Kind: app.TextInsert, Text: "settings"}.Apply(state.EditorFor(hitCommandPaletteSearch))
	var canvas ui.Canvas
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	if state.commandPalette.visibleCount != 1 {
		t.Fatalf("visible commands = %d", state.commandPalette.visibleCount)
	}
	if hit, ok := hitByID(&canvas, hitCommandPaletteItems.At(0)); !ok || hit.Label != state.messages.NavSettings {
		t.Fatalf("settings command hit = %+v, ok=%v", hit, ok)
	}
	result := state.Key(ui.KeyEvent{Target: hitCommandPaletteSearch, Key: ui.KeyEnter})
	if !result.Handled || state.modal != modalNone || state.activeSection != sectionSettings {
		t.Fatalf("execute result=%+v modal=%d section=%d", result, state.modal, state.activeSection)
	}
}

func TestCommandPaletteKeyboardNavigationAndSessionSwitch(t *testing.T) {
	state := newTestState(t, testOptions(t))
	state.sessions = append(state.sessions,
		Session{ID: 1, Title: "Alpha", restored: true},
		Session{ID: 2, Title: "Beta", restored: true},
	)
	state.activeSessionID = 1
	state.nextSessionID = 3
	state.Click(hitCommandPalette)
	app.TextEvent{Kind: app.TextInsert, Text: "Beta"}.Apply(state.EditorFor(hitCommandPaletteSearch))
	state.refreshCommandPalette()
	if state.commandPalette.visibleCount != 1 {
		t.Fatalf("visible session commands = %d", state.commandPalette.visibleCount)
	}
	result := state.Key(ui.KeyEvent{Target: hitCommandPaletteItems.At(0), Key: ui.KeyEnter})
	if !result.Handled || state.activeSessionID != 2 || state.activeSection != sectionTerminal || state.modal != modalNone {
		t.Fatalf("session switch result=%+v active=%d section=%d modal=%d", result, state.activeSessionID, state.activeSection, state.modal)
	}
}

func TestCommandPaletteShortcutOpensFromTerminal(t *testing.T) {
	state := newTestState(t, testOptions(t))
	result := state.Shortcut(ui.KeyEvent{Text: "p", Modifiers: defaultShortcutModifier() | ui.ModifierShift})
	if !result.Handled || !result.Repaint || result.Focus != hitCommandPaletteSearch || state.modal != modalCommandPalette {
		t.Fatalf("shortcut result=%+v modal=%d", result, state.modal)
	}
}

func TestNewWindowCommandAndShortcutUseTheHostLifecycle(t *testing.T) {
	opened := 0
	options := testOptions(t)
	options.NewWindow = func() error { opened++; return nil }
	state := newTestState(t, options)
	state.Click(hitCommandPalette)
	app.TextEvent{Kind: app.TextInsert, Text: "new window"}.Apply(state.EditorFor(hitCommandPaletteSearch))
	state.refreshCommandPalette()
	if state.commandPalette.visibleCount != 1 {
		t.Fatalf("visible commands = %d", state.commandPalette.visibleCount)
	}
	state.Key(ui.KeyEvent{Target: hitCommandPaletteSearch, Key: ui.KeyEnter})
	if opened != 1 {
		t.Fatalf("palette opened %d windows", opened)
	}
	result := state.Shortcut(ui.KeyEvent{Text: "n", Modifiers: defaultShortcutModifier() | ui.ModifierShift})
	if !result.Handled || opened != 2 {
		t.Fatalf("shortcut result=%+v opened=%d", result, opened)
	}
}

func TestCommandPalettePaintDoesNotAllocateOnceWarm(t *testing.T) {
	state := newTestState(t, testOptions(t))
	state.Click(hitCommandPalette)
	app.TextEvent{Kind: app.TextInsert, Text: "SET"}.Apply(state.EditorFor(hitCommandPaletteSearch))
	var canvas ui.Canvas
	size := ui.Size{Width: 1400, Height: 900}
	state.Paint(&canvas, size)
	if allocations := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		state.Paint(&canvas, size)
	}); allocations != 0 {
		t.Fatalf("command palette paint allocations = %v", allocations)
	}
	if _, scheduled := canvas.NextFrame(); scheduled {
		t.Fatal("idle command palette scheduled another frame")
	}
}

func BenchmarkPaintCommandPalette(b *testing.B) {
	state := newTestState(b, testOptions(b))
	state.Click(hitCommandPalette)
	app.TextEvent{Kind: app.TextInsert, Text: "set"}.Apply(state.EditorFor(hitCommandPaletteSearch))
	var canvas ui.Canvas
	size := ui.Size{Width: 1400, Height: 900}
	state.Paint(&canvas, size)
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		state.Paint(&canvas, size)
	}
}

func TestCommandPaletteReachesEntriesBelowTheTenRowsItShows(t *testing.T) {
	state := newTestState(t, testOptions(t))
	// With a session open the palette lists its commands too: more than ten.
	for id := uint32(1); id <= 3; id++ {
		state.sessions = append(state.sessions, Session{ID: id, Title: "Box", restored: true})
	}
	state.activeSessionID, state.nextSessionID = 1, 4
	state.Click(hitCommandPalette)
	var canvas ui.Canvas
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	total := int(state.commandPalette.visibleCount)
	if total <= commandPaletteVisibleRows {
		t.Fatalf("only %d entries; the test needs more than the %d rows shown", total, commandPaletteVisibleRows)
	}
	last := hitCommandPaletteItems.At(commandPaletteVisibleRows - 1)
	if _, ok := hitByID(&canvas, hitCommandPaletteItems.At(commandPaletteVisibleRows)); ok {
		t.Fatal("an eleventh row is painted before anything scrolled")
	}
	result := state.Key(ui.KeyEvent{Target: hitCommandPaletteItems.At(0), Key: ui.KeyEnd})
	if !result.Handled || result.Focus != last {
		t.Fatalf("End: %+v, want focus on the last visible row %v", result, last)
	}
	canvas.Reset()
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	hit, ok := hitByID(&canvas, last)
	if !ok || state.commandPalette.selected != uint8(total-1) {
		t.Fatalf("after End: last row painted %v, selected %d of %d", ok, state.commandPalette.selected, total)
	}
	if want := state.commandPalette.entries[state.commandPalette.visible[total-1]].label; hit.Label != want {
		t.Fatalf("the last row is %q, want the last entry %q", hit.Label, want)
	}
	// The wheel moves the list back, and a click on a row runs the entry that row shows.
	if r := state.Scroll(ui.ScrollEvent{DeltaY: -1000, Target: hitCommandPaletteItems.At(0)}); !r.Repaint || state.commandPalette.top != 0 {
		t.Fatalf("wheel up: %+v top=%d", r, state.commandPalette.top)
	}
	state.Scroll(ui.ScrollEvent{DeltaY: 1000})
	state.Click(hitCommandPaletteItems.At(commandPaletteVisibleRows - 1))
	if state.modal != modalNone || state.activeSessionID != 3 {
		t.Fatalf("clicking the last row: modal=%d active session %d, want the last session", state.modal, state.activeSessionID)
	}
}
