package e2e

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/google/go-cmp/cmp"
)

func TestGroupedWindowsComeBackGrouped(t *testing.T) {
	hr := setup(t)

	want := []string{"hrtest-group-a+hrtest-group-b*@ws1"}

	nested.FocusWorkspace(t, 1)
	nested.Spawn(t, "hrtest-group-a")

	// The first window becomes a group of one; auto_group then folds the next
	// window to map on top of it into that group.
	nested.Eval(t, `hl.dispatch(hl.dsp.group.toggle())`)
	nested.Spawn(t, "hrtest-group-b")

	if diff := cmp.Diff(want, grouping(t)); diff != "" {
		t.Fatalf("the windows were not grouped to begin with (-want +got):\n%s", diff)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, grouping(t)); diff != "" {
		t.Errorf("the group did not survive the round trip (-want +got):\n%s", diff)
	}
}

func TestTwoGroupsKeepTheirTabsApart(t *testing.T) {
	hr := setup(t)

	want := []string{
		"hrtest-g1a+hrtest-g1b*@ws2",
		"hrtest-g2a*+hrtest-g2b@ws2",
	}

	nested.FocusWorkspace(t, 2)

	g1a := nested.Spawn(t, "hrtest-g1a")
	g1b := nested.Spawn(t, "hrtest-g1b")
	nested.Spawn(t, "hrtest-loner")
	g2a := nested.Spawn(t, "hrtest-g2a")
	g2b := nested.Spawn(t, "hrtest-g2b")

	tabTogether(t, g1a, g1b)
	tabTogether(t, g2a, g2b)
	raiseTab(t, g2a, 1)

	if diff := cmp.Diff(want, grouping(t)); diff != "" {
		t.Fatalf("the groups were not built as the test meant them (-want +got):\n%s", diff)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, grouping(t)); diff != "" {
		t.Errorf("the groups did not survive the round trip (-want +got):\n%s", diff)
	}

	if want, got := 5, len(hyprctlClients(t)); want != got {
		t.Errorf("%d windows came back, want %d", got, want)
	}
}

func tabTogether(t *testing.T, head hypr.Client, members ...hypr.Client) {
	t.Helper()

	nested.Eval(t, fmt.Sprintf(`hl.dispatch(hl.dsp.group.toggle({window = %q}))`, "address:"+head.Address))

	for _, m := range members {
		nested.Eval(t, fmt.Sprintf(`hl.get_window(%q).group:add(hl.get_window(%q))`,
			"address:"+head.Address, "address:"+m.Address))
	}
}

func raiseTab(t *testing.T, member hypr.Client, index int) {
	t.Helper()

	nested.Eval(t, fmt.Sprintf(`hl.dispatch(hl.dsp.group.active({window = %q, index = %d}))`,
		"address:"+member.Address, index))
}

// grouping describes every group in the session as "a+b@wsN": its members by
// class, in tab order, with a star on the raised one. Ungrouped windows are
// left out. Classes rather than addresses, because an address does not survive
// the window being closed and respawned.
func grouping(t *testing.T) []string {
	t.Helper()

	live := hyprctlClients(t)

	byAddress := make(map[string]client, len(live))
	for _, c := range live {
		byAddress[c.Address] = c
	}

	var (
		groups []string
		seen   = make(map[string]bool)
	)

	for _, c := range live {
		if len(c.Grouped) == 0 || seen[c.Address] {
			continue
		}

		members := make([]string, 0, len(c.Grouped))

		for _, address := range c.Grouped {
			seen[address] = true

			member := byAddress[address]

			tab := member.Class
			if member.Visible {
				tab += "*"
			}

			members = append(members, tab)
		}

		groups = append(groups, fmt.Sprintf("%s@ws%d", strings.Join(members, "+"), c.Workspace.ID))
	}

	slices.Sort(groups)

	return groups
}
