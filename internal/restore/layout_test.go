package restore

import (
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/google/go-cmp/cmp"
)

func relayoutFor(windows []liveWindow, snap snapshot.Snapshot) []Step {
	return relayout(windows, snap, claim(windows, snap))
}

func placed(address, class string, workspace, x, y, w, h int) liveWindow {
	return liveWindow{
		Client: hypr.Client{
			Address:   address,
			Class:     class,
			Workspace: hypr.WorkspaceRef{ID: workspace},
			At:        [2]int{x, y},
			Size:      [2]int{w, h},
		},
		Command: []string{class},
	}
}

func sideBySide() snapshot.Snapshot {
	return snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "alpha", Workspace: 1, At: [2]int{0, 0}, Size: [2]int{600, 1000}, Command: []string{"alpha"}},
			{Class: "beta", Workspace: 1, At: [2]int{600, 0}, Size: [2]int{400, 1000}, Command: []string{"beta"}},
		},
		Layouts: []snapshot.Layout{{
			Workspace: 1,
			Root: &snapshot.Node{
				Toward: snapshot.SplitRight,
				First:  snapshot.Leaf(0),
				Second: snapshot.Leaf(1),
			},
		}},
	}
}

func evenSplit() []liveWindow {
	return []liveWindow{
		placed("0xA", "alpha", 1, 0, 0, 500, 1000),
		placed("0xB", "beta", 1, 500, 0, 500, 1000),
	}
}

func TestRelayoutParksThenTilesBackInOrder(t *testing.T) {
	want := []Step{
		{
			What: "hold split directions still",
			Lua: `hyprresurrect_preserve_split = hl.get_config("dwindle:preserve_split") ` +
				`hl.config({dwindle = {preserve_split = true}})`,
		},
		{
			What: "park alpha",
			Lua: `hl.dispatch(hl.dsp.window.move({window = "address:0xA", ` +
				`workspace = "special:hyprresurrect", silent = true}))`,
		},
		{
			What: "park beta",
			Lua: `hl.dispatch(hl.dsp.window.move({window = "address:0xB", ` +
				`workspace = "special:hyprresurrect", silent = true}))`,
		},
		{
			What: "tile alpha alone on workspace 1",
			Lua:  `hl.dispatch(hl.dsp.window.move({window = "address:0xA", workspace = "1", silent = true}))`,
		},
		{
			What: "tile beta right of alpha",
			Lua: `hl.dispatch(hl.dsp.focus({window = "address:0xA"})) ` +
				`hl.dispatch(hl.dsp.layout("preselect r")) ` +
				`hl.dispatch(hl.dsp.window.move({window = "address:0xB", workspace = "1", silent = true}))`,
		},
		{
			What: "size alpha to 600x1000",
			Lua:  `hl.dispatch(hl.dsp.window.resize({window = "address:0xA", x = 600, y = 1000, exact = true}))`,
		},
		{
			What: "size beta to 400x1000",
			Lua:  `hl.dispatch(hl.dsp.window.resize({window = "address:0xB", x = 400, y = 1000, exact = true}))`,
		},
		{
			What: "put preserve_split back",
			Lua: `hl.config({dwindle = {preserve_split = hyprresurrect_preserve_split}}) ` +
				`hyprresurrect_preserve_split = nil`,
		},
	}

	if diff := cmp.Diff(want, relayoutFor(evenSplit(), sideBySide())); diff != "" {
		t.Errorf("relayout() mismatch (-want +got):\n%s", diff)
	}
}

func TestRelayoutStacksDownwards(t *testing.T) {
	snap := sideBySide()
	snap.Layouts[0].Root.Toward = snapshot.SplitDown

	want := `hl.dispatch(hl.dsp.focus({window = "address:0xA"})) ` +
		`hl.dispatch(hl.dsp.layout("preselect d")) ` +
		`hl.dispatch(hl.dsp.window.move({window = "address:0xB", workspace = "1", silent = true}))`

	steps := relayoutFor(evenSplit(), snap)
	if got := steps[4].Lua; got != want {
		t.Errorf("relayout() split step = %q, want %q", got, want)
	}
}

func TestRelayoutScalesToTheMonitorItFindsNow(t *testing.T) {
	live := []liveWindow{
		placed("0xA", "alpha", 1, 0, 0, 1000, 2000),
		placed("0xB", "beta", 1, 1000, 0, 1000, 2000),
	}

	want := []string{"size alpha to 1200x2000", "size beta to 800x2000"}

	steps := relayoutFor(live, sideBySide())

	got := []string{steps[5].What, steps[6].What}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("relayout() resize steps mismatch (-want +got):\n%s", diff)
	}
}

func TestRelayoutLeavesALoneWindowAlone(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "alpha", Workspace: 1, Size: [2]int{600, 1000}, Command: []string{"alpha"}},
		},
		Layouts: []snapshot.Layout{{Workspace: 1, Root: snapshot.Leaf(0)}},
	}

	live := []liveWindow{placed("0xA", "alpha", 1, 0, 0, 600, 1000)}

	if got := relayoutFor(live, snap); got != nil {
		t.Errorf("relayout() = %v, want none", got)
	}
}

func TestRelayoutSkipsAWorkspaceThatIsMissingAWindow(t *testing.T) {
	live := []liveWindow{placed("0xA", "alpha", 1, 0, 0, 1000, 1000)}

	if got := relayoutFor(live, sideBySide()); got != nil {
		t.Errorf("relayout() = %v, want none", got)
	}
}

func TestRelayoutSkipsAWorkspaceWithAWindowItDoesNotOwn(t *testing.T) {
	live := append(evenSplit(), placed("0xC", "stranger", 1, 0, 0, 100, 100))

	if got := relayoutFor(live, sideBySide()); got != nil {
		t.Errorf("relayout() = %v, want none", got)
	}
}

func TestRelayoutIgnoresAFloatingStranger(t *testing.T) {
	stranger := placed("0xC", "stranger", 1, 0, 0, 100, 100)
	stranger.Floating = true

	if got := relayoutFor(append(evenSplit(), stranger), sideBySide()); got == nil {
		t.Error("relayout() = none, want the workspace rebuilt anyway")
	}
}

func TestRelayoutSaysNothingWithoutALayout(t *testing.T) {
	snap := sideBySide()
	snap.Layouts = nil

	if got := relayoutFor(evenSplit(), snap); got != nil {
		t.Errorf("relayout() = %v, want none", got)
	}
}

func TestLayoutTargetsDrawTheTree(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "alpha"}, {Class: "beta"}, {Class: "gamma"},
		},
		Layouts: []snapshot.Layout{{
			Workspace: 3,
			Root: &snapshot.Node{
				Toward: snapshot.SplitRight,
				First:  snapshot.Leaf(0),
				Second: &snapshot.Node{
					Toward: snapshot.SplitDown,
					First:  snapshot.Leaf(1),
					Second: snapshot.Leaf(2),
				},
			},
		}},
	}

	want := []layoutTarget{{workspace: 3, tree: "(alpha | (beta / gamma))"}}

	if diff := cmp.Diff(want, layoutTargets(snap), cmp.AllowUnexported(layoutTarget{})); diff != "" {
		t.Errorf("layoutTargets() mismatch (-want +got):\n%s", diff)
	}
}
