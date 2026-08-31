package restore

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
)

// A liveWindow is a window and the command behind it, which only tells two
// windows apart when one class runs several. This will be mostly terminals
type liveWindow struct {
	hypr.Client
	Command []string
}

// reconcile returns the moves needed for windows the spawn rules could not
// place. An app with a single-instance mode (ghostty --gtk-single-instance, a
// second firefox) creates its window from a process that was already running,
// so exec_cmd's rules never attach to it and it lands on the active workspace.
//
// Each snapshot window claims the live window it describes, strongest evidence
// first: an identical command before a shared class, and within each of
// those the windows that are already on the right workspace before the ones
// that would have to move. Command first is what keeps cliamp on its own
// workspace instead of swapping it with the btop next to it. Class is only a
// fallback, for the windows a command genuinely cannot separate - seven ghostty
// windows sharing one pid report one identical argv, and those really are
// interchangeable.
//
// Whatever is left unclaimed - the terminal the restore was started from,
// anything opened since - is left alone.
func reconcile(live []liveWindow, snap snapshot.Snapshot) []Step {
	windows := snap.Windows

	claimed := make([]int, len(windows))
	for i := range claimed {
		claimed[i] = -1
	}
	taken := make([]bool, len(live))

	for _, pass := range []struct{ sameCommand, onTarget bool }{
		{sameCommand: true, onTarget: true},
		{sameCommand: true},
		{onTarget: true},
		{},
	} {
		for wi, w := range windows {
			if claimed[wi] >= 0 {
				continue
			}

			for li, l := range live {
				if taken[li] || l.Class != w.Class {
					continue
				}
				if pass.sameCommand && !slices.Equal(l.Command, w.Command) {
					continue
				}
				if pass.onTarget && l.Workspace.ID != w.Workspace {
					continue
				}

				taken[li], claimed[wi] = true, li

				break
			}
		}
	}

	var steps []Step
	for wi, w := range windows {
		li := claimed[wi]
		if li < 0 || live[li].Workspace.ID == w.Workspace {
			continue
		}

		steps = append(steps, Step{
			What: fmt.Sprintf("move %s to workspace %d", w.Class, w.Workspace),
			Lua: fmt.Sprintf("hl.dispatch(hl.dsp.window.move({window = %s, workspace = %d}))",
				luaString("address:"+live[li].Address), w.Workspace),
		})
	}

	return steps
}

type target struct {
	class     string
	workspace int
	count     int
}

func targets(snap snapshot.Snapshot) []target {
	counts := make(map[target]int)
	for _, w := range snap.Windows {
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
