package workbench

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func pressChord(state *State, chord shortcutChord) ui.EventResult {
	return state.Shortcut(ui.KeyEvent{Modifiers: chord.Modifiers, Text: string([]byte{chord.Key})})
}

func TestASnippetShortcutSendsItFromTheTerminalAndIsKeptOnThisDevice(t *testing.T) {
	world, options := newSharingWorld(t)
	state := newTestState(t, options)
	world.connectFirst(t, state)
	shell := world.conns[0].shell(t)
	chord := shortcutChord{Modifiers: ui.ModifierMeta | ui.ModifierShift, Key: '1'}

	state.Click(hitSettings)
	state.Click(hitSettingsSnippetAdd)
	typeInto(state, hitSnippetName, "Restart nginx")
	typeInto(state, hitSnippetCommand, "systemctl restart nginx")
	state.Click(hitSnippetShortcut)
	if !state.snippets.capturing {
		t.Fatal("the shortcut button did not start waiting for a chord")
	}
	pressChord(state, shortcutChord{Modifiers: ui.ModifierControl, Key: 'r'})
	if !state.snippets.capturing || state.snippets.dialogError != state.messages.Tools.SnippetShortcutInvalid {
		t.Fatalf("Ctrl+R alone belongs to the terminal: capturing=%v error=%q", state.snippets.capturing, state.snippets.dialogError)
	}
	taken := state.shortcuts.chords[shortcutFind]
	pressChord(state, taken)
	if !state.snippets.capturing || state.snippets.dialogError == "" {
		t.Fatalf("a built-in chord was taken: capturing=%v error=%q", state.snippets.capturing, state.snippets.dialogError)
	}
	pressChord(state, chord)
	if state.snippets.capturing || state.snippets.shortcut != chord || state.snippets.dialogError != "" {
		t.Fatalf("capturing=%v shortcut=%+v error=%q", state.snippets.capturing, state.snippets.shortcut, state.snippets.dialogError)
	}
	state.Click(hitSnippetSubmit)
	if got := state.snippets.list[0].Shortcut; got != chord {
		t.Fatalf("saved shortcut = %+v", got)
	}

	// On the Settings page the chord must not type into a terminal that is out of sight.
	if got := pressChord(state, chord); got.Handled || shell.typed() != "" {
		t.Fatalf("settings page: handled=%v, shell heard %q", got.Handled, shell.typed())
	}
	state.activeSection = sectionTerminal
	if got := pressChord(state, chord); !got.Handled || shell.typed() != "systemctl restart nginx\r" {
		t.Fatalf("handled=%v, shell heard %q", got.Handled, shell.typed())
	}
	held := ui.KeyEvent{Modifiers: chord.Modifiers, Text: "1", Repeat: true}
	if got := state.Shortcut(held); !got.Handled || shell.typed() != "systemctl restart nginx\r" {
		t.Fatalf("a held key sent the snippet again: heard %q", shell.typed())
	}

	// A built-in shortcut cannot be moved onto it.
	state.openShortcutCapture(shortcutFind)
	pressChord(state, chord)
	if state.modal != modalShortcutCapture || state.shortcutError == "" || state.shortcuts.chords[shortcutFind] != taken {
		t.Fatalf("modal=%d error=%q find=%+v", state.modal, state.shortcutError, state.shortcuts.chords[shortcutFind])
	}
	state.clickShortcutCapture(hitShortcutCaptureCancel)

	if restarted := newTestState(t, options); len(restarted.snippets.list) != 1 || restarted.snippets.list[0].Shortcut != chord {
		t.Fatalf("restored snippets = %+v", restarted.snippets.list)
	}
	state.Click(hitSnippetEditList.At(0))
	state.Click(hitSnippetShortcutOff)
	state.Click(hitSnippetSubmit)
	if got := pressChord(state, chord); got.Handled {
		t.Fatal("a cleared shortcut still fired")
	}
	if restarted := newTestState(t, options); restarted.snippets.list[0].Shortcut.valid() {
		t.Fatalf("a cleared shortcut came back: %+v", restarted.snippets.list[0])
	}
}

func TestAChordTwoSnippetsWantGoesToTheFirst(t *testing.T) {
	_, options := newWorld(t)
	state := newTestState(t, options)
	chord := shortcutChord{Modifiers: ui.ModifierMeta | ui.ModifierShift, Key: '2'}
	state.snippets.list = []snippet{{Name: "a", Command: "ls", Shortcut: chord}, {Name: "b", Command: "pwd"}}

	state.openSnippetDialog(1)
	state.Click(hitSnippetShortcut)
	pressChord(state, chord)
	if !state.snippets.capturing || state.snippets.dialogError == "" || state.snippets.shortcut.valid() {
		t.Fatalf("a chord of snippet a was given to b: %+v", state.snippets)
	}
}

func TestResettingShortcutsUnbindsSnippetsTheyNowCollideWith(t *testing.T) {
	_, options := newWorld(t)
	state := newTestState(t, options)
	defaultPalette := state.shortcuts.chords[shortcutCommandPalette]
	moved := shortcutChord{Modifiers: defaultPalette.Modifiers | ui.ModifierAlt, Key: 'p'}
	if _, ok := state.shortcuts.bind(shortcutCommandPalette, moved); !ok {
		t.Fatal("could not move the command palette")
	}
	state.snippets.list = []snippet{{Name: "a", Command: "ls", Shortcut: defaultPalette}}
	state.save()
	if restarted := newTestState(t, options); restarted.snippets.list[0].Shortcut != defaultPalette {
		t.Fatalf("a free chord was not kept: %+v", restarted.snippets.list[0])
	}
	state.Click(hitShortcutReset)
	if state.snippets.list[0].Shortcut.valid() {
		t.Fatalf("snippet kept a chord the palette now uses: %+v", state.snippets.list[0])
	}
}
