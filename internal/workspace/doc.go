// Package workspace owns the persistent, UI-independent arrangement of terminal
// sessions in one workspace.
//
// A Layout stores a binary split tree in a flat slot array. Leaves own session
// IDs; split nodes own an axis and the fraction assigned to their first child.
// Slots use generation-tagged compact IDs so removing and reusing a slot cannot
// make an old NodeID refer to an unrelated node.
//
// Layout is synchronous and is not safe for concurrent mutation. It starts no
// workers and performs no I/O. DTO conversion is the persistence boundary.
package workspace
