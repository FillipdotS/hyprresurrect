package snapshot

import (
	"fmt"
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/google/go-cmp/cmp"
)

type fakeSource struct {
	clients     []hypr.Client
	monitors    []hypr.Monitor
	clientsErr  error
	monitorsErr error
}

func (f fakeSource) Clients() ([]hypr.Client, error)   { return f.clients, f.clientsErr }
func (f fakeSource) Monitors() ([]hypr.Monitor, error) { return f.monitors, f.monitorsErr }

func TestCapture(t *testing.T) {
	const uid = 1000

	proc := newFakeProc(t)
	proc.addProcess(t, 1000, 1, uid, "foot", "-e", "cliamp")
	proc.addProcess(t, 2000, 1, uid, "/usr/bin/ghostty")

	src := fakeSource{
		monitors: []hypr.Monitor{
			{ID: 0, Name: "HDMI-A-1", ActiveWorkspace: hypr.WorkspaceRef{ID: 3}},
			{ID: 1, Name: "DP-1", ActiveWorkspace: hypr.WorkspaceRef{ID: 5}},
		},
		clients: []hypr.Client{
			{
				Class:     "foot",
				Workspace: hypr.WorkspaceRef{ID: 3},
				MonitorID: 0,
				At:        [2]int{3628, 30},
				Size:      [2]int{590, 516},
				PID:       1000,
			},
			{
				Class:     "com.mitchellh.ghostty",
				Workspace: hypr.WorkspaceRef{ID: 5},
				MonitorID: 1,
				At:        [2]int{4, 28},
				Size:      [2]int{713, 629},
				Floating:  true,
				PID:       2000,
			},
			{
				// Process already gone: nothing to relaunch, so it's dropped.
				Class:     "Aseprite",
				Workspace: hypr.WorkspaceRef{ID: 2},
				MonitorID: 1,
				PID:       9999,
			},
		},
	}

	got, err := capture(src, proc.root)
	if err != nil {
		t.Fatalf("capture() error = %v", err)
	}

	if got.CapturedAt.IsZero() {
		t.Error("CapturedAt is zero, want the time of capture")
	}

	wantMonitors := []Monitor{
		{Name: "HDMI-A-1", ActiveWorkspace: 3},
		{Name: "DP-1", ActiveWorkspace: 5},
	}
	if diff := cmp.Diff(wantMonitors, got.Monitors); diff != "" {
		t.Errorf("Monitors mismatch (-want +got):\n%s", diff)
	}

	// Monitor ids are resolved to names, and the dead pid is gone.
	wantWindows := []Window{
		{
			Class:     "foot",
			Workspace: 3,
			Monitor:   "HDMI-A-1",
			At:        [2]int{3628, 30},
			Size:      [2]int{590, 516},
			Command:   []string{"foot", "-e", "cliamp"},
		},
		{
			Class:     "com.mitchellh.ghostty",
			Workspace: 5,
			Monitor:   "DP-1",
			At:        [2]int{4, 28},
			Size:      [2]int{713, 629},
			Floating:  true,
			Command:   []string{"/usr/bin/ghostty"},
		},
	}
	if diff := cmp.Diff(wantWindows, got.Windows); diff != "" {
		t.Errorf("Windows mismatch (-want +got):\n%s", diff)
	}
}

// Hyprland lists a group's members wherever they fall in the client list, and
// only the `grouped` array says which order the tabs are in. Capture has to put
// them back together, in that order, with the raised tab marked.
func TestCaptureGroups(t *testing.T) {
	const uid = 1000

	proc := newFakeProc(t)
	for pid, class := range map[int]string{1000: "alpha", 2000: "beta", 3000: "gamma", 4000: "delta"} {
		proc.addProcess(t, pid, 1, uid, class)
	}

	grouped := []string{"0xb", "0xa"} // beta is the first tab

	src := fakeSource{
		monitors: []hypr.Monitor{{ID: 0, Name: "DP-1", ActiveWorkspace: hypr.WorkspaceRef{ID: 1}}},
		clients: []hypr.Client{
			{Class: "alpha", Address: "0xa", PID: 1000, Grouped: grouped},
			{Class: "gamma", Address: "0xc", PID: 3000, Visible: true},
			{Class: "beta", Address: "0xb", PID: 2000, Grouped: grouped, Visible: true},
			{Class: "delta", Address: "0xd", PID: 4000, Grouped: []string{"0xd"}, Visible: true},
		},
	}

	got, err := capture(src, proc.root)
	if err != nil {
		t.Fatalf("capture() error = %v", err)
	}

	// beta before alpha: tab order, not client order. gamma is ungrouped and
	// keeps its place, and its Visible means nothing outside a group.
	want := []string{
		"beta group1 active",
		"alpha group1",
		"gamma",
		"delta group2 active",
	}

	if diff := cmp.Diff(want, describe(got.Windows)); diff != "" {
		t.Errorf("groups mismatch (-want +got):\n%s", diff)
	}
}

func describe(windows []Window) []string {
	out := make([]string, 0, len(windows))

	for _, w := range windows {
		s := w.Class

		if w.Group > 0 {
			s += fmt.Sprintf(" group%d", w.Group)
		}
		if w.GroupActive {
			s += " active"
		}

		out = append(out, s)
	}

	return out
}
