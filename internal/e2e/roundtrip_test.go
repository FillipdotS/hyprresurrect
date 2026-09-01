package e2e

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRoundTrip(t *testing.T) {
	t.Cleanup(func() { nested.CloseAllWindows(t) })

	hr := nested.CLI(t)

	session := []struct {
		class     string
		workspace int
	}{
		{"hrtest-a", 1},
		{"hrtest-b", 2},
		{"hrtest-c", 2},
	}

	want := []string{"hrtest-a@ws1", "hrtest-b@ws2", "hrtest-c@ws2"}

	for _, w := range session {
		nested.FocusWorkspace(t, w.workspace)
		nested.Spawn(t, w.class)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, placed(t)); diff != "" {
		t.Errorf("session did not survive the round trip (-want +got):\n%s", diff)
	}
}

// placed is every window hyprctl reports, as "class@wsN".
func placed(t *testing.T) []string {
	t.Helper()

	var clients []struct {
		Class     string `json:"class"`
		Workspace struct {
			ID int `json:"id"`
		} `json:"workspace"`
	}

	out := nested.Hyprctl(t, "-j", "clients")
	if err := json.Unmarshal([]byte(out), &clients); err != nil {
		t.Fatalf("decoding hyprctl clients: %v\n%s", err, out)
	}

	windows := make([]string, 0, len(clients))
	for _, c := range clients {
		windows = append(windows, fmt.Sprintf("%s@ws%d", c.Class, c.Workspace.ID))
	}

	slices.Sort(windows)

	return windows
}
