//go:build darwin || windows || linux

package workbench

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
)

func TestHeadlessSynchronizedUpdatePresentsTheScreenBeforeTheBlock(t *testing.T) {
	state, _, _, shell := liveSession(t)
	var input app.TextInput
	resolveInput := func(context app.TextInputContext) *app.TextInput {
		if context.Terminal {
			return state.TerminalInput(context)
		}
		input.Editor = state.EditorFor(context.ID)
		if input.Editor == nil {
			return nil
		}
		return &input
	}
	var invalidator app.Invalidator
	session, err := headless.Open(app.Config{
		Title: "synchronized update", Width: 1400, Height: 900,
		Paint: state.Paint, TextInput: resolveInput,
		OnPointer: state.Pointer, OnKey: state.Key,
		OnShortcut: state.Shortcut, OnResize: state.Resize,
		Invalidator: &invalidator, OnWake: state.Wake,
	}, nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	terminalLabel := fmt.Sprintf(state.messages.LabelTerminalFmt, state.sessions[0].Address)
	if err := session.ClickLabel(terminalLabel); err != nil {
		t.Fatal(err)
	}

	shell.say("before")
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if !containsExact(session.Texts(), "before") {
		t.Fatalf("headless texts do not contain the first screen: %q", session.Texts())
	}

	// BSU, clear and replacement text arrive in one write. The presentation
	// keeps the screen from before BSU until ESU.
	shell.say("\x1b[?2026h\x1b[2J\x1b[Hafter")
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if containsExact(session.Texts(), "after") {
		t.Fatalf("a synchronized update leaked its new screen: %q", session.Texts())
	}
	if !containsExact(session.Texts(), "before") {
		t.Fatalf("the held screen is missing: %q", session.Texts())
	}

	shell.say("\x1b[?2026l")
	invalidator.Invalidate()
	if err := session.Driver().Pump(); err != nil {
		t.Fatal(err)
	}
	if !containsExact(session.Texts(), "after") || containsExact(session.Texts(), "before") {
		t.Fatalf("the completed update did not replace the screen: %q", session.Texts())
	}
}
