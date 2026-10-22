package workspace

import (
	"errors"
	"fmt"
	"math"
)

// SessionID identifies a terminal session. Zero is reserved as invalid.
type SessionID uint32

// NodeID identifies a live layout node. IDs become invalid when their node is
// removed; generation bits prevent a reused slot from reviving an old ID.
type NodeID uint32

// Kind identifies a node's payload.
type Kind uint8

const (
	KindLeaf  Kind = 1
	KindSplit Kind = 2
)

// Axis is the direction in which a split places its children.
type Axis uint8

const (
	// Horizontal places the first child left of the second child.
	Horizontal Axis = 1
	// Vertical places the first child above the second child.
	Vertical Axis = 2
)

// Placement chooses where a new or moved session appears relative to a target.
type Placement uint8

const (
	Before Placement = 1
	After  Placement = 2
)

var (
	ErrInvalidSession   = errors.New("workspace: invalid session")
	ErrSessionExists    = errors.New("workspace: session already exists")
	ErrSessionNotFound  = errors.New("workspace: session not found")
	ErrTargetNotFound   = errors.New("workspace: target session not found")
	ErrInvalidAxis      = errors.New("workspace: invalid split axis")
	ErrInvalidRatio     = errors.New("workspace: invalid split ratio")
	ErrInvalidPlacement = errors.New("workspace: invalid placement")
	ErrInvalidNode      = errors.New("workspace: invalid node")
	ErrNotSplit         = errors.New("workspace: node is not a split")
	ErrLayoutFull       = errors.New("workspace: layout node limit reached")
	ErrInvalidLayout    = errors.New("workspace: invalid layout")
)

const (
	nodeIndexBits = 16
	nodeIndexMask = 1<<nodeIndexBits - 1
	maxGeneration = 1<<(32-nodeIndexBits) - 1
)

// Node is an immutable view of one live layout node. Ratio is meaningful for a
// split and is the fraction assigned to First.
type Node struct {
	ID      NodeID
	Parent  NodeID
	Kind    Kind
	Session SessionID
	Axis    Axis
	Ratio   float32
	First   NodeID
	Second  NodeID
}

type slot struct {
	parent     NodeID
	first      NodeID
	second     NodeID
	session    SessionID
	ratio      float32
	nextFree   uint32
	generation uint16
	kind       Kind
	axis       Axis
}

// Layout is one workspace split tree. Its zero value is an empty valid layout.
type Layout struct {
	slots    []slot
	root     NodeID
	free     uint32
	sessions uint32
}

// New creates a workspace containing one session.
func New(session SessionID) (Layout, error) {
	var layout Layout
	if err := layout.Add(session); err != nil {
		return Layout{}, err
	}
	return layout, nil
}

// Add installs the first session in an empty layout.
func (layout *Layout) Add(session SessionID) error {
	if session == 0 {
		return ErrInvalidSession
	}
	if layout.root != 0 {
		if layout.findSession(session) != 0 {
			return ErrSessionExists
		}
		return fmt.Errorf("%w: split an existing session", ErrInvalidLayout)
	}
	leaf, err := layout.allocate(KindLeaf)
	if err != nil {
		return err
	}
	layout.mustSlot(leaf).session = session
	layout.root = leaf
	layout.sessions = 1
	return nil
}

// Root returns the root node, or zero for an empty layout.
func (layout *Layout) Root() NodeID {
	return layout.root
}

// Len returns the number of session leaves.
func (layout *Layout) Len() int {
	return int(layout.sessions)
}

// Node returns a copy of a live node.
func (layout *Layout) Node(id NodeID) (Node, bool) {
	entry, ok := layout.slot(id)
	if !ok {
		return Node{}, false
	}
	return Node{
		ID: id, Parent: entry.parent, Kind: entry.kind, Session: entry.session,
		Axis: entry.axis, Ratio: entry.ratio, First: entry.first, Second: entry.second,
	}, true
}

