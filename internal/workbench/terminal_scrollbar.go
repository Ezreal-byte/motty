package workbench

import (
	"math"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// hitTerminalBar is the scrollbar a terminal shows while it has scrollback.
var hitTerminalBar = tree.Sub(shellHits, tree.Name("terminal-bar"))

// scrollbarSession finds the session whose terminal owns a scrollbar hit.
func (state *State) scrollbarSession(hit ui.ID) *Session {
	if hit.Scope() != hitTerminalBar {
		return nil
	}
	for index := range state.sessions {
		if state.sessions[index].ID == uint32(item(hit)) {
			return &state.sessions[index]
		}
	}
	return nil
}

// terminalOrBarSession finds the session whose terminal, or whose terminal's
// scrollbar, owns a hit: the places a pointer or wheel over the terminal lands.
func (state *State) terminalOrBarSession(hit ui.ID) *Session {
	if session := state.terminalSession(hit); session != nil {
		return session
	}
	return state.scrollbarSession(hit)
}

// terminalScrollbarShown reports whether the terminal has history to scroll
// through. The alternate screen has none, and a synchronized update is a frozen
// picture of the screen, not a place to move in.
func (t *terminal) terminalScrollbarShown() bool {
	return t.sync == nil && !t.presentedAltScreen() && t.screen.Scrollback() > 0
}

// terminalScrollArea is the terminal's text as a scroll area: the history above
// the screen and the screen itself, one cell height per line. It paints only the
// bar, over the right edge of the grid, in the terminal's own colors since the
// application theme has no say in what the terminal's ground looks like. A nil
// canvas gives the geometry alone.
func (state *State) terminalScrollArea(canvas *ui.Canvas, session *Session, area ui.Rect) components.ScrollArea {
	screen := session.term.screen
	history := float32(screen.Scrollback()) * state.cellHeight
	theme := state.Theme()
	theme.Border = withAlpha(state.termPal.termFG, 0.22)
	theme.MutedForeground = withAlpha(state.termPal.termFG, 0.55)
	id := hitTerminalBar.At(int(session.ID))
	return components.BeginScrollArea(canvas, theme, area, ui.Size{Width: area.Width, Height: area.Height + history}).
		Offset(0, float32(screen.Scrollback()-screen.ViewOffset())*state.cellHeight).
		VerticalScrollbar(id, state.messages.Tools.ScrollbarLabel, "")
}

func (state *State) paintTerminalScrollbar(canvas *ui.Canvas, session *Session, geometry terminalGeometry) {
	if !session.term.terminalScrollbarShown() || state.cellHeight <= 0 {
		return
	}
	area := state.terminalScrollArea(canvas, session, geometry.area)
	area.EndContent()
}

// terminalScrollbarPointer drags the thumb and pages when the track beside it is
// pressed.
func (state *State) terminalScrollbarPointer(event ui.PointerEvent) (ui.EventResult, bool) {
	session := state.scrollbarSession(event.Target)
	if session == nil || session.term == nil || state.cellHeight <= 0 {
		return ui.EventResult{}, false
	}
	t := session.term
	if state.modal != modalNone || state.activeSection != sectionTerminal || !t.terminalScrollbarShown() {
		t.barDragging = false
		return ui.EventResult{}, false
	}
	layout := terminalLayoutWithInfoBar(event.Viewport, state.cellWidth, state.cellHeight, state.terminalInfoBarHeight(session.ID), state.padding())
	geometry := state.terminalScrollArea(nil, session, layout.area).Geometry()
	switch event.Kind {
	case ui.PointerDown:
		if event.Button != ui.PointerButtonPrimary && event.Button != ui.PointerButtonNone {
			return ui.EventResult{}, false
		}
		thumb := geometry.VerticalThumb
		switch {
		case event.Y < thumb.Y:
			return state.scrollTerminalView(t, layout.rows-1), true
		case event.Y >= thumb.Y+thumb.Height:
			return state.scrollTerminalView(t, -(layout.rows - 1)), true
		}
		t.barDragging = true
		t.barGrab = event.Y - thumb.Y
		return ui.EventResult{Handled: true, Repaint: true}, true
	case ui.PointerMove, ui.PointerUp:
		if !t.barDragging {
			return ui.EventResult{}, false
		}
		offset := geometry.VerticalOffsetForThumb(event.Y - t.barGrab)
		line := int(math.Round(float64(offset / state.cellHeight)))
		result := state.scrollTerminalView(t, t.screen.Scrollback()-line-t.screen.ViewOffset())
		if event.Kind == ui.PointerUp {
			t.barDragging = false
		}
		return result, true
	case ui.PointerExit:
		if !t.barDragging {
			return ui.EventResult{}, false
		}
		t.barDragging = false
		return ui.EventResult{Handled: true}, true
	}
	return ui.EventResult{}, false
}

// scrollTerminalView moves the view by lines, positive going back into history.
func (state *State) scrollTerminalView(t *terminal, lines int) ui.EventResult {
	if lines == 0 {
		return ui.EventResult{Handled: true}
	}
	t.selection.clear()
	return ui.EventResult{Handled: true, Repaint: t.screen.Scroll(lines)}
}
