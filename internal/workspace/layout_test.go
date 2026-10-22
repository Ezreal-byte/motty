package workspace

import (
	"errors"
	"slices"
	"testing"
)

func TestLayoutOperations(t *testing.T) {
	layout := mustNew(t, 1)
	if err := layout.Split(1, 2, Horizontal, 0.6, After); err != nil {
		t.Fatal(err)
	}
	root := layout.Root()
	if err := layout.Split(2, 3, Vertical, 0.4, Before); err != nil {
		t.Fatal(err)
	}
	assertSessions(t, &layout, 1, 3, 2)
	if err := layout.SetRatio(root, 0.7); err != nil {
		t.Fatal(err)
	}
	if node, ok := layout.Node(root); !ok || node.Ratio != 0.7 {
		t.Fatalf("root = %+v, ok=%v", node, ok)
	}
	if err := layout.Remove(3); err != nil {
		t.Fatal(err)
	}
	assertSessions(t, &layout, 1, 2)
	leafBefore := layout.findSession(2)
	if err := layout.Move(2, 1, Vertical, 0.3, Before); err != nil {
		t.Fatal(err)
	}
	if leafAfter := layout.findSession(2); leafAfter != leafBefore {
		t.Fatalf("move changed leaf ID: %d -> %d", leafBefore, leafAfter)
	}
	assertSessions(t, &layout, 2, 1)
	if err := layout.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := layout.Remove(2); err != nil {
		t.Fatal(err)
	}
	if err := layout.Remove(1); err != nil {
		t.Fatal(err)
	}
	if layout.Root() != 0 || layout.Len() != 0 || len(layout.slots) != 0 {
		t.Fatalf("empty layout = %+v", layout)
	}
	if err := layout.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestLayoutErrorsDoNotMutate(t *testing.T) {
	tests := []struct {
		name string
		run  func(*Layout) error
		want error
	}{
		{"zero session", func(layout *Layout) error { return layout.Split(1, 0, Horizontal, 0.5, After) }, ErrInvalidSession},
		{"duplicate session", func(layout *Layout) error { return layout.Split(1, 1, Horizontal, 0.5, After) }, ErrSessionExists},
		{"missing target", func(layout *Layout) error { return layout.Split(9, 2, Horizontal, 0.5, After) }, ErrTargetNotFound},
		{"bad axis", func(layout *Layout) error { return layout.Split(1, 2, 9, 0.5, After) }, ErrInvalidAxis},
		{"bad ratio zero", func(layout *Layout) error { return layout.Split(1, 2, Horizontal, 0, After) }, ErrInvalidRatio},
		{"bad ratio one", func(layout *Layout) error { return layout.Split(1, 2, Horizontal, 1, After) }, ErrInvalidRatio},
		{"bad placement", func(layout *Layout) error { return layout.Split(1, 2, Horizontal, 0.5, 9) }, ErrInvalidPlacement},
		{"remove missing", func(layout *Layout) error { return layout.Remove(9) }, ErrSessionNotFound},
		{"move same", func(layout *Layout) error { return layout.Move(1, 1, Horizontal, 0.5, After) }, ErrInvalidSession},
		{"move missing", func(layout *Layout) error { return layout.Move(9, 1, Horizontal, 0.5, After) }, ErrSessionNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout := mustNew(t, 1)
			before := layout.DTO()
			err := test.run(&layout)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			assertDTOEqual(t, layout.DTO(), before)
		})
	}
}

func TestRemoveCollapsesAndInvalidatesOldSplitID(t *testing.T) {
	layout := mustNew(t, 1)
	if err := layout.Split(1, 2, Horizontal, 0.5, After); err != nil {
		t.Fatal(err)
	}
	oldSplit := layout.Root()
	if err := layout.Remove(2); err != nil {
		t.Fatal(err)
	}
	if err := layout.SetRatio(oldSplit, 0.4); !errors.Is(err, ErrInvalidNode) {
		t.Fatalf("SetRatio(stale) = %v", err)
	}
	if err := layout.Split(1, 3, Vertical, 0.5, After); err != nil {
		t.Fatal(err)
	}
	if layout.Root() == oldSplit {
		t.Fatal("reused slot revived stale split ID")
	}
}

func TestDeepAndWideLayouts(t *testing.T) {
	t.Run("deep", func(t *testing.T) {
		layout := mustNew(t, 1)
		for session := SessionID(2); session <= 1024; session++ {
			if err := layout.Split(session-1, session, Vertical, 0.5, After); err != nil {
				t.Fatal(err)
			}
		}
		assertValidRoundTrip(t, &layout)
	})
	t.Run("wide", func(t *testing.T) {
		layout := mustNew(t, 1)
		queue := []SessionID{1}
		next := SessionID(2)
		for len(queue) > 0 && next <= 1024 {
			target := queue[0]
			queue = queue[1:]
			if err := layout.Split(target, next, Horizontal, 0.5, After); err != nil {
				t.Fatal(err)
			}
			queue = append(queue, target, next)
			next++
		}
		assertValidRoundTrip(t, &layout)
	})
}

func mustNew(t *testing.T, session SessionID) Layout {
	t.Helper()
	layout, err := New(session)
	if err != nil {
		t.Fatal(err)
	}
	return layout
}

func assertSessions(t *testing.T, layout *Layout, want ...SessionID) {
	t.Helper()
	got := layout.Sessions(make([]SessionID, 0, layout.Len()))
	if !slices.Equal(got, want) {
		t.Fatalf("sessions = %v, want %v", got, want)
	}
}

func assertValidRoundTrip(t *testing.T, layout *Layout) {
	t.Helper()
	if err := layout.Validate(); err != nil {
		t.Fatal(err)
	}
	dto := layout.DTO()
	if err := dto.Validate(); err != nil {
		t.Fatal(err)
	}
	restored, err := Restore(dto)
	if err != nil {
		t.Fatal(err)
	}
	if err := restored.Validate(); err != nil {
		t.Fatal(err)
	}
	assertDTOEqual(t, restored.DTO(), dto)
}

func assertDTOEqual(t *testing.T, got, want DTO) {
	t.Helper()
	if got.Version != want.Version || got.Root != want.Root || !slices.Equal(got.Nodes, want.Nodes) {
		t.Fatalf("DTO = %+v, want %+v", got, want)
	}
}
