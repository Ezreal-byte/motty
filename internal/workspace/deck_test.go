package workspace

import (
	"errors"
	"testing"
)

func TestDeckCreatesIndependentWorkspacesAndEmptyPanes(t *testing.T) {
	var deck Deck
	production, productionPane, err := deck.Create("Production")
	if err != nil {
		t.Fatal(err)
	}
	database, databasePane, err := deck.Create("Database")
	if err != nil {
		t.Fatal(err)
	}
	if production == database || productionPane == databasePane || deck.ActiveWorkspace() != database {
		t.Fatalf("workspace IDs = %d/%d panes=%d/%d active=%d", production, database, productionPane, databasePane, deck.ActiveWorkspace())
	}
	if pane, ok := deck.Pane(production, productionPane); !ok || pane.SessionID != 0 {
		t.Fatalf("new pane = %+v ok=%v", pane, ok)
	}
	if !deck.ActivateWorkspace(production) || deck.ActiveWorkspace() != production || deck.ActivePane(production) != productionPane {
		t.Fatal("production workspace did not retain its active pane")
	}
	if err := deck.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDeckSplitRightAndDownBuildNestedTree(t *testing.T) {
	var deck Deck
	workspaceID, first, _ := deck.Create("Troubleshooting")
	second, err := deck.Split(workspaceID, first, Horizontal)
	if err != nil {
		t.Fatal(err)
	}
	third, err := deck.Split(workspaceID, second, Vertical)
	if err != nil {
		t.Fatal(err)
	}
	if deck.ActivePane(workspaceID) != third {
		t.Fatalf("active pane = %d, want %d", deck.ActivePane(workspaceID), third)
	}
	layout, _ := deck.Layout(workspaceID)
	root, _ := layout.Node(layout.Root())
	if root.Kind != KindSplit || root.Axis != Horizontal || root.Ratio != 0.5 {
		t.Fatalf("root = %+v", root)
	}
	secondBranch, _ := layout.Node(root.Second)
	if secondBranch.Kind != KindSplit || secondBranch.Axis != Vertical || secondBranch.Ratio != 0.5 {
		t.Fatalf("second branch = %+v", secondBranch)
	}
	panes := deck.Panes(workspaceID, nil)
	if len(panes) != 3 || panes[0].ID != first || panes[1].ID != second || panes[2].ID != third {
		t.Fatalf("visual panes = %+v", panes)
	}
	if err := deck.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDeckBindsEachSessionOnceAndTargetsActivePane(t *testing.T) {
	var deck Deck
	firstWorkspace, firstPane, _ := deck.Create("One")
	secondWorkspace, secondPane, _ := deck.Create("Two")
	if err := deck.Bind(firstWorkspace, firstPane, 10); err != nil {
		t.Fatal(err)
	}
	if err := deck.Bind(secondWorkspace, secondPane, 10); !errors.Is(err, ErrSessionExists) {
		t.Fatalf("duplicate binding = %v", err)
	}
	if err := deck.Bind(firstWorkspace, firstPane, 11); !errors.Is(err, ErrPaneFull) {
		t.Fatalf("full pane binding = %v", err)
	}
	if session, err := deck.Unbind(firstWorkspace, firstPane); err != nil || session != 10 {
		t.Fatalf("unbind = %d, %v", session, err)
	}
	if err := deck.Bind(secondWorkspace, secondPane, 10); err != nil {
		t.Fatal(err)
	}
	if !deck.ActivatePane(secondWorkspace, secondPane) || deck.ActiveWorkspace() != secondWorkspace {
		t.Fatal("pane activation did not activate its workspace")
	}
	workspaceID, paneID, ok := deck.PaneForSession(10)
	if !ok || workspaceID != secondWorkspace || paneID != secondPane {
		t.Fatalf("session location = %d/%d/%v", workspaceID, paneID, ok)
	}
}

func TestDeckMovesPaneWithoutChangingBinding(t *testing.T) {
	var deck Deck
	workspaceID, first, _ := deck.Create("Move")
	second, _ := deck.Split(workspaceID, first, Horizontal)
	third, _ := deck.Split(workspaceID, second, Vertical)
	if err := deck.Bind(workspaceID, third, 30); err != nil {
		t.Fatal(err)
	}
	if err := deck.Move(workspaceID, third, first, Horizontal, 0.25, Before); err != nil {
		t.Fatal(err)
	}
	panes := deck.Panes(workspaceID, nil)
	if len(panes) != 3 || panes[0].ID != third || panes[0].SessionID != 30 || deck.ActivePane(workspaceID) != third {
		t.Fatalf("moved panes = %+v active=%d", panes, deck.ActivePane(workspaceID))
	}
}

func TestDeckClosePaneCollapsesTreeAndChoosesNeighbour(t *testing.T) {
	var deck Deck
	workspaceID, first, _ := deck.Create("Close")
	second, _ := deck.Split(workspaceID, first, Horizontal)
	third, _ := deck.Split(workspaceID, second, Vertical)
	if err := deck.Bind(workspaceID, second, 20); err != nil {
		t.Fatal(err)
	}
	detached, err := deck.ClosePane(workspaceID, second)
	if err != nil || detached != 20 {
		t.Fatalf("close = session %d err %v", detached, err)
	}
	if deck.ActivePane(workspaceID) != third {
		t.Fatalf("active after middle close = %d, want %d", deck.ActivePane(workspaceID), third)
	}
	layout, _ := deck.Layout(workspaceID)
	root, _ := layout.Node(layout.Root())
	if layout.Len() != 2 || root.Axis != Horizontal {
		t.Fatalf("collapsed layout = len %d root %+v", layout.Len(), root)
	}
	if _, err := deck.ClosePane(workspaceID, third); err != nil {
		t.Fatal(err)
	}
	if deck.ActivePane(workspaceID) != first {
		t.Fatalf("active after last visual close = %d, want %d", deck.ActivePane(workspaceID), first)
	}
	if _, err := deck.ClosePane(workspaceID, first); err != nil {
		t.Fatal(err)
	}
	if deck.ActivePane(workspaceID) != 0 || len(deck.Panes(workspaceID, nil)) != 0 {
		t.Fatal("closing final pane did not leave an empty workspace")
	}
	replacement, err := deck.EnsurePane(workspaceID)
	if err != nil || replacement == 0 {
		t.Fatalf("ensure pane = %d, %v", replacement, err)
	}
	if err := deck.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDeckRatioAndPaneLimits(t *testing.T) {
	var deck Deck
	workspaceID, first, _ := deck.Create("Limits")
	second, _ := deck.Split(workspaceID, first, Horizontal)
	layout, _ := deck.Layout(workspaceID)
	root := layout.Root()
	if err := deck.SetRatio(workspaceID, root, 0.01); err != nil {
		t.Fatal(err)
	}
	node, _ := layout.Node(root)
	if node.Ratio != MinPaneRatio {
		t.Fatalf("minimum ratio = %f", node.Ratio)
	}
	if err := deck.SetRatio(workspaceID, root, 0.99); err != nil {
		t.Fatal(err)
	}
	node, _ = layout.Node(root)
	if node.Ratio != MaxPaneRatio {
		t.Fatalf("maximum ratio = %f", node.Ratio)
	}
	target := second
	for count := 2; count < MaxPanes; count++ {
		target, _ = deck.Split(workspaceID, target, Vertical)
	}
	if _, err := deck.Split(workspaceID, target, Vertical); !errors.Is(err, ErrPaneLimit) {
		t.Fatalf("ninth pane = %v", err)
	}
}

func TestDeckRemoveWorkspaceSelectsDeterministicNeighbour(t *testing.T) {
	var deck Deck
	first, _, _ := deck.Create("One")
	second, secondPane, _ := deck.Create("Two")
	third, _, _ := deck.Create("Three")
	if err := deck.Bind(second, secondPane, 22); err != nil {
		t.Fatal(err)
	}
	deck.ActivateWorkspace(second)
	sessions, err := deck.RemoveWorkspace(second, nil)
	if err != nil || len(sessions) != 1 || sessions[0] != 22 || deck.ActiveWorkspace() != third {
		t.Fatalf("remove middle = sessions %v active %d err %v", sessions, deck.ActiveWorkspace(), err)
	}
	if _, err := deck.RemoveWorkspace(third, nil); err != nil || deck.ActiveWorkspace() != first {
		t.Fatalf("remove last = active %d err %v", deck.ActiveWorkspace(), err)
	}
}
