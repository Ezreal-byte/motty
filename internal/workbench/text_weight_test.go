package workbench

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A heavier face is wider: on Windows semibold Segoe UI is wide enough that a
// button sized from the regular-weight measurement clipped "New Host" to
// "New Hos". Widths must come from the weight the label is drawn at.
func TestToolbarButtonsAreSizedForTheirSemiboldLabel(t *testing.T) {
	var canvas ui.Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * (0.5 + weight)
	})
	const label = "Import SSH Config"
	semibold := canvas.TextWidthWeight(label, 12, ui.WeightSemibold)
	if semibold <= canvas.TextWidth(label, 12) {
		t.Fatal("test measurer does not grow with weight")
	}
	if width := toolbarButtonWidth(&canvas, label, false); width < toolbarPadding*2+semibold {
		t.Fatalf("button is %.1f wide, less than padding plus its semibold label (%.1f)", width, toolbarPadding*2+semibold)
	}
}

func TestElidedTextFitsAtTheWeightItIsDrawnWith(t *testing.T) {
	var canvas ui.Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * size * (0.5 + weight)
	})
	const width = 100
	regular := elide(&canvas, "a long workspace title", 12, ui.WeightRegular, width)
	heavy := elide(&canvas, "a long workspace title", 12, ui.WeightSemibold, width)
	if canvas.TextWidthWeight(regular, 12, ui.WeightRegular) > width || canvas.TextWidthWeight(heavy, 12, ui.WeightSemibold) > width {
		t.Fatalf("elided text does not fit: %q, %q", regular, heavy)
	}
	if len([]rune(heavy)) >= len([]rune(regular)) {
		t.Fatalf("the heavier text kept as many letters (%q) as the regular one (%q)", heavy, regular)
	}
}
