package restore

import (
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/google/go-cmp/cmp"
)

func TestPlanBindsWorkspacesBeforeSpawning(t *testing.T) {
	snap := snapshot.Snapshot{
		Monitors: []snapshot.Monitor{
			{Name: "HDMI-A-1", ActiveWorkspace: 3},
			{Name: "DP-1", ActiveWorkspace: 5},
		},
		Windows: []snapshot.Window{
			{
				Class:     "foot",
				Workspace: 3,
				Monitor:   "HDMI-A-1",
				At:        [2]int{3628, 30},
				Size:      [2]int{590, 516},
				Command:   []string{"foot", "-e", "cliamp"},
			},
			{
				Class:     "com.mitchellh.ghostty",
				Workspace: 5,
				Monitor:   "DP-1",
				At:        [2]int{4, 28},
				Size:      [2]int{713, 629},
				Floating:  true,
				Command:   []string{"/usr/bin/ghostty"},
			},
		},
	}

	want := []Step{
		{
			What: "bind workspace 3 to HDMI-A-1",
			Lua:  `hl.workspace_rule({workspace = "3", monitor = "HDMI-A-1"})`,
		},
		{
			What: "bind workspace 5 to DP-1",
			Lua:  `hl.workspace_rule({workspace = "5", monitor = "DP-1"})`,
		},
		{
			What: "spawn foot",
			Lua: `hl.exec_cmd("foot -e cliamp", ` +
				`{workspace = "3 silent", no_initial_focus = true})`,
		},
		{
			What: "spawn com.mitchellh.ghostty",
			Lua: `hl.exec_cmd("/usr/bin/ghostty", {workspace = "5 silent", ` +
				`float = true, size = "713 629", move = "4 28", ` +
				`no_initial_focus = true})`,
		},
	}

	if diff := cmp.Diff(want, Plan(snap)); diff != "" {
		t.Errorf("Plan() mismatch (-want +got):\n%s", diff)
	}
}

// A workspace holding several windows must be bound once, not once per window:
// re-registering the rule is harmless but the duplicate statements are noise in
// --dry-run output.
func TestPlanBindsEachWorkspaceOnce(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "a", Workspace: 2, Monitor: "DP-1", Command: []string{"a"}},
			{Class: "b", Workspace: 2, Monitor: "DP-1", Command: []string{"b"}},
		},
	}

	var binds int
	for _, step := range Plan(snap) {
		if step.Lua[:len("hl.workspace_rule")] == "hl.workspace_rule" {
			binds++
		}
	}

	if binds != 1 {
		t.Errorf("Plan() emitted %d workspace_rule steps, want 1", binds)
	}
}

// A tiled window ignores exact geometry, so move/size are dead weight on it.
func TestPlanOmitsGeometryWhenTiled(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{{
			Class:     "Aseprite",
			Workspace: 2,
			Monitor:   "DP-1",
			At:        [2]int{4, 56},
			Size:      [2]int{1145, 1236},
			Command:   []string{"aseprite"},
		}},
	}

	want := []Step{
		{
			What: "bind workspace 2 to DP-1",
			Lua:  `hl.workspace_rule({workspace = "2", monitor = "DP-1"})`,
		},
		{
			What: "spawn Aseprite",
			Lua: `hl.exec_cmd("aseprite", ` +
				`{workspace = "2 silent", no_initial_focus = true})`,
		},
	}

	if diff := cmp.Diff(want, Plan(snap)); diff != "" {
		t.Errorf("Plan() mismatch (-want +got):\n%s", diff)
	}
}

func TestPlanEmptySnapshot(t *testing.T) {
	if got := Plan(snapshot.Snapshot{}); len(got) != 0 {
		t.Errorf("Plan(empty) = %v, want no steps", got)
	}
}
