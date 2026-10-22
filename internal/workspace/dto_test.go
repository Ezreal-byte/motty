package workspace

import (
	"errors"
	"math"
	"testing"
)

func TestDTOValidation(t *testing.T) {
	valid := DTO{
		Version: DTOVersion,
		Root:    1,
		Nodes: []NodeDTO{
			{ID: 1, Kind: KindSplit, Axis: Horizontal, Ratio: 0.5, First: 2, Second: 3},
			{ID: 2, Kind: KindLeaf, Session: 10},
			{ID: 3, Kind: KindLeaf, Session: 20},
		},
	}
	tests := []struct {
		name string
		dto  DTO
	}{
		{"version", DTO{Version: 9}},
		{"empty root", DTO{Version: DTOVersion, Root: 1}},
		{"noncanonical root", DTO{Version: DTOVersion, Root: 2, Nodes: valid.Nodes}},
		{"noncanonical ID", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 2, Kind: KindLeaf, Session: 1}}}},
		{"zero session", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindLeaf}}}},
		{"leaf fields", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindLeaf, Session: 1, Ratio: 0.5}}}},
		{"bad axis", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: 9, Ratio: 0.5, First: 2, Second: 3}, valid.Nodes[1], valid.Nodes[2]}}},
		{"zero ratio", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: Horizontal, First: 2, Second: 3}, valid.Nodes[1], valid.Nodes[2]}}},
		{"negative ratio", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: Horizontal, Ratio: -0.5, First: 2, Second: 3}, valid.Nodes[1], valid.Nodes[2]}}},
		{"nan ratio", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: Horizontal, Ratio: float32(math.NaN()), First: 2, Second: 3}, valid.Nodes[1], valid.Nodes[2]}}},
		{"infinite ratio", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: Horizontal, Ratio: float32(math.Inf(1)), First: 2, Second: 3}, valid.Nodes[1], valid.Nodes[2]}}},
		{"child range", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: Horizontal, Ratio: 0.5, First: 2, Second: 4}, valid.Nodes[1], valid.Nodes[2]}}},
		{"same child", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: Horizontal, Ratio: 0.5, First: 2, Second: 2}, valid.Nodes[1]}}},
		{"duplicate session", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{valid.Nodes[0], valid.Nodes[1], {ID: 3, Kind: KindLeaf, Session: 10}}}},
		{"cycle", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: Horizontal, Ratio: 0.5, First: 2, Second: 3}, {ID: 2, Kind: KindSplit, Axis: Vertical, Ratio: 0.5, First: 1, Second: 4}, {ID: 3, Kind: KindLeaf, Session: 3}, {ID: 4, Kind: KindLeaf, Session: 4}}}},
		{"shared child", DTO{Version: DTOVersion, Root: 1, Nodes: []NodeDTO{{ID: 1, Kind: KindSplit, Axis: Horizontal, Ratio: 0.5, First: 2, Second: 3}, {ID: 2, Kind: KindSplit, Axis: Vertical, Ratio: 0.5, First: 4, Second: 5}, {ID: 3, Kind: KindSplit, Axis: Vertical, Ratio: 0.5, First: 4, Second: 6}, {ID: 4, Kind: KindLeaf, Session: 4}, {ID: 5, Kind: KindLeaf, Session: 5}, {ID: 6, Kind: KindLeaf, Session: 6}}}},
		{"unreachable", DTO{Version: DTOVersion, Root: 1, Nodes: append(append([]NodeDTO(nil), valid.Nodes...), NodeDTO{ID: 4, Kind: KindLeaf, Session: 30})}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid DTO: %v", err)
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.dto.Validate(); !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("Validate() = %v", err)
			}
			if _, err := Restore(test.dto); !errors.Is(err, ErrInvalidLayout) {
				t.Fatalf("Restore() = %v", err)
			}
		})
	}
}

func TestCanonicalRoundTripIgnoresSlotHistory(t *testing.T) {
	first := mustNew(t, 10)
	if err := first.Split(10, 20, Horizontal, 0.6, After); err != nil {
		t.Fatal(err)
	}
	if err := first.Split(20, 30, Vertical, 0.4, Before); err != nil {
		t.Fatal(err)
	}

	second := mustNew(t, 99)
	if err := second.Split(99, 10, Vertical, 0.5, After); err != nil {
		t.Fatal(err)
	}
	if err := second.Remove(99); err != nil {
		t.Fatal(err)
	}
	if err := second.Split(10, 20, Horizontal, 0.6, After); err != nil {
		t.Fatal(err)
	}
	if err := second.Split(20, 30, Vertical, 0.4, Before); err != nil {
		t.Fatal(err)
	}

	assertDTOEqual(t, first.DTO(), second.DTO())
	assertValidRoundTrip(t, &second)
}
