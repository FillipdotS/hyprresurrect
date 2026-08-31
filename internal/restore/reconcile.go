package restore

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
)

// reconcile returns the moves needed for windows the spawn rules could not
// place. An app with a single-instance mode (ghostty --gtk-single-instance, a
// second firefox) creates its window from a process that was already running,
// so exec_cmd's rules never attach to it and it lands on the active workspace.
//
// Live windows are matched to snapshot windows per class, in order. Windows of
// one class are interchangeable for placement purposes, so pairing them
// arbitrarily is correct rather than a compromise: ten identical terminals only
// need to end up on the right ten workspaces, not in a specific order.
func reconcile(live []hypr.Client, snap snapshot.Snapshot) []Step {
	byClass := make(map[string][]hypr.Client, len(live))
	for _, c := range live {
		byClass[c.Class] = append(byClass[c.Class], c)
	}

	// Windows not in the snapshot - the terminal the restore was started from,
	// anything opened since - have no entry here and are left alone.
	taken := make(map[string]int, len(byClass))

	var steps []Step
	for _, w := range restorable(snap) {
		i := taken[w.Class]
		if i >= len(byClass[w.Class]) {
			continue
		}
		taken[w.Class]++

		c := byClass[w.Class][i]
		if c.Workspace.ID == w.Workspace {
			continue
		}

		steps = append(steps, Step{
			What: fmt.Sprintf("move %s to workspace %d", w.Class, w.Workspace),
			Lua: fmt.Sprintf("hl.dispatch(hl.dsp.window.move({window = %s, workspace = %d}))",
				luaString("address:"+c.Address), w.Workspace),
		})
	}

	return steps
}

type target struct {
	class     string
	workspace int
	count     int
}

// targets is the placement the move pass enforces: how many windows of each
// class belong on each workspace. A dry run can't know where the windows will
// actually land - that's the whole reason the move pass exists - so what it
// reconciles *against* is the honest preview of it.
func targets(snap snapshot.Snapshot) []target {
	counts := make(map[target]int)
	for _, w := range restorable(snap) {
		counts[target{class: w.Class, workspace: w.Workspace}]++
	}

	out := make([]target, 0, len(counts))
	for t, n := range counts {
		t.count = n
		out = append(out, t)
	}

	slices.SortFunc(out, func(a, b target) int {
		if c := cmp.Compare(a.workspace, b.workspace); c != 0 {
			return c
		}

		return strings.Compare(a.class, b.class)
	})

	return out
}
