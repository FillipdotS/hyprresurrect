package e2e

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

type client struct {
	Address   string   `json:"address"`
	Grouped   []string `json:"grouped"`
	Class     string   `json:"class"`
	Title     string   `json:"title"`
	Monitor   int      `json:"monitor"`
	Floating  bool     `json:"floating"`
	Visible   bool     `json:"visible"`
	At        [2]int   `json:"at"`
	Size      [2]int   `json:"size"`
	Workspace struct {
		ID int `json:"id"`
	} `json:"workspace"`
}

type monitor struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func hyprctlClients(t *testing.T) []client {
	t.Helper()

	return decode[client](t, "clients")
}

func hyprctlMonitors(t *testing.T) []monitor {
	t.Helper()

	return decode[monitor](t, "monitors")
}

func decode[T any](t *testing.T, what string) []T {
	t.Helper()

	out := nested.Hyprctl(t, "-j", what)

	var list []T
	if err := json.Unmarshal([]byte(out), &list); err != nil {
		t.Fatalf("decoding hyprctl %s: %v\n%s", what, err, out)
	}

	return list
}

func summarize(t *testing.T, describe func(client) string) []string {
	t.Helper()

	live := hyprctlClients(t)

	windows := make([]string, 0, len(live))
	for _, c := range live {
		windows = append(windows, describe(c))
	}

	slices.Sort(windows)

	return windows
}

// placed is every window as "class@wsN".
func placed(t *testing.T) []string {
	return summarize(t, func(c client) string {
		return fmt.Sprintf("%s@ws%d", c.Class, c.Workspace.ID)
	})
}
