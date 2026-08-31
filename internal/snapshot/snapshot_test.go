package snapshot

import (
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
