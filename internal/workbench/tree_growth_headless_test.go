//go:build darwin || windows || linux

package workbench

import "testing"

// The identity tree never shrinks, so painting must name its places once. With
// two panes and the files panel open (where mounted scopes are made), painting
// again must not add one.
func TestPaintingAgainDoesNotGrowTheIdentityTree(t *testing.T) {
	state, _ := openLocal(t)
	session := openWorkbenchSession(t, state)
	state.Click(hitPaneSplitHorizontal)
	state.Click(hitSFTPOpen)
	if err := session.Resize(1400, 900); err != nil {
		t.Fatal(err)
	}
	if len(state.panes) != 2 || state.deck.ActiveWorkspace() == 0 {
		t.Fatalf("setup: %d panes; the test must exercise the pane mounts", len(state.panes))
	}
	active, ok := state.sessionByIDPointer(state.activeSessionID)
	if !ok || active.files == nil {
		t.Fatal("setup: the files panel is not open; the test must exercise its mount")
	}

	grown := tree.Len()
	for range 5 {
		if err := session.Resize(1380, 900); err != nil {
			t.Fatal(err)
		}
		if err := session.Resize(1400, 900); err != nil {
			t.Fatal(err)
		}
	}
	if tree.Len() != grown {
		t.Fatalf("the identity tree grew from %d to %d scopes across repaints", grown, tree.Len())
	}
}
