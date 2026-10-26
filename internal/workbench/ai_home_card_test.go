package workbench

import (
	"fmt"
	"math"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestHomeAssistantCardsAlignAndKeepAResizeGutter(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 3} {
		for _, light := range []bool{false, true} {
			t.Run(fmt.Sprintf("scale=%g/light=%v", scale, light), func(t *testing.T) {
				_, options := newWorld(t)
				state := newTestState(t, options)
				t.Cleanup(state.Close)
				if light {
					state.Click(hitTheme)
				}
				state.ai.open = true
				var canvas ui.Canvas
				canvas.SetPixelScale(scale)
				state.Paint(&canvas, panelWide)
				var cards []ui.Rectangle
				for _, r := range canvas.Rectangles() {
					if r.Color == state.pal.windowStroke && r.Radius > 0 && r.Bounds.Height > panelWide.Height/2 {
						cards = append(cards, r)
					}
				}
				if len(cards) != 2 {
					t.Fatalf("got %d large rounded card borders, want two", len(cards))
				}
				left, right := cards[0], cards[1]
				if left.Bounds.Y != right.Bounds.Y || left.Bounds.Height != right.Bounds.Height || left.Radius != right.Radius {
					t.Fatalf("card edges do not align: left=%+v right=%+v", left, right)
				}
				gap := right.Bounds.X - left.Bounds.X - left.Bounds.Width
				outer := panelWide.Width - right.Bounds.X - right.Bounds.Width
				bottom := panelWide.Height - right.Bounds.Y - right.Bounds.Height
				tolerance := 1/float64(scale) + 1e-3 // one device pixel, allowing float32 roundoff
				if gap <= 0 || math.Abs(float64(gap-outer)) > tolerance || math.Abs(float64(gap-bottom)) > tolerance {
					t.Fatalf("uneven card spacing: gap=%g right=%g bottom=%g", gap, outer, bottom)
				}
				divider := panelHit(t, &canvas, hitAIDivider).Bounds
				center := divider.X + divider.Width/2
				if center <= left.Bounds.X+left.Bounds.Width || center >= right.Bounds.X {
					t.Fatalf("resize handle is not between the cards: %+v", divider)
				}
				for _, r := range canvas.Rectangles() {
					if r.Color.Alpha > 0 && r.Bounds.Width <= 3 && r.Bounds.Height > panelWide.Height/2 &&
						r.Bounds.X >= left.Bounds.X+left.Bounds.Width && r.Bounds.X+r.Bounds.Width <= right.Bounds.X {
						t.Fatal("an idle separator is still painted between the cards")
					}
				}
				// Pointer-down at any part of the gutter must not jump the panel's
				// width; its border and the pointer are no longer at the same x.
				before := state.ai.panel.rect.Width
				pointer := func(kind ui.PointerEventKind, x float32) {
					state.Pointer(ui.PointerEvent{Kind: kind, Target: hitAIDivider, X: x, Y: divider.Y + 50, Viewport: panelWide, Captured: true, Button: ui.PointerButtonPrimary})
				}
				pointer(ui.PointerDown, center+2)
				if state.aiPanelExtent(panelWide) != before {
					t.Fatal("pressing the gutter changed the panel width")
				}
				pointer(ui.PointerMove, center+2-80)
				pointer(ui.PointerUp, center+2-80)
				if state.aiPanelExtent(panelWide) != before+80 {
					t.Fatal("gutter drag did not track pointer displacement")
				}
				// Narrow windows keep the existing drawer, not a squeezed card.
				canvasNarrow, _ := paintAt(state, panelNarrow)
				panelHit(t, canvasNarrow, hitAIDrawerDismiss)
				if _, ok := hitByID(canvasNarrow, hitAIDivider); ok {
					t.Fatal("the drawer retained a docked resize gutter")
				}
			})
		}
	}
}
