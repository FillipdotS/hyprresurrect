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

// claim decides which live window each snapshot window describes, as the index
// of that live window, or -1 for a snapshot window nothing came back for.
//
// An app with a single-instance mode (ghostty --gtk-single-instance, a
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
func claim(live []liveWindow, snap snapshot.Snapshot) []int {
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

	return claimed
}

// moves returns the steps for the windows the spawn rules could not place: the
// single-instance apps that ignored them, and anything that landed on the
// active workspace instead of its own.
func moves(live []liveWindow, snap snapshot.Snapshot, claimed []int) []Step {
	var steps []Step

	for wi, w := range snap.Windows {
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

// regroup returns the steps that rebuild the groups the snapshot recorded. The
// group spawn rule is no use here: a restore spawns with no_initial_focus onto
// a silent workspace, and with nothing focused there "group set" gives every
// window its own group of one. So groups are built afterwards, by address, off
// the HL.Group object - no direction, no focus, no geometry. See plan.md.
//
// Members are added in snapshot order, which is tab order; raiseTabs puts the
// recorded tab back up once everything else has settled.
func regroup(live []liveWindow, snap snapshot.Snapshot, claimed []int) []Step {
	var steps []Step

	for _, g := range groups(snap, claimed) {
		head := live[claimed[g.members[0]]]
		headSelector := luaString("address:" + head.Address)

		steps = append(steps, Step{
			What: "group " + head.Class,
			Lua:  fmt.Sprintf("hl.dispatch(hl.dsp.group.toggle({window = %s}))", headSelector),
		})

		for _, wi := range g.members[1:] {
			member := live[claimed[wi]]

			steps = append(steps, Step{
				What: fmt.Sprintf("tab %s into the %s group", member.Class, head.Class),
				Lua: fmt.Sprintf("hl.get_window(%s).group:add(hl.get_window(%s))",
					headSelector, luaString("address:"+member.Address)),
			})
		}
	}

	return steps
}

func raiseTabs(live []liveWindow, snap snapshot.Snapshot, claimed []int) []Step {
	var steps []Step

	for _, g := range groups(snap, claimed) {
		if g.active == 0 {
			continue
		}

		head := live[claimed[g.members[0]]]

		steps = append(steps, Step{
			What: fmt.Sprintf("raise tab %d of the %s group", g.active, head.Class),
			Lua: fmt.Sprintf("hl.dispatch(hl.dsp.group.active({window = %s, index = %d}))",
				luaString("address:"+head.Address), g.active),
		})
	}

	return steps
}

type group struct {
	members []int
	active  int
}

// groups gathers the snapshot's groups in tab order. A nil claimed counts every
// window, which is what --dry-run wants before anything has been spawned.
func groups(snap snapshot.Snapshot, claimed []int) []group {
	var (
		out  []group
		byID = make(map[int]int) // snapshot group id -> index into out
	)

	for wi, w := range snap.Windows {
		if w.Group == 0 || (claimed != nil && claimed[wi] < 0) {
			continue
		}

		i, seen := byID[w.Group]
		if !seen {
			i = len(out)
			byID[w.Group] = i

			out = append(out, group{})
		}

		out[i].members = append(out[i].members, wi)

		if w.GroupActive {
			out[i].active = len(out[i].members)
		}
	}

	return out
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

// groupTargets describes the groups a restore would rebuild, for --dry-run.
// Unlike regroup it works off the snapshot alone, before anything is spawned.
func groupTargets(snap snapshot.Snapshot) []groupTarget {
	var out []groupTarget

	for _, g := range groups(snap, nil) {
		t := groupTarget{workspace: snap.Windows[g.members[0]].Workspace}

		for _, wi := range g.members {
			t.classes = append(t.classes, snap.Windows[wi].Class)
		}

		out = append(out, t)
	}

	return out
}

type groupTarget struct {
	workspace int
	classes   []string
}
