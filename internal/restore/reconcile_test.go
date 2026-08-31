package restore

import (
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/google/go-cmp/cmp"
)

func live(address, class string, workspace int, command ...string) liveWindow {
	return liveWindow{
		Client: hypr.Client{
			Address:   address,
			Class:     class,
			Workspace: hypr.WorkspaceRef{ID: workspace},
		},
		Command: command,
	}
}

// The case that class-only matching gets wrong: two terminals running different
// programs, both of which missed their spawn rule. Matching on class alone sees
// one foot on each workspace, calls it done, and leaves them swapped.
func TestReconcileKeepsEachCommandOnItsOwnWorkspace(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 3, Command: []string{"foot", "-e", "cliamp"}},
			{Class: "foot", Workspace: 5, Command: []string{"foot", "-e", "btop"}},
		},
	}

	windows := []liveWindow{
		live("0xBTOP", "foot", 3, "foot", "-e", "btop"),
		live("0xCLIAMP", "foot", 5, "foot", "-e", "cliamp"),
	}

	want := []Step{
		{
			What: "move foot to workspace 3",
			Lua:  `hl.dispatch(hl.dsp.window.move({window = "address:0xCLIAMP", workspace = 3}))`,
		},
		{
			What: "move foot to workspace 5",
			Lua:  `hl.dispatch(hl.dsp.window.move({window = "address:0xBTOP", workspace = 5}))`,
		},
	}

	if diff := cmp.Diff(want, reconcile(windows, snap)); diff != "" {
		t.Errorf("reconcile() mismatch (-want +got):\n%s", diff)
	}
}

// Windows a command cannot separate - one pid serving several windows, or an
// argv that could not be read back - still have to be placed, on class alone.
func TestReconcileFallsBackToClassWithoutACommand(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 3, Command: []string{"foot", "-e", "cliamp"}},
		},
	}

	want := []Step{{
		What: "move foot to workspace 3",
		Lua:  `hl.dispatch(hl.dsp.window.move({window = "address:0x1", workspace = 3}))`,
	}}

	if diff := cmp.Diff(want, reconcile([]liveWindow{live("0x1", "foot", 9)}, snap)); diff != "" {
		t.Errorf("reconcile() mismatch (-want +got):\n%s", diff)
	}
}

func TestReconcilePairsInterchangeableWindows(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 1, Command: []string{"foot"}},
			{Class: "foot", Workspace: 2, Command: []string{"foot"}},
		},
	}

	windows := []liveWindow{
		live("0x1", "foot", 1, "foot"),
		live("0x2", "foot", 9, "foot"),
	}

	want := []Step{{
		What: "move foot to workspace 2",
		Lua:  `hl.dispatch(hl.dsp.window.move({window = "address:0x2", workspace = 2}))`,
	}}

	if diff := cmp.Diff(want, reconcile(windows, snap)); diff != "" {
		t.Errorf("reconcile() mismatch (-want +got):\n%s", diff)
	}
}

// Matching by class alone cannot tell a restored window from one that was
// already open. The terminal the restore was launched from shares foot's class
// with a window that spawned exactly where it belongs, and must not be dragged
// off its own workspace to satisfy it.
func TestReconcileLeavesCorrectlyPlacedWindowsAlone(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 3, Command: []string{"foot"}},
		},
	}

	windows := []liveWindow{
		live("0x1", "foot", 1, "foot"),
		live("0x2", "foot", 3, "foot"),
	}

	if got := reconcile(windows, snap); len(got) != 0 {
		t.Errorf("reconcile() = %v, want no moves: 0x2 is already on workspace 3", got)
	}
}

// Pairing purely in list order moves every window when only one is misplaced,
// shuffling windows that were already right.
func TestReconcileMovesOnlyTheMisplacedWindow(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 1, Command: []string{"foot"}},
			{Class: "foot", Workspace: 2, Command: []string{"foot"}},
		},
	}

	windows := []liveWindow{
		live("0x1", "foot", 2, "foot"),
		live("0x2", "foot", 9, "foot"),
	}

	want := []Step{{
		What: "move foot to workspace 1",
		Lua:  `hl.dispatch(hl.dsp.window.move({window = "address:0x2", workspace = 1}))`,
	}}

	if diff := cmp.Diff(want, reconcile(windows, snap)); diff != "" {
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

	windows := []liveWindow{
		live("0x1", "foot", 1, "foot"),
		live("0x2", "com.mitchellh.ghostty", 9, "ghostty"),
	}

	if got := reconcile(windows, snap); len(got) != 0 {
		t.Errorf("reconcile() = %v, want no moves", got)
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