// Sessions appends session IDs in visual first-to-second order. Passing a
// reusable destination avoids allocations once it has enough capacity.
func (layout *Layout) Sessions(dst []SessionID) []SessionID {
	if layout.root == 0 {
		return dst
	}
	return layout.appendSessions(dst, layout.root)
}

// Split inserts session beside target. Ratio must be finite and strictly
// between zero and one; it always describes the new split's first child.
func (layout *Layout) Split(target, session SessionID, axis Axis, ratio float32, placement Placement) error {
	if session == 0 {
		return ErrInvalidSession
	}
	if err := validateSplit(axis, ratio, placement); err != nil {
		return err
	}
	if layout.findSession(session) != 0 {
		return ErrSessionExists
	}
	targetLeaf := layout.findSession(target)
	if targetLeaf == 0 {
		return ErrTargetNotFound
	}
	if !layout.canAllocate(2) {
		return ErrLayoutFull
	}
	leaf, _ := layout.allocate(KindLeaf)
	split, _ := layout.allocate(KindSplit)
	layout.mustSlot(leaf).session = session
	layout.attachSplit(split, targetLeaf, leaf, axis, ratio, placement)
	layout.sessions++
	return nil
}

// Remove deletes a session and collapses its now-redundant parent split. An
// empty result is represented by the Layout zero state.
func (layout *Layout) Remove(session SessionID) error {
	leaf := layout.findSession(session)
	if leaf == 0 {
		return ErrSessionNotFound
	}
	if leaf == layout.root {
		layout.release(leaf)
		layout.root = 0
		layout.sessions = 0
		layout.trimEmpty()
		return nil
	}
	parent := layout.mustSlot(leaf).parent
	parentSlot := layout.mustSlot(parent)
	sibling := parentSlot.first
	if sibling == leaf {
		sibling = parentSlot.second
	}
	grandparent := parentSlot.parent
	layout.replaceChild(grandparent, parent, sibling)
	layout.mustSlot(sibling).parent = grandparent
	layout.release(leaf)
	layout.release(parent)
	layout.sessions--
	return nil
}

// Move detaches session and inserts it beside target. The leaf itself remains
// live, so its NodeID is stable across the move.
func (layout *Layout) Move(session, target SessionID, axis Axis, ratio float32, placement Placement) error {
	if session == 0 || target == 0 || session == target {
		return ErrInvalidSession
	}
	if err := validateSplit(axis, ratio, placement); err != nil {
		return err
	}
	leaf := layout.findSession(session)
	if leaf == 0 {
		return ErrSessionNotFound
	}
	targetLeaf := layout.findSession(target)
	if targetLeaf == 0 {
		return ErrTargetNotFound
	}
	if !layout.canAllocate(1) {
		return ErrLayoutFull
	}
	newSplit, _ := layout.allocate(KindSplit)
	layout.detachForMove(leaf)
	layout.attachSplit(newSplit, targetLeaf, leaf, axis, ratio, placement)
	return nil
}

// SetRatio changes a split divider. It is O(1) and does not allocate.
func (layout *Layout) SetRatio(split NodeID, ratio float32) error {
	entry, ok := layout.slot(split)
	if !ok {
		return ErrInvalidNode
	}
	if entry.kind != KindSplit {
		return ErrNotSplit
	}
	if !validRatio(ratio) {
		return ErrInvalidRatio
	}
	entry.ratio = ratio
	return nil
}

