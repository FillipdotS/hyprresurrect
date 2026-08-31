// Package restore turns a snapshot back into a running session.
package restore

import "github.com/FillipdotS/hyprresurrect/internal/snapshot"

// A Step is one Lua statement to send, with a label for --dry-run output and
// for reporting which window failed.
type Step struct {
	What string
	Lua  string
}

// Plan turns a snapshot into the ordered statements that recreate it: every
// workspace binding first, since a workspace rule only applies at creation
// time, then one spawn per window.
//
// TODO: unimplemented. restore_test.go describes the intended output.
func Plan(snap snapshot.Snapshot) []Step {
	return nil
}
