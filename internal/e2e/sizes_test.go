package e2e

import (
	"fmt"
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/google/go-cmp/cmp"
)

func TestTileSizesSurviveTheRoundTrip(t *testing.T) {
	hr := setup(t)

	want := []string{"hrtest-size-a 426x718", "hrtest-size-b 850x718"}

	nested.FocusWorkspace(t, 1)
	a := nested.Spawn(t, "hrtest-size-a")
	nested.Spawn(t, "hrtest-size-b")

	resizeExact(t, a, 426, 720)

	if diff := cmp.Diff(want, summarize(t, sized)); diff != "" {
		t.Fatalf("the resize did not split the screen as the test meant it (-want +got):\n%s", diff)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, summarize(t, sized)); diff != "" {
		t.Errorf("tile sizes did not survive the round trip (-want +got):\n%s", diff)
	}
}

// A 2x2 of tiles with both splits pulled off centre
func TestManyTilesKeepTheirLayout(t *testing.T) {
	hr := setup(t)

	want := []string{
		"hrtest-many-a tiled 426x240 at 1,1 on ws1",
		"hrtest-many-b tiled 850x240 at 429,1 on ws1",
		"hrtest-many-c tiled 850x476 at 429,243 on ws1",
		"hrtest-many-d tiled 426x476 at 1,243 on ws1",
	}

	nested.FocusWorkspace(t, 1)

	a := nested.Spawn(t, "hrtest-many-a")
	b := nested.Spawn(t, "hrtest-many-b")

	focusWindow(t, b)
	nested.Spawn(t, "hrtest-many-c")

	focusWindow(t, a)
	nested.Spawn(t, "hrtest-many-d")

	resizeExact(t, a, 426, 240)
	resizeExact(t, b, 850, 240)

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Fatalf("the tiles were not laid out as the test meant them (-want +got):\n%s", diff)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Errorf("the layout did not survive the round trip (-want +got):\n%s", diff)
	}
}

// One of the two tiles is a group
func TestGroupedTileKeepsItsSize(t *testing.T) {
	hr := setup(t)

	want := []string{
		"hrtest-gt-a tiled 426x697 at 1,22 on ws1",
		"hrtest-gt-b tiled 426x697 at 1,22 on ws1",
		"hrtest-gt-c tiled 850x718 at 429,1 on ws1",
	}
	wantGroups := []string{"hrtest-gt-a+hrtest-gt-b*@ws1"}

	nested.FocusWorkspace(t, 1)

	a := nested.Spawn(t, "hrtest-gt-a")
	b := nested.Spawn(t, "hrtest-gt-b")
	nested.Spawn(t, "hrtest-gt-c")

	// b's own tile goes back to c when b joins the group, leaving the group
	// and c to split the screen.
	tabTogether(t, a, b)
	resizeExact(t, a, 426, 720)

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Fatalf("the group was not sized as the test meant it (-want +got):\n%s", diff)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Errorf("the grouped tile did not survive the round trip (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(wantGroups, grouping(t)); diff != "" {
		t.Errorf("the group itself did not survive the round trip (-want +got):\n%s", diff)
	}
}

// A group across two thirds of the screen, with two windows stacked in the
// remaining third and their own split off centre.
func TestBigGroupBesideTwoStackedTiles(t *testing.T) {
	hr := setup(t)

	want := []string{
		"hrtest-bg-1 tiled 857x697 at 1,22 on ws1",
		"hrtest-bg-2 tiled 857x697 at 1,22 on ws1",
		"hrtest-bg-s1 tiled 419x240 at 860,1 on ws1",
		"hrtest-bg-s2 tiled 419x476 at 860,243 on ws1",
	}
	wantGroups := []string{"hrtest-bg-1+hrtest-bg-2*@ws1"}

	nested.FocusWorkspace(t, 1)

	one := nested.Spawn(t, "hrtest-bg-1")
	s1 := nested.Spawn(t, "hrtest-bg-s1")
	nested.Spawn(t, "hrtest-bg-s2")
	two := nested.Spawn(t, "hrtest-bg-2")

	tabTogether(t, one, two)

	resizeExact(t, one, 853, 720)
	resizeExact(t, s1, 427, 240)

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Fatalf("the layout was not built as the test meant it (-want +got):\n%s", diff)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Errorf("the layout did not survive the round trip (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(wantGroups, grouping(t)); diff != "" {
		t.Errorf("the group did not survive the round trip (-want +got):\n%s", diff)
	}
}

// Two workspaces laid out differently, with a window of the same class and
// command on each: the layouts have to come back per workspace, and the two
// interchangeable windows cannot borrow each other's geometry.
func TestWorkspacesKeepTheirOwnLayouts(t *testing.T) {
	hr := setup(t)

	want := []string{
		"hrtest-ws-a tiled 850x718 at 429,1 on ws1",
		"hrtest-ws-b tiled 419x240 at 860,1 on ws2",
		"hrtest-ws-c tiled 419x476 at 860,243 on ws2",
		"hrtest-ws-dup tiled 426x718 at 1,1 on ws1",
		"hrtest-ws-dup tiled 857x718 at 1,1 on ws2",
	}

	nested.FocusWorkspace(t, 1)
	first := nested.Spawn(t, "hrtest-ws-dup")
	nested.Spawn(t, "hrtest-ws-a")
	resizeExact(t, first, 426, 720)

	nested.FocusWorkspace(t, 2)
	second := nested.Spawn(t, "hrtest-ws-dup")
	b := nested.Spawn(t, "hrtest-ws-b")
	nested.Spawn(t, "hrtest-ws-c")
	resizeExact(t, second, 853, 720)
	resizeExact(t, b, 427, 240)

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Fatalf("the workspaces were not laid out as the test meant them (-want +got):\n%s", diff)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Errorf("the layouts did not survive the round trip (-want +got):\n%s", diff)
	}
}

func sized(c client) string {
	return fmt.Sprintf("%s %dx%d", c.Class, c.Size[0], c.Size[1])
}

func focusWindow(t *testing.T, c hypr.Client) {
	t.Helper()

	nested.Eval(t, fmt.Sprintf(`hl.dispatch(hl.dsp.focus({window = %q}))`, "address:"+c.Address))
}

// resizeExact sets the window's size in pixels; dwindle turns that into the
// split ratios around it, so the sibling tiles move too.
func resizeExact(t *testing.T, c hypr.Client, w, h int) {
	t.Helper()

	nested.Eval(t, fmt.Sprintf(`hl.dispatch(hl.dsp.window.resize({window = %q, x = %d, y = %d, exact = true}))`,
		"address:"+c.Address, w, h))
}
