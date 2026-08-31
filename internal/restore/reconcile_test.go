package restore

import (
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/google/go-cmp/cmp"
)

func TestReconcilePairsWindowsOfAClassInOrder(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 1, Command: []string{"foot"}},
			{Class: "foot", Workspace: 2, Command: []string{"foot"}},
		},
	}

	live := []hypr.Client{
		{Address: "0x1", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 1}},
		{Address: "0x2", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 9}},
	}

	want := []Step{{
		What: "move foot to workspace 2",
		Lua:  `hl.dispatch(hl.dsp.window.move({window = "address:0x2", workspace = 2}))`,
	}}

	if diff := cmp.Diff(want, reconcile(live, snap)); diff != "" {
		t.Errorf("reconcile() mismatch (-want +got):\n%s", diff)
	}
}

// The terminal the restore was launched from is live but not in the snapshot,
// and nothing in the snapshot claims it.
func TestReconcileLeavesUnknownWindowsAlone(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 1, Command: []string{"foot"}},
		},
	}

	live := []hypr.Client{
		{Address: "0x1", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 1}},
		{Address: "0x2", Class: "com.mitchellh.ghostty", Workspace: hypr.WorkspaceRef{ID: 9}},
	}

	if got := reconcile(live, snap); len(got) != 0 {
		t.Errorf("reconcile() = %v, want no moves", got)
	}
}

// A window with no captured command was never spawned, so it must not claim a
// live window of the same class.
func TestReconcileIgnoresWindowsWithoutACommand(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 1},
			{Class: "foot", Workspace: 2, Command: []string{"foot"}},
		},
	}

	live := []hypr.Client{
		{Address: "0x1", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 9}},
	}

	want := []Step{{
		What: "move foot to workspace 2",
		Lua:  `hl.dispatch(hl.dsp.window.move({window = "address:0x1", workspace = 2}))`,
	}}

	if diff := cmp.Diff(want, reconcile(live, snap)); diff != "" {
		t.Errorf("reconcile() mismatch (-want +got):\n%s", diff)
	}
}

func TestReconcileNothingLive(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{{Class: "foot", Workspace: 1, Command: []string{"foot"}}},
	}

	if got := reconcile(nil, snap); len(got) != 0 {
		t.Errorf("reconcile() = %v, want no moves", got)
	}
}

func TestTargetsCountsWindowsPerClassAndWorkspace(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 3, Command: []string{"foot"}},
			{Class: "ghostty", Workspace: 2, Command: []string{"ghostty"}},
			{Class: "ghostty", Workspace: 2, Command: []string{"ghostty"}},
			{Class: "Aseprite", Workspace: 2, Command: []string{"aseprite"}},
			{Class: "dead", Workspace: 2},
		},
	}

	want := []target{
		{class: "Aseprite", workspace: 2, count: 1},
		{class: "ghostty", workspace: 2, count: 2},
		{class: "foot", workspace: 3, count: 1},
	}

	if diff := cmp.Diff(want, targets(snap), cmp.AllowUnexported(target{})); diff != "" {
		t.Errorf("targets() mismatch (-want +got):\n%s", diff)
	}
}
