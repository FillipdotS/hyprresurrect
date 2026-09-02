package snapshot

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func tiled(class string, workspace, x, y, w, h int) Window {
	return Window{
		Class:     class,
		Workspace: workspace,
		At:        [2]int{x, y},
		Size:      [2]int{w, h},
	}
}

func split(toward string, first, second *Node) *Node {
	return &Node{Toward: toward, First: first, Second: second}
}

func TestLayoutsRebuildTwoColumns(t *testing.T) {
	windows := []Window{
		tiled("a", 1, 1, 1, 426, 240),
		tiled("b", 1, 429, 1, 850, 240),
		tiled("c", 1, 429, 243, 850, 476),
		tiled("d", 1, 1, 243, 426, 476),
	}

	want := []Layout{{
		Workspace: 1,
		Root: split(SplitRight,
			split(SplitDown, Leaf(0), Leaf(3)),
			split(SplitDown, Leaf(1), Leaf(2)),
		),
	}}

	if diff := cmp.Diff(want, layouts(windows)); diff != "" {
		t.Errorf("layouts() mismatch (-want +got):\n%s", diff)
	}
}

func TestLayoutsKeepEachWorkspaceApart(t *testing.T) {
	windows := []Window{
		tiled("a", 2, 1, 1, 638, 718),
		tiled("b", 1, 1, 1, 1278, 358),
		tiled("c", 2, 641, 1, 638, 718),
		tiled("d", 1, 1, 361, 1278, 358),
	}

	want := []Layout{
		{Workspace: 1, Root: split(SplitDown, Leaf(1), Leaf(3))},
		{Workspace: 2, Root: split(SplitRight, Leaf(0), Leaf(2))},
	}

	if diff := cmp.Diff(want, layouts(windows)); diff != "" {
		t.Errorf("layouts() mismatch (-want +got):\n%s", diff)
	}
}

func TestLayoutsGiveAGroupOneTile(t *testing.T) {
	group := func(class string, x, y, w, h int) Window {
		win := tiled(class, 1, x, y, w, h)
		win.Group = 1

		return win
	}

	windows := []Window{
		group("a", 1, 22, 426, 697),
		group("b", 1, 22, 426, 697),
		tiled("c", 1, 429, 1, 850, 718),
	}

	want := []Layout{{Workspace: 1, Root: split(SplitRight, Leaf(0), Leaf(2))}}

	if diff := cmp.Diff(want, layouts(windows)); diff != "" {
		t.Errorf("layouts() mismatch (-want +got):\n%s", diff)
	}
}

func TestLayoutsIgnoreFloatingWindows(t *testing.T) {
	floater := tiled("floater", 1, 100, 100, 600, 400)
	floater.Floating = true

	windows := []Window{
		tiled("a", 1, 1, 1, 638, 718),
		floater,
		tiled("b", 1, 641, 1, 638, 718),
	}

	want := []Layout{{Workspace: 1, Root: split(SplitRight, Leaf(0), Leaf(2))}}

	if diff := cmp.Diff(want, layouts(windows)); diff != "" {
		t.Errorf("layouts() mismatch (-want +got):\n%s", diff)
	}
}

func TestLayoutsGiveALoneWindowATreeOfOne(t *testing.T) {
	want := []Layout{{Workspace: 4, Root: Leaf(0)}}

	if diff := cmp.Diff(want, layouts([]Window{tiled("a", 4, 1, 1, 1278, 718)})); diff != "" {
		t.Errorf("layouts() mismatch (-want +got):\n%s", diff)
	}
}

func TestLayoutsDropAWorkspaceItCannotCut(t *testing.T) {
	windows := []Window{
		tiled("a", 1, 0, 0, 600, 600),
		tiled("b", 1, 300, 300, 600, 600),
	}

	if got := layouts(windows); got != nil {
		t.Errorf("layouts() = %v, want none", got)
	}
}

func TestSplitsComeParentsFirst(t *testing.T) {
	root := split(SplitRight,
		split(SplitDown, Leaf(0), Leaf(3)),
		split(SplitDown, Leaf(1), Leaf(2)),
	)

	want := []*Node{root, root.First, root.Second}
	if diff := cmp.Diff(want, root.SplitsParentsFirst()); diff != "" {
		t.Errorf("SplitsParentsFirst() mismatch (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff([]int{0, 3, 1, 2}, root.Leaves()); diff != "" {
		t.Errorf("Leaves() mismatch (-want +got):\n%s", diff)
	}

	if got := root.FirstLeaf(); got != 0 {
		t.Errorf("FirstLeaf() = %d, want 0", got)
	}
}
