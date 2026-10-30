package workbench

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func emptyStateActionHit(canvas *ui.Canvas, id ui.ID) (ui.Hit, bool) {
	for _, hit := range canvas.Hits() {
		if hit.ID == id &&
			hit.Bounds.Width == emptyStateActionWidth &&
			hit.Bounds.Height == toolbarHeight {
			return hit, true
		}
	}
	return ui.Hit{}, false
}

func TestCatalogEmptyStatesUseSharedVisualMetrics(t *testing.T) {
	tests := []struct {
		name        string
		section     uint8
		title       string
		action      ui.ID
		actionLabel string
		prepare     func(*State)
		hasAction   bool
	}{
		{
			name: "hosts", section: sectionHosts, title: "Set up your hosts", action: hitAddHostEmpty, actionLabel: "New Host", hasAction: true,
			prepare: func(state *State) { state.hosts, state.visible = nil, nil },
		},
		{
			name: "monitor", section: sectionDashboard, title: "Your hosts, at a glance", action: hitDashboardNew, actionLabel: "Create panel", hasAction: true,
			prepare: func(state *State) { state.dashboard.panels, state.dashboard.active = nil, "" },
		},
		{
			name: "keys", section: sectionKeys, title: "No additional identities", action: hitKeysImport, actionLabel: "Import Key", hasAction: true,
			prepare: func(state *State) { state.identities, state.selectedIdentityID = nil, 0 },
		},
		{
			name: "proxies", section: sectionProxies, title: "No jump routes", action: hitProxyAdd, actionLabel: "Add Route", hasAction: true,
			prepare: func(state *State) { state.proxies, state.selectedProxyID = nil, 0 },
		},
		{
			name: "forwards", section: sectionForwards, title: "No port forwards", action: hitForwardAdd, actionLabel: "New Forward", hasAction: true,
			prepare: func(state *State) { state.forwards, state.selectedForwardID = nil, 0 },
		},
		{
			name: "known hosts", section: sectionKnownHosts, title: "No trusted host keys",
			prepare: func(state *State) {
				state.knownHosts.rows, state.knownHosts.visible, state.knownHosts.errorText = nil, nil, ""
			},
		},
		{
			name: "snippets", section: sectionSnippets, title: "No snippets", action: hitSnippetAdd, actionLabel: "Add snippet", hasAction: true,
			prepare: func(state *State) { state.snippets.list, state.snippets.selected = nil, 0 },
		},
	}

	for _, size := range []ui.Size{{Width: 1400, Height: 900}, {Width: 760, Height: 620}} {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				state := newTestState(t, testOptions(t))
				test.prepare(state)
				state.activeSection = test.section
				var canvas ui.Canvas
				state.Paint(&canvas, size)

				texts := make([]string, 0, len(canvas.Texts()))
				for _, text := range canvas.Texts() {
					texts = append(texts, text.Value)
				}
				if !contains(texts, test.title) {
					t.Fatalf("empty title %q was not painted", test.title)
				}

				tileFound := false
				for _, rectangle := range canvas.Rectangles() {
					if rectangle.Bounds.Width == emptyStateTileSize &&
						rectangle.Bounds.Height == emptyStateTileSize &&
						rectangle.Radius == emptyStateTileRadius &&
						rectangle.Color == state.pal.surface {
						tileFound = true
						break
					}
				}
				if !tileFound {
					t.Fatal("the shared empty-state icon tile was not painted")
				}

				if !test.hasAction {
					for _, hit := range canvas.Hits() {
						if hit.Bounds.Width == emptyStateActionWidth && hit.Bounds.Height == toolbarHeight {
							t.Fatalf("a catalog page without a create workflow painted an empty-state action: %+v", hit)
						}
					}
					return
				}
				if width := toolbarButtonWidth(&canvas, test.actionLabel, true); width > emptyStateActionWidth {
					t.Fatalf("action %q needs %.1f, more than the shared %.0f width", test.actionLabel, width, emptyStateActionWidth)
				}
				button, ok := emptyStateActionHit(&canvas, test.action)
				if !ok {
					t.Fatalf("the shared %.0fx%.0f empty-state action was not painted", emptyStateActionWidth, toolbarHeight)
				}
				if button.Bounds.X < 0 || button.Bounds.Y < 0 ||
					button.Bounds.X+button.Bounds.Width > size.Width ||
					button.Bounds.Y+button.Bounds.Height > size.Height {
					t.Fatalf("empty-state action is outside the %v window: %+v", size, button.Bounds)
				}
			})
		}
	}
}
