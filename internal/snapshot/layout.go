package snapshot

import (
	"maps"
	"slices"
)

const (
	SplitRight = "right"
	SplitDown  = "down"
)

type Layout struct {
	Workspace int   `json:"workspace"`
	Root      *Node `json:"root"`
}

type Node struct {
	Window *int   `json:"window,omitzero"`
	Toward string `json:"toward,omitzero"`
	First  *Node  `json:"first,omitzero"`
	Second *Node  `json:"second,omitzero"`
}

func Leaf(window int) *Node {
	return &Node{Window: &window}
}

func (n *Node) IsLeaf() bool {
	return n != nil && n.Window != nil
}

func (n *Node) Leaves() []int {
	if n == nil {
		return nil
	}

	if n.IsLeaf() {
		return []int{*n.Window}
	}

	return append(n.First.Leaves(), n.Second.Leaves()...)
}

func (n *Node) SplitsParentsFirst() []*Node {
	if n == nil || n.IsLeaf() {
		return nil
	}

	return append(append([]*Node{n}, n.First.SplitsParentsFirst()...), n.Second.SplitsParentsFirst()...)
}

func (n *Node) FirstLeaf() int {
	for !n.IsLeaf() {
		n = n.First
	}

	return *n.Window
}

type tile struct {
	window int
	at     [2]int
	size   [2]int
}

func (t tile) start(axis int) int { return t.at[axis] }
func (t tile) end(axis int) int   { return t.at[axis] + t.size[axis] }

func layouts(windows []Window) []Layout {
	var order []int

	tiles := make(map[int][]tile)
	grouped := make(map[int]bool)

	for i, w := range windows {
		if w.Floating {
			continue
		}

		if w.Group > 0 {
			if grouped[w.Group] {
				continue
			}

			grouped[w.Group] = true
		}

		if _, seen := tiles[w.Workspace]; !seen {
			order = append(order, w.Workspace)
		}

		tiles[w.Workspace] = append(tiles[w.Workspace], tile{window: i, at: w.At, size: w.Size})
	}

	slices.Sort(order)

	var out []Layout

	for _, workspace := range order {
		root := partition(tiles[workspace])
		if root == nil {
			continue
		}

		out = append(out, Layout{Workspace: workspace, Root: root})
	}

	return out
}

func partition(tiles []tile) *Node {
	if len(tiles) == 0 {
		return nil
	}

	if len(tiles) == 1 {
		return Leaf(tiles[0].window)
	}

	for axis, toward := range []string{SplitRight, SplitDown} {
		first, second, ok := cut(tiles, axis)
		if !ok {
			continue
		}

		a, b := partition(first), partition(second)
		if a == nil || b == nil {
			return nil
		}

		return &Node{Toward: toward, First: a, Second: b}
	}

	return nil
}

func cut(tiles []tile, axis int) ([]tile, []tile, bool) {
	edges := make(map[int]bool, len(tiles))
	for _, t := range tiles {
		edges[t.end(axis)] = true
	}

	for _, edge := range slices.Sorted(maps.Keys(edges)) {
		var first, second []tile

		for _, t := range tiles {
			switch {
			case t.end(axis) <= edge:
				first = append(first, t)
			case t.start(axis) >= edge:
				second = append(second, t)
			}
		}

		if len(first) > 0 && len(second) > 0 && len(first)+len(second) == len(tiles) {
			return first, second, true
		}
	}

	return nil, nil, false
}
