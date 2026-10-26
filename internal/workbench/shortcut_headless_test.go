//go:build darwin || windows || linux

package workbench

import (
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
)

func TestHeadlessShortcutCaptureClickAndChord(t *testing.T) {
	state := newTestState(t, freshOptions(t))
	showShortcutSettings(state)
	session, err := headless.Open(app.Config{
		Title: "shortcuts", Width: 1400, Height: 900,
		Paint: state.Paint, OnPointer: state.Pointer,
		OnKey: state.Key, OnShortcut: state.Shortcut, OnResize: state.Resize,
	}, nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	label := state.shortcuts.labels[shortcutCommandPalette]
	if err := session.ClickLabel(label); err != nil {
		t.Fatal(err)
	}
	if state.modal != modalShortcutCapture || state.shortcutCapture != shortcutCommandPalette {
		t.Fatalf("shortcut click left modal=%d action=%d", state.modal, state.shortcutCapture)
	}

	if err := session.Keys("cmd+alt+x"); err != nil {
		t.Fatal(err)
	}
	want := shortcutChord{Modifiers: ui.ModifierMeta | ui.ModifierAlt, Key: 'x'}
	if state.shortcuts.chords[shortcutCommandPalette] != want || state.modal != modalNone {
		t.Fatalf("binding=%+v modal=%d, want %+v and closed", state.shortcuts.chords[shortcutCommandPalette], state.modal, want)
	}
}

// A shortcut row has no explanation, so its name is alone in the row and must sit
// in the middle of the row, level with the chord it names.
func TestHeadlessShortcutRowNameIsCenteredWithItsChord(t *testing.T) {
	state := newTestState(t, freshOptions(t))
	showShortcutSettings(state)
	session, err := headless.Open(app.Config{
		Title: "shortcuts", Width: 1400, Height: 900,
		Paint: state.Paint, OnPointer: state.Pointer,
		OnKey: state.Key, OnShortcut: state.Shortcut, OnResize: state.Resize,
	}, nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	texts := session.Snapshot().Texts
	centerY := func(value string) (float32, bool) {
		for _, text := range texts {
			if text.Value == value {
				return text.Bounds.Y + text.Bounds.Height/2, true
			}
		}
		return 0, false
	}
	name, chord := state.shortcutActionLabel(shortcutCommandPalette), state.shortcuts.labels[shortcutCommandPalette]
	nameY, ok := centerY(name)
	if !ok {
		t.Fatalf("shortcut name %q was not painted", name)
	}
	chordY, ok := centerY(chord)
	if !ok {
		t.Fatalf("shortcut chord %q was not painted", chord)
	}
	if difference := nameY - chordY; difference < -1 || difference > 1 {
		t.Fatalf("name center %.1f, chord center %.1f: the name is not centered in its row", nameY, chordY)
	}
}

// A dialog fades out after it is closed and the window then goes idle: no
// transition is left in flight, and the dialog no longer takes input.
func TestHeadlessClosedDialogLeavesAndTheWindowGoesIdle(t *testing.T) {
	state := newTestState(t, freshOptions(t))
	showShortcutSettings(state)
	session, err := headless.Open(app.Config{
		Title: "shortcuts", Width: 1400, Height: 900,
		Paint: state.Paint, OnPointer: state.Pointer,
		OnKey: state.Key, OnShortcut: state.Shortcut, OnResize: state.Resize,
	}, nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	if err := session.ClickLabel(state.shortcuts.labels[shortcutCommandPalette]); err != nil {
		t.Fatal(err)
	}
	if err := session.Rest(); err != nil {
		t.Fatal(err)
	}
	if session.Driver().Transitioning() {
		t.Fatal("an open dialog at rest still asks for frames")
	}
	if err := session.Keys("esc"); err != nil {
		t.Fatal(err)
	}
	if state.modal != modalNone {
		t.Fatalf("Escape left modal=%d", state.modal)
	}
	if !session.Driver().Transitioning() {
		t.Fatal("a closed dialog did not leave through a transition")
	}
	for _, hit := range session.Snapshot().Hits {
		if hit.Scope == dialogScope {
			t.Fatalf("the leaving dialog still declares %q", hit.Label)
		}
	}
	if err := session.Wait(time.Second); err != nil {
		t.Fatal(err)
	}
	if session.Driver().Transitioning() {
		t.Fatal("the window is still animating a second after the dialog closed")
	}
}

// The chord reaches the application through the real key path, not just
// State.Shortcut, and the window then shows the new local terminal.
func TestHeadlessLocalTerminalShortcutOpensItsTab(t *testing.T) {
	skipWithoutUnixShell(t)
	state := newTestState(t, freshOptions(t))
	state.options.Invalidate = func() {}
	session, err := headless.Open(app.Config{
		Title: "local terminal", Width: 1400, Height: 900,
		Paint: state.Paint, OnPointer: state.Pointer,
		OnKey: state.Key, OnShortcut: state.Shortcut, OnResize: state.Resize,
		TextInput: state.TerminalInput,
	}, nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	modifier := "ctrl"
	if defaultShortcutModifier()&ui.ModifierMeta != 0 {
		modifier = "cmd"
	}
	if err := session.Keys(modifier + "+shift+t"); err != nil {
		t.Fatal(err)
	}
	opened, found := state.sessionByIDPointer(state.activeSessionID)
	if !found || !opened.isLocal() {
		t.Fatalf("session after the shortcut = %+v", opened)
	}
	t.Cleanup(func() {
		if opened.term != nil && opened.term.shell != nil {
			opened.term.shell.Close()
		}
	})
	if err := session.WaitFor(func(snapshot app.Snapshot) bool {
		for _, text := range snapshot.Texts {
			if text.Value == state.messages.LocalTerminalTitle {
				return true
			}
		}
		return false
	}, 3*time.Second); err != nil {
		t.Fatal(err)
	}
}
