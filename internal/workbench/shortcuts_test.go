package workbench

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestShortcutCaptureRejectsConflictsAndPersistsTypedBinding(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	showShortcutSettings(state)
	state.Click(hitShortcutRowList.At(int(shortcutFind - 1)))
	if state.modal != modalShortcutCapture || state.shortcutCapture != shortcutFind {
		t.Fatalf("shortcut row did not open capture: modal=%d action=%d", state.modal, state.shortcutCapture)
	}
	state.clickShortcutCapture(hitShortcutCaptureCancel)

	state.openShortcutCapture(shortcutFind)
	palette := state.shortcuts.chords[shortcutCommandPalette]
	result := state.Shortcut(ui.KeyEvent{Modifiers: palette.Modifiers, Text: string([]byte{palette.Key})})
	if !result.Handled || !result.Repaint || state.modal != modalShortcutCapture || state.shortcutError == "" {
		t.Fatalf("conflict result=%+v modal=%d error=%q", result, state.modal, state.shortcutError)
	}
	if state.shortcuts.chords[shortcutFind] == palette {
		t.Fatal("conflicting shortcut replaced the find binding")
	}

	custom := shortcutChord{Modifiers: defaultShortcutModifier() | ui.ModifierAlt, Key: 'x'}
	result = state.Shortcut(ui.KeyEvent{Modifiers: custom.Modifiers, Text: "x"})
	if !result.Handled || state.modal != modalNone || state.shortcuts.chords[shortcutFind] != custom {
		t.Fatalf("custom result=%+v modal=%d chord=%+v", result, state.modal, state.shortcuts.chords[shortcutFind])
	}

	restored := newTestState(t, options)
	if restored.shortcuts.chords[shortcutFind] != custom || restored.activeSection != sectionSettings || restored.settingsCategory != settingsShortcuts {
		t.Fatalf("restored chord=%+v section=%d", restored.shortcuts.chords[shortcutFind], restored.activeSection)
	}
	if got := restored.Shortcut(ui.KeyEvent{Modifiers: defaultShortcutModifier(), Text: "f"}); got.Handled {
		t.Fatal("old find binding remained active")
	}
	if got := restored.Shortcut(ui.KeyEvent{Modifiers: custom.Modifiers, Text: "x"}); !got.Handled {
		t.Fatal("custom find binding was not active")
	}
}

func TestShortcutSettingsSteadyPaintDoesNotAllocate(t *testing.T) {
	state := newTestState(t, testOptions(t))
	showShortcutSettings(state)
	var canvas ui.Canvas
	size := ui.Size{Width: 1400, Height: 900}
	state.Paint(&canvas, size)
	allocations := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		state.Paint(&canvas, size)
	})
	if allocations != 0 {
		t.Fatalf("shortcut settings paint allocations = %v", allocations)
	}
}

func BenchmarkPaintShortcutSettings(b *testing.B) {
	state := newTestState(b, testOptions(b))
	showShortcutSettings(state)
	var canvas ui.Canvas
	size := ui.Size{Width: 1400, Height: 900}
	state.Paint(&canvas, size)
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		state.Paint(&canvas, size)
	}
}

// showShortcutSettings opens the Keyboard shortcuts category of the settings page.
func showShortcutSettings(state *State) {
	state.activeSection = sectionSettings
	state.settingsCategory = settingsShortcuts
}
