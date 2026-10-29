package workbench

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/ui"
)

// historyState is a five-row terminal with sixty numbered lines behind it.
func historyState() (*State, *Session, terminalGeometry) {
	state, session, geometry := selectionState()
	var text strings.Builder
	for line := 0; line < 65; line++ {
		fmt.Fprintf(&text, "line %d\r\n", line)
	}
	session.term.write([]byte(text.String()))
	return state, session, geometry
}

func barPointer(state *State, session *Session, kind ui.PointerEventKind, x, y float32) ui.EventResult {
	return state.Pointer(ui.PointerEvent{
		Kind: kind, Button: ui.PointerButtonPrimary, Target: hitTerminalBar.At(int(session.ID)),
		Captured: kind != ui.PointerDown, X: x, Y: y, Viewport: state.viewport,
	})
}

func TestTerminalWithHistoryShowsAScrollbarThatMovesTheView(t *testing.T) {
	state, session, geometry := historyState()
	screen := session.term.screen
	history := screen.Scrollback()
	if history < 50 {
		t.Fatalf("fixture has only %d lines of history", history)
	}

	var canvas ui.Canvas
	state.paintGrid(&canvas, session, geometry)
	bar, ok := hitByID(&canvas, hitTerminalBar.At(int(session.ID)))
	if !ok {
		t.Fatal("a terminal with history paints no scrollbar")
	}
	x := bar.Bounds.X + bar.Bounds.Width/2

	// At the live screen the thumb is at the bottom: dragging it up goes back.
	area := state.terminalScrollArea(nil, session, geometry.area).Geometry()
	grab := area.VerticalThumb.Y + area.VerticalThumb.Height/2
	barPointer(state, session, ui.PointerDown, x, grab)
	if result := barPointer(state, session, ui.PointerMove, x, bar.Bounds.Y); !result.Handled || !result.Repaint {
		t.Fatalf("dragging the thumb up = %+v", result)
	}
	if screen.ViewOffset() != history {
		t.Fatalf("thumb at the top shows %d lines back, want the oldest (%d)", screen.ViewOffset(), history)
	}
	barPointer(state, session, ui.PointerUp, x, bar.Bounds.Y)

	// A press on the track below the thumb pages toward the live screen.
	before := screen.ViewOffset()
	barPointer(state, session, ui.PointerDown, x, bar.Bounds.Y+bar.Bounds.Height-2)
	if want := before - (geometry.rows - 1); screen.ViewOffset() != want {
		t.Fatalf("a press below the thumb moved to %d, want %d", screen.ViewOffset(), want)
	}
}

func TestTerminalScrollbarFollowsTheWheelAndNeedsHistory(t *testing.T) {
	state, session, geometry := historyState()
	screen := session.term.screen
	screen.Scroll(10)
	before := state.terminalScrollArea(nil, session, geometry.area).Geometry().VerticalThumb.Y

	screen.Scroll(10)
	after := state.terminalScrollArea(nil, session, geometry.area).Geometry().VerticalThumb.Y
	if after >= before {
		t.Fatalf("scrolling back left the thumb at %v, was %v", after, before)
	}

	var canvas ui.Canvas
	state.paintGrid(&canvas, session, geometry)
	if _, ok := hitByID(&canvas, hitTerminalBar.At(int(session.ID))); !ok {
		t.Fatal("no scrollbar over a terminal with history")
	}

	// The alternate screen has no history, and a terminal that has not scrolled
	// anything off has nothing to show.
	session.term.write([]byte("\x1b[?1049h"))
	canvas = ui.Canvas{}
	state.paintGrid(&canvas, session, geometry)
	if _, ok := hitByID(&canvas, hitTerminalBar.At(int(session.ID))); ok {
		t.Fatal("a scrollbar on the alternate screen")
	}
	fresh, freshSession, freshGeometry := selectionState()
	canvas = ui.Canvas{}
	fresh.paintGrid(&canvas, freshSession, freshGeometry)
	if _, ok := hitByID(&canvas, hitTerminalBar.At(int(freshSession.ID))); ok {
		t.Fatal("a scrollbar with nothing to scroll")
	}
}
