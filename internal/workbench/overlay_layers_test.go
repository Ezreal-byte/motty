package workbench

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
	"motty/internal/sshconn"
	"motty/internal/update"
)

// A narrow window docks the assistant as a drawer, which is painted before the
// modal pass. The drawer and a dialog are separate overlays, so they must not
// share a layer: one layer paints all its shapes and then all its text, and a
// shared layer would float the drawer's answer over the dialog's surface.
func TestTheAssistantDrawerKeepsItsOwnLayerUnderADialog(t *testing.T) {
	world, options := newWorld(t)
	options.Invalidate = func() {}
	options.CheckUpdate = func() (update.Release, error) {
		return update.Release{Version: "99.0.0", URL: "https://example.com/release"}, nil
	}
	state := newTestState(t, options)
	fake := connect(t, state, world)
	fake.set(func(s *sshconn.Status) {
		s.Phase = sshconn.PhaseConnected
		s.ServerVersion = "SSH-2.0-Test"
		s.Method = "password"
	})
	state.Wake()
	shell := fake.shell(t)
	shell.say("\x1b[0m")
	state.Wake()

	state.ai.open = true
	state.ai.items = []aiItem{{kind: aiAssistant, text: "## SOCKS\n\n- 在 ~/.ssh/config 里加 ServerAliveInterval 60"}}
	openGeneralSettings(state)
	state.Click(hitSettingsUpdateCheck)
	waitUpdate(t, state, func() bool { return state.modal == modalUpdate })
	state.activeSection = sectionTerminal

	// 726 is narrower than aiDockAt, so the assistant is the drawer.
	var canvas ui.Canvas
	state.Paint(&canvas, ui.Size{Width: 726, Height: 546})

	assistantLayer, dialogLayer := -1, -1
	for _, text := range canvas.Texts() {
		switch {
		case strings.Contains(text.Value, "SOCKS"):
			assistantLayer = int(text.Layer)
		case strings.Contains(text.Value, "is available"):
			dialogLayer = int(text.Layer)
		}
	}
	if assistantLayer < 0 || dialogLayer < 0 {
		t.Fatalf("the frame lacks the drawer or the dialog: assistant %d, dialog %d", assistantLayer, dialogLayer)
	}
	if assistantLayer >= dialogLayer {
		t.Fatalf("the assistant's text is on layer %d and the dialog on %d: one layer paints its text after "+
			"its shapes, so the answer would show through the dialog", assistantLayer, dialogLayer)
	}
}
