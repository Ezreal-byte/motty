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

func TestShortcutFilesPathHasPrimaryLDefaultAndPersistsCustomBinding(t *testing.T) {
	options := freshOptions(t)
	state := newTestState(t, options)
	wantDefault := shortcutChord{Modifiers: defaultShortcutModifier(), Key: 'l'}
	if state.shortcuts.chords[shortcutFilesPath] != wantDefault {
		t.Fatalf("default files path shortcut=%+v, want %+v", state.shortcuts.chords[shortcutFilesPath], wantDefault)
	}
	custom := shortcutChord{Modifiers: defaultShortcutModifier() | ui.ModifierAlt, Key: 'l'}
	if conflict, ok := state.shortcuts.bind(shortcutFilesPath, custom); !ok || conflict != shortcutNone {
		t.Fatalf("bind custom files path shortcut conflict=%v ok=%v", conflict, ok)
	}
	state.save()
	restored := newTestState(t, options)
	if restored.shortcuts.chords[shortcutFilesPath] != custom || shortcutActionFromString("files_path") != shortcutFilesPath {
		t.Fatalf("restored files path shortcut=%+v action=%v", restored.shortcuts.chords[shortcutFilesPath], shortcutActionFromString("files_path"))
	}
}

func TestShortcutFilesPathLeavesTerminalCtrlLAloneWithoutAVisibleFilePanel(t *testing.T) {
	state, _, _, _ := liveSession(t)
	session, _ := state.sessionByIDPointer(state.activeSessionID)
	shortcut := state.shortcuts.chords[shortcutFilesPath]
	event := ui.KeyEvent{Target: state.activeTerminalFocusHit(), Modifiers: shortcut.Modifiers, Text: string(shortcut.Key)}
	if result := state.Shortcut(event); result.Handled {
		t.Fatalf("Ctrl/Cmd+L without Files was consumed: %+v", result)
	}

	state.Click(hitSFTPOpen)
	if result := state.Shortcut(event); result.Handled || session.files.sftpFolder.pathEditing {
		t.Fatalf("Ctrl/Cmd+L with terminal focused and visible Files result=%+v editing=%v", result, session.files.sftpFolder.pathEditing)
	}
	context, _ := state.deck.Context(state.deck.ActiveWorkspace())
	context.Open = false
	if err := state.deck.SetContext(state.deck.ActiveWorkspace(), context); err != nil {
		t.Fatal(err)
	}
	if session.files == nil {
		t.Fatal("test needs retained file state behind a hidden panel")
	}
	if result := state.Shortcut(event); result.Handled || session.files.sftpFolder.pathEditing {
		t.Fatalf("Ctrl/Cmd+L with hidden Files result=%+v editing=%v", result, session.files.sftpFolder.pathEditing)
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