// Validate checks tree, parent, free-list, and unique-session invariants.
func (layout *Layout) Validate() error {
	if layout.root == 0 {
		if layout.sessions != 0 {
			return fmt.Errorf("%w: empty root with %d sessions", ErrInvalidLayout, layout.sessions)
		}
		for index := 1; index < len(layout.slots); index++ {
			if layout.slots[index].kind != 0 {
				return fmt.Errorf("%w: live node %d without root", ErrInvalidLayout, index)
			}
		}
		return layout.validateFreeList(nil)
	}
	rootSlot, ok := layout.slot(layout.root)
	if !ok || rootSlot.parent != 0 {
		return fmt.Errorf("%w: bad root", ErrInvalidLayout)
	}
	seen := make([]bool, len(layout.slots))
	sessions := make(map[SessionID]struct{}, layout.sessions)
	stack := []NodeID{layout.root}
	var leaves uint32
	for len(stack) > 0 {
		last := len(stack) - 1
		id := stack[last]
		stack = stack[:last]
		index := id.index()
		entry, live := layout.slot(id)
		if !live || index == 0 || seen[index] {
			return fmt.Errorf("%w: repeated or dead node %d", ErrInvalidLayout, id)
		}
		seen[index] = true
		switch entry.kind {
		case KindLeaf:
			if entry.session == 0 || entry.first != 0 || entry.second != 0 || entry.axis != 0 || entry.ratio != 0 {
				return fmt.Errorf("%w: malformed leaf %d", ErrInvalidLayout, id)
			}
			if _, exists := sessions[entry.session]; exists {
				return fmt.Errorf("%w: duplicate session %d", ErrInvalidLayout, entry.session)
			}
			sessions[entry.session] = struct{}{}
			leaves++
		case KindSplit:
			if entry.session != 0 || !validAxis(entry.axis) || !validRatio(entry.ratio) || entry.first == 0 || entry.second == 0 || entry.first == entry.second {
				return fmt.Errorf("%w: malformed split %d", ErrInvalidLayout, id)
			}
			first, firstOK := layout.slot(entry.first)
			second, secondOK := layout.slot(entry.second)
			if !firstOK || !secondOK || first.parent != id || second.parent != id {
				return fmt.Errorf("%w: bad child parent at %d", ErrInvalidLayout, id)
			}
			stack = append(stack, entry.second, entry.first)
		default:
			return fmt.Errorf("%w: unknown node kind at %d", ErrInvalidLayout, id)
		}
	}
	if leaves != layout.sessions {
		return fmt.Errorf("%w: counted %d sessions, stored %d", ErrInvalidLayout, leaves, layout.sessions)
	}
	for index := 1; index < len(layout.slots); index++ {
		if layout.slots[index].kind != 0 && !seen[index] {
			return fmt.Errorf("%w: unreachable node %d", ErrInvalidLayout, index)
		}
	}
	return layout.validateFreeList(seen)
}

func (layout *Layout) appendSessions(dst []SessionID, id NodeID) []SessionID {
	entry := layout.mustSlot(id)
	if entry.kind == KindLeaf {
		return append(dst, entry.session)
	}
	dst = layout.appendSessions(dst, entry.first)
	return layout.appendSessions(dst, entry.second)
}

func (layout *Layout) findSession(session SessionID) NodeID {
	if session == 0 {
		return 0
	}
	for index := 1; index < len(layout.slots); index++ {
		entry := &layout.slots[index]
		if entry.kind == KindLeaf && entry.session == session {
			return makeNodeID(uint32(index), uint32(entry.generation))
		}
	}
	return 0
}

func (layout *Layout) attachSplit(split, target, leaf NodeID, axis Axis, ratio float32, placement Placement) {
	targetSlot := layout.mustSlot(target)
	oldParent := targetSlot.parent
	entry := layout.mustSlot(split)
	entry.parent = oldParent
	entry.axis = axis
	entry.ratio = ratio
	if placement == Before {
		entry.first, entry.second = leaf, target
	} else {
		entry.first, entry.second = target, leaf
	}
	layout.mustSlot(entry.first).parent = split
	layout.mustSlot(entry.second).parent = split
	layout.replaceChild(oldParent, target, split)
}

func (layout *Layout) detachForMove(leaf NodeID) {
	parent := layout.mustSlot(leaf).parent
	parentSlot := layout.mustSlot(parent)
	sibling := parentSlot.first
	if sibling == leaf {
		sibling = parentSlot.second
	}
	grandparent := parentSlot.parent
	layout.replaceChild(grandparent, parent, sibling)
	layout.mustSlot(sibling).parent = grandparent
	layout.mustSlot(leaf).parent = 0
	layout.release(parent)
}

