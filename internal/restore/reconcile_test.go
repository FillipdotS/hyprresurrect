package restore

import (
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/google/go-cmp/cmp"
)

// movesFor is moves over a fresh claim, the pair as Run uses them.
func movesFor(windows []liveWindow, snap snapshot.Snapshot) []Step {
	return moves(windows, snap, claim(windows, snap))
}

// regroupFor is regroup over a fresh claim, the pair as Run uses them.
func regroupFor(windows []liveWindow, snap snapshot.Snapshot) []Step {
	return regroup(windows, snap, claim(windows, snap))
}

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

	if diff := cmp.Diff(want, movesFor(windows, snap)); diff != "" {
		t.Errorf("moves() mismatch (-want +got):\n%s", diff)
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

	if diff := cmp.Diff(want, movesFor([]liveWindow{live("0x1", "foot", 9)}, snap)); diff != "" {
		t.Errorf("moves() mismatch (-want +got):\n%s", diff)
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

	if diff := cmp.Diff(want, movesFor(windows, snap)); diff != "" {
		t.Errorf("moves() mismatch (-want +got):\n%s", diff)
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

	if got := movesFor(windows, snap); len(got) != 0 {
		t.Errorf("moves() = %v, want no moves: 0x2 is already on workspace 3", got)
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

	if diff := cmp.Diff(want, movesFor(windows, snap)); diff != "" {
		t.Errorf("moves() mismatch (-want +got):\n%s", diff)
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

	if got := movesFor(windows, snap); len(got) != 0 {
		t.Errorf("moves() = %v, want no moves", got)
	}
}

func TestReconcileNothingLive(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{{Class: "foot", Workspace: 1, Command: []string{"foot"}}},
	}

	if got := movesFor(nil, snap); len(got) != 0 {
		t.Errorf("moves() = %v, want no moves", got)
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

// groupedWindow is a snapshot window that came back as part of a group.
func groupedWindow(class string, workspace, group int, active bool) snapshot.Window {
	return snapshot.Window{
		Class:       class,
		Workspace:   workspace,
		Group:       group,
		GroupActive: active,
		Command:     []string{class},
	}
}

// Two groups on one workspace: the members of each have to find their own head,
// and the tabs have to go on in the order they were saved.
func TestRegroupBuildsEachGroupFromItsOwnHead(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			groupedWindow("alpha", 1, 1, false),
			groupedWindow("beta", 1, 1, false),
			groupedWindow("gamma", 1, 2, false),
			{Class: "loner", Workspace: 1, Command: []string{"loner"}},
			groupedWindow("delta", 1, 2, false),
		},
	}

	windows := []liveWindow{
		live("0xALPHA", "alpha", 1, "alpha"),
		live("0xBETA", "beta", 1, "beta"),
		live("0xGAMMA", "gamma", 1, "gamma"),
		live("0xLONER", "loner", 1, "loner"),
		live("0xDELTA", "delta", 1, "delta"),
	}

	want := []Step{
		{
			What: "group alpha",
			Lua:  `hl.dispatch(hl.dsp.group.toggle({window = "address:0xALPHA"}))`,
		},
		{
			What: "tab beta into the alpha group",
			Lua:  `hl.get_window("address:0xALPHA").group:add(hl.get_window("address:0xBETA"))`,
		},
		{
			What: "group gamma",
			Lua:  `hl.dispatch(hl.dsp.group.toggle({window = "address:0xGAMMA"}))`,
		},
		{
			What: "tab delta into the gamma group",
			Lua:  `hl.get_window("address:0xGAMMA").group:add(hl.get_window("address:0xDELTA"))`,
		},
	}

	if diff := cmp.Diff(want, regroupFor(windows, snap)); diff != "" {
		t.Errorf("regroup() mismatch (-want +got):\n%s", diff)
	}
}

func raiseTabsFor(windows []liveWindow, snap snapshot.Snapshot) []Step {
	return raiseTabs(windows, snap, claim(windows, snap))
}

func TestRaiseTabsPutsTheSavedTabBackUp(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			groupedWindow("alpha", 2, 1, true),
			groupedWindow("beta", 2, 1, false),
		},
	}

	windows := []liveWindow{
		live("0xALPHA", "alpha", 2, "alpha"),
		live("0xBETA", "beta", 2, "beta"),
	}

	want := []Step{{
		What: "raise tab 1 of the alpha group",
		Lua:  `hl.dispatch(hl.dsp.group.active({window = "address:0xALPHA", index = 1}))`,
	}}

	if diff := cmp.Diff(want, raiseTabsFor(windows, snap)); diff != "" {
		t.Errorf("raiseTabs() mismatch (-want +got):\n%s", diff)
	}
}

func TestRaiseTabsNamesTheLastTabToo(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			groupedWindow("alpha", 2, 1, false),
			groupedWindow("beta", 2, 1, true),
		},
	}

	windows := []liveWindow{
		live("0xALPHA", "alpha", 2, "alpha"),
		live("0xBETA", "beta", 2, "beta"),
	}

	want := []Step{{
		What: "raise tab 2 of the alpha group",
		Lua:  `hl.dispatch(hl.dsp.group.active({window = "address:0xALPHA", index = 2}))`,
	}}

	if diff := cmp.Diff(want, raiseTabsFor(windows, snap)); diff != "" {
		t.Errorf("raiseTabs() mismatch (-want +got):\n%s", diff)
	}
}

func TestRaiseTabsSaysNothingWhenNoTabWasRecorded(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			groupedWindow("alpha", 2, 1, false),
			groupedWindow("beta", 2, 1, false),
		},
	}

	windows := []liveWindow{
		live("0xALPHA", "alpha", 2, "alpha"),
		live("0xBETA", "beta", 2, "beta"),
	}

	if got := raiseTabsFor(windows, snap); len(got) != 0 {
		t.Errorf("raiseTabs() = %v, want none", got)
	}
}
