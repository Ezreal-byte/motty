package workspace

import "fmt"

// DTOVersion is the current stable persistence format.
const DTOVersion uint16 = 1

// DTO is the stable persistence representation of a Layout. Nodes are in
// canonical pre-order and use dense IDs from 1 through len(Nodes); non-empty
// DTOs always have Root == 1.
type DTO struct {
	Version uint16    `json:"version"`
	Root    NodeID    `json:"root,omitempty"`
	Nodes   []NodeDTO `json:"nodes,omitempty"`
}

// NodeDTO stores one canonical persistence node.
type NodeDTO struct {
	ID      NodeID    `json:"id"`
	Kind    Kind      `json:"kind"`
	Session SessionID `json:"session,omitempty"`
	Axis    Axis      `json:"axis,omitempty"`
	Ratio   float32   `json:"ratio,omitempty"`
	First   NodeID    `json:"first,omitempty"`
	Second  NodeID    `json:"second,omitempty"`
}

// DTO returns a canonical, deterministic persistence snapshot.
func (layout *Layout) DTO() DTO {
	result := DTO{Version: DTOVersion}
	if layout.root == 0 {
		return result
	}
	order := make([]NodeID, 0, layout.sessions*2-1)
	stack := []NodeID{layout.root}
	for len(stack) > 0 {
		last := len(stack) - 1
		id := stack[last]
		stack = stack[:last]
		order = append(order, id)
		entry := layout.mustSlot(id)
		if entry.kind == KindSplit {
			stack = append(stack, entry.second, entry.first)
		}
	}
	canonical := make([]NodeID, len(layout.slots))
	for index, id := range order {
		canonical[id.index()] = NodeID(index + 1)
	}
	result.Root = 1
	result.Nodes = make([]NodeDTO, len(order))
	for index, id := range order {
		entry := layout.mustSlot(id)
		node := NodeDTO{
			ID: NodeID(index + 1), Kind: entry.kind, Session: entry.session,
			Axis: entry.axis, Ratio: entry.ratio,
		}
		if entry.kind == KindSplit {
			node.First = canonical[entry.first.index()]
			node.Second = canonical[entry.second.index()]
		}
		result.Nodes[index] = node
	}
	return result
}

// Restore validates and reconstructs a deterministic Layout from a DTO.
func Restore(dto DTO) (Layout, error) {
	if err := dto.Validate(); err != nil {
		return Layout{}, err
	}
	if len(dto.Nodes) == 0 {
		return Layout{}, nil
	}
	layout := Layout{slots: make([]slot, len(dto.Nodes)+1)}
	ids := make([]NodeID, len(dto.Nodes)+1)
	for index, node := range dto.Nodes {
		runtimeID := makeNodeID(uint32(index+1), 1)
		ids[node.ID] = runtimeID
		layout.slots[index+1] = slot{
			generation: 1, kind: node.Kind, session: node.Session,
			axis: node.Axis, ratio: node.Ratio,
		}
		if node.Kind == KindLeaf {
			layout.sessions++
		}
	}
	for _, node := range dto.Nodes {
		if node.Kind != KindSplit {
			continue
		}
		entry := &layout.slots[node.ID]
		entry.first = ids[node.First]
		entry.second = ids[node.Second]
		layout.mustSlot(entry.first).parent = ids[node.ID]
		layout.mustSlot(entry.second).parent = ids[node.ID]
	}
	layout.root = ids[dto.Root]
	return layout, nil
}

// Validate checks DTO version, canonical shape, split values, reachability,
// cycles, shared children, and duplicate sessions.
func (dto DTO) Validate() error {
	if dto.Version != DTOVersion {
		return fmt.Errorf("%w: unsupported DTO version %d", ErrInvalidLayout, dto.Version)
	}
	if len(dto.Nodes) == 0 {
		if dto.Root != 0 {
			return fmt.Errorf("%w: empty DTO has root %d", ErrInvalidLayout, dto.Root)
		}
		return nil
	}
	if len(dto.Nodes) > nodeIndexMask || dto.Root != 1 {
		return fmt.Errorf("%w: non-canonical root or node count", ErrInvalidLayout)
	}
	parents := make([]uint8, len(dto.Nodes)+1)
	sessions := make(map[SessionID]struct{}, (len(dto.Nodes)+1)/2)
	for index, node := range dto.Nodes {
		expected := NodeID(index + 1)
		if node.ID != expected {
			return fmt.Errorf("%w: node %d has ID %d", ErrInvalidLayout, index, node.ID)
		}
		switch node.Kind {
		case KindLeaf:
			if node.Session == 0 || node.Axis != 0 || node.Ratio != 0 || node.First != 0 || node.Second != 0 {
				return fmt.Errorf("%w: malformed leaf %d", ErrInvalidLayout, node.ID)
			}
			if _, exists := sessions[node.Session]; exists {
				return fmt.Errorf("%w: duplicate session %d", ErrInvalidLayout, node.Session)
			}
			sessions[node.Session] = struct{}{}
		case KindSplit:
			if node.Session != 0 || !validAxis(node.Axis) || !validRatio(node.Ratio) || node.First == 0 || node.Second == 0 || node.First == node.Second {
				return fmt.Errorf("%w: malformed split %d", ErrInvalidLayout, node.ID)
			}
			if int(node.First) > len(dto.Nodes) || int(node.Second) > len(dto.Nodes) {
				return fmt.Errorf("%w: split %d child out of range", ErrInvalidLayout, node.ID)
			}
			parents[node.First]++
			parents[node.Second]++
			if parents[node.First] > 1 || parents[node.Second] > 1 {
				return fmt.Errorf("%w: node has multiple parents", ErrInvalidLayout)
			}
		default:
			return fmt.Errorf("%w: invalid kind at node %d", ErrInvalidLayout, node.ID)
		}
	}
	if parents[dto.Root] != 0 {
		return fmt.Errorf("%w: root has parent", ErrInvalidLayout)
	}
	seen := make([]bool, len(dto.Nodes)+1)
	stack := []NodeID{dto.Root}
	for len(stack) > 0 {
		last := len(stack) - 1
		id := stack[last]
		stack = stack[:last]
		if seen[id] {
			return fmt.Errorf("%w: cycle at node %d", ErrInvalidLayout, id)
		}
		seen[id] = true
		node := dto.Nodes[id-1]
		if node.Kind == KindSplit {
			stack = append(stack, node.Second, node.First)
		}
	}
	for id := NodeID(1); int(id) <= len(dto.Nodes); id++ {
		if !seen[id] || (id != dto.Root && parents[id] != 1) {
			return fmt.Errorf("%w: unreachable node %d", ErrInvalidLayout, id)
		}
	}
	return nil
}