func (layout *Layout) replaceChild(parent, oldChild, newChild NodeID) {
	if parent == 0 {
		layout.root = newChild
		return
	}
	entry := layout.mustSlot(parent)
	if entry.first == oldChild {
		entry.first = newChild
		return
	}
	entry.second = newChild
}

func (layout *Layout) canAllocate(count uint32) bool {
	available := uint32(0)
	for index := layout.free; index != 0 && available < count; index = layout.slots[index].nextFree {
		available++
	}
	needed := count - available
	return uint64(len(layout.slots))+uint64(needed) <= uint64(nodeIndexMask)+1
}

func (layout *Layout) allocate(kind Kind) (NodeID, error) {
	if !layout.canAllocate(1) {
		return 0, ErrLayoutFull
	}
	if layout.free != 0 {
		index := layout.free
		generation := layout.slots[index].generation
		layout.free = layout.slots[index].nextFree
		layout.slots[index] = slot{generation: generation, kind: kind}
		return makeNodeID(index, uint32(generation)), nil
	}
	index := uint32(len(layout.slots))
	if index == 0 {
		layout.slots = append(layout.slots, slot{})
		index = 1
	}
	layout.slots = append(layout.slots, slot{generation: 1, kind: kind})
	return makeNodeID(index, 1), nil
}

func (layout *Layout) release(id NodeID) {
	index := id.index()
	generation := uint32(layout.slots[index].generation) + 1
	if generation > maxGeneration {
		generation = 1
	}
	layout.slots[index] = slot{generation: uint16(generation), nextFree: layout.free}
	layout.free = index
}

func (layout *Layout) trimEmpty() {
	if layout.root == 0 {
		layout.slots = nil
		layout.free = 0
	}
}

func (layout *Layout) slot(id NodeID) (*slot, bool) {
	index := id.index()
	if index == 0 || int(index) >= len(layout.slots) {
		return nil, false
	}
	entry := &layout.slots[index]
	return entry, entry.kind != 0 && uint32(entry.generation) == id.generation()
}

func (layout *Layout) mustSlot(id NodeID) *slot {
	entry, ok := layout.slot(id)
	if !ok {
		panic("workspace: broken internal node reference")
	}
	return entry
}

func (layout *Layout) validateFreeList(live []bool) error {
	free := make([]bool, len(layout.slots))
	for index := layout.free; index != 0; index = layout.slots[index].nextFree {
		if int(index) >= len(layout.slots) || free[index] || layout.slots[index].kind != 0 {
			return fmt.Errorf("%w: corrupt free list", ErrInvalidLayout)
		}
		free[index] = true
	}
	for index := 1; index < len(layout.slots); index++ {
		isLive := live != nil && live[index]
		if !isLive && !free[index] {
			return fmt.Errorf("%w: lost free slot %d", ErrInvalidLayout, index)
		}
	}
	return nil
}

func validateSplit(axis Axis, ratio float32, placement Placement) error {
	if !validAxis(axis) {
		return ErrInvalidAxis
	}
	if !validRatio(ratio) {
		return ErrInvalidRatio
	}
	if placement != Before && placement != After {
		return ErrInvalidPlacement
	}
	return nil
}

func validAxis(axis Axis) bool {
	return axis == Horizontal || axis == Vertical
}

func validRatio(ratio float32) bool {
	return ratio > 0 && ratio < 1 && !float32NaNOrInf(ratio)
}

func float32NaNOrInf(value float32) bool {
	return math.IsNaN(float64(value)) || math.IsInf(float64(value), 0)
}

func makeNodeID(index, generation uint32) NodeID {
	return NodeID(generation<<nodeIndexBits | index)
}

func (id NodeID) index() uint32 {
	return uint32(id) & nodeIndexMask
}

func (id NodeID) generation() uint32 {
	return uint32(id) >> nodeIndexBits
}
