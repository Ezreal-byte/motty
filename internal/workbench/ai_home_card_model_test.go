package workbench

import (
	"path/filepath"
	"strings"
	"testing"

	"motty/internal/ai/store"
)

// The model menu is a window-level popup: beside the home card it may open
// outside the card, so the card's body clip must not cut the menu or its
// dismiss backdrop.
func TestHomeAssistantModelMenuEscapesTheCard(t *testing.T) {
	_, options := newWorld(t)
	options.AIStorePath = filepath.Join(t.TempDir(), "ai.db")
	state := newTestState(t, options)
	t.Cleanup(state.Close)
	withModel(t, state)
	db, err := state.aiStore()
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("wide-model-name-", 4)
	if err := db.SaveModels("p", []store.Model{
		{ProviderID: "p", Name: long, Enabled: true},
		{ProviderID: "p", Name: "gpt-x", Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveProvider(store.Provider{ID: "q", Name: "Home", Kind: store.KindOpenAI, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveModels("q", []store.Model{{ProviderID: "q", Name: long, Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	state.aiWrote()
	state.ai.open = true
	state.ai.panelWidth = aiPanelMin
	paintAt(state, panelWide)
	state.Click(hitAIModelChip)
	paintAt(state, panelWide)
	state.Click(hitAIModelProviders.At(0))
	canvas, _ := paintAt(state, panelWide)

	card := state.ai.panel.rect
	menu := state.ai.panel.modelsRect
	if menu.Width <= 0 || menu.X >= card.X {
		t.Fatalf("fixture did not put the model menu outside the card: menu=%+v card=%+v", menu, card)
	}
	row := panelHit(t, canvas, hitAIModels.At(0))
	if row.Bounds.X >= card.X {
		t.Fatalf("model row starts at the card edge, so the menu is clipped: menu=%+v row=%+v card=%+v", menu, row.Bounds, card)
	}
	if row.Bounds.Width < menu.Width-24 {
		t.Fatalf("model row is clipped: menu width %g, row width %g", menu.Width, row.Bounds.Width)
	}
	dismiss := panelHit(t, canvas, hitAIModelDismiss)
	if dismiss.Bounds.Width < panelWide.Width || dismiss.Bounds.Height < panelWide.Height {
		t.Fatalf("dismiss backdrop is clipped: %+v", dismiss.Bounds)
	}
}
