package restore

import (
	"fmt"

	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
)

const (
	parkingWorkspace = "special:hyprresurrect"
	splitHolder      = "hyprresurrect_preserve_split"
)

// TODO: skip if not dwindle
func relayout(live []liveWindow, snap snapshot.Snapshot, claimed []int) []Step {
	var steps []Step

	for _, layout := range snap.Layouts {
		steps = append(steps, rebuild(layout, live, snap, claimed)...)
	}

	if len(steps) == 0 {
		return nil
	}

	// TODO: If restore crashes/fails, then we might not restore the users 'preserve_split' option
	held := Step{
		What: "hold split directions still",
		Lua: fmt.Sprintf(`%s = hl.get_config("dwindle:preserve_split") `+
			`hl.config({dwindle = {preserve_split = true}})`, splitHolder),
	}
	released := Step{
		What: "put preserve_split back",
		Lua:  fmt.Sprintf(`hl.config({dwindle = {preserve_split = %s}}) %s = nil`, splitHolder, splitHolder),
	}

	return append(append([]Step{held}, steps...), released)
}

func rebuild(layout snapshot.Layout, live []liveWindow, snap snapshot.Snapshot, claimed []int) []Step {
	leaves := layout.Root.Leaves()
	if len(leaves) < 2 {
		return nil
	}

	selector := make(map[int]string, len(leaves))

	for _, wi := range leaves {
		li := claimed[wi]
		if li < 0 {
			return nil
		}

		selector[wi] = luaString("address:" + live[li].Address)
	}

	if hasStrangers(live, claimed, layout.Workspace) {
		return nil
	}

	workspace := luaString(fmt.Sprintf("%d", layout.Workspace))
	class := func(wi int) string { return snap.Windows[wi].Class }

	var steps []Step

	for _, wi := range leaves {
		steps = append(steps, Step{
			What: fmt.Sprintf("park %s", class(wi)),
			Lua:  moveTo(selector[wi], luaString(parkingWorkspace)),
		})
	}

	first := layout.Root.FirstLeaf()

	steps = append(steps, Step{
		What: fmt.Sprintf("tile %s alone on workspace %d", class(first), layout.Workspace),
		Lua:  moveTo(selector[first], workspace),
	})

	for _, split := range layout.Root.SplitsParentsFirst() {
		anchor, next := split.First.FirstLeaf(), split.Second.FirstLeaf()

		steps = append(steps, Step{
			What: fmt.Sprintf("tile %s %s of %s", class(next), split.Toward, class(anchor)),
			Lua: fmt.Sprintf("hl.dispatch(hl.dsp.focus({window = %s})) %s %s",
				selector[anchor], preselect(split.Toward), moveTo(selector[next], workspace)),
		})
	}

	factor := rescale(layout, live, snap, claimed)

	for _, wi := range leaves {
		size := scaled(snap.Windows[wi].Size, factor)

		steps = append(steps, Step{
			What: fmt.Sprintf("size %s to %dx%d", class(wi), size[0], size[1]),
			Lua: fmt.Sprintf("hl.dispatch(hl.dsp.window.resize({window = %s, x = %d, y = %d, exact = true}))",
				selector[wi], size[0], size[1]),
		})
	}

	return steps
}

func moveTo(window, workspace string) string {
	return fmt.Sprintf("hl.dispatch(hl.dsp.window.move({window = %s, workspace = %s, silent = true}))",
		window, workspace)
}

func preselect(toward string) string {
	direction := "r"
	if toward == snapshot.SplitDown {
		direction = "d"
	}

	return fmt.Sprintf("hl.dispatch(hl.dsp.layout(%s))", luaString("preselect "+direction))
}

func hasStrangers(live []liveWindow, claimed []int, workspace int) bool {
	ours := make(map[int]bool, len(claimed))
	for _, li := range claimed {
		if li >= 0 {
			ours[li] = true
		}
	}

	for li, l := range live {
		if l.Workspace.ID == workspace && !l.Floating && !ours[li] {
			return true
		}
	}

	return false
}

type box struct {
	at   [2]int
	size [2]int
}

func rescale(layout snapshot.Layout, live []liveWindow, snap snapshot.Snapshot, claimed []int) [2]float64 {
	var saved, current []box

	for _, wi := range layout.Root.Leaves() {
		w := snap.Windows[wi]
		saved = append(saved, box{at: w.At, size: w.Size})

		l := live[claimed[wi]]
		current = append(current, box{at: l.At, size: l.Size})
	}

	was, is := span(saved), span(current)

	factor := [2]float64{1, 1}

	for axis := range factor {
		if was[axis] > 0 && is[axis] > 0 {
			factor[axis] = float64(is[axis]) / float64(was[axis])
		}
	}

	return factor
}

func span(boxes []box) [2]int {
	if len(boxes) == 0 {
		return [2]int{}
	}

	lo, hi := boxes[0].at, [2]int{}

	for axis := range hi {
		hi[axis] = boxes[0].at[axis] + boxes[0].size[axis]
	}

	for _, b := range boxes[1:] {
		for axis := range 2 {
			lo[axis] = min(lo[axis], b.at[axis])
			hi[axis] = max(hi[axis], b.at[axis]+b.size[axis])
		}
	}

	return [2]int{hi[0] - lo[0], hi[1] - lo[1]}
}

func scaled(size [2]int, factor [2]float64) [2]int {
	var out [2]int
	for axis := range out {
		out[axis] = int(float64(size[axis])*factor[axis] + 0.5)
	}

	return out
}

type layoutTarget struct {
	workspace int
	tree      string
}

func layoutTargets(snap snapshot.Snapshot) []layoutTarget {
	var out []layoutTarget

	for _, layout := range snap.Layouts {
		if len(layout.Root.Leaves()) < 2 {
			continue
		}

		out = append(out, layoutTarget{
			workspace: layout.Workspace,
			tree:      renderTree(layout.Root, snap.Windows),
		})
	}

	return out
}

func renderTree(n *snapshot.Node, windows []snapshot.Window) string {
	if n.IsLeaf() {
		return windows[*n.Window].Class
	}

	separator := " | "
	if n.Toward == snapshot.SplitDown {
		separator = " / "
	}

	return "(" + renderTree(n.First, windows) + separator + renderTree(n.Second, windows) + ")"
}
