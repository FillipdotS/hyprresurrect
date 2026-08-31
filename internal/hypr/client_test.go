package hypr

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func serveFixture(t *testing.T, request, file string) *fakeHypr {
	t.Helper()

	payload, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	return newFakeHypr(t, map[string]string{request: string(payload)})
}

func checkRequests(t *testing.T, fake *fakeHypr, want ...string) {
	t.Helper()

	if diff := cmp.Diff(want, fake.Requests()); diff != "" {
		t.Errorf("requests mismatch (-want +got):\n%s", diff)
	}
}

func TestClients(t *testing.T) {
	fake := serveFixture(t, "[j]/clients", "clients.json")

	got, err := New(fake.SockPath).Clients()
	if err != nil {
		t.Fatalf("Clients() error = %v", err)
	}

	if len(got) != 11 {
		t.Fatalf("Clients() returned %d clients, want 11", len(got))
	}

	want := Client{
		Class:     "foot",
		Workspace: WorkspaceRef{ID: 3},
		MonitorID: 0,
		At:        [2]int{3628, 30},
		Size:      [2]int{590, 516},
		Floating:  false,
		PID:       2035226,
	}
	if diff := cmp.Diff(want, got[0]); diff != "" {
		t.Errorf("Clients()[0] mismatch (-want +got):\n%s", diff)
	}

	checkRequests(t, fake, "[j]/clients")
}

func TestMonitors(t *testing.T) {
	fake := serveFixture(t, "[j]/monitors", "monitors.json")

	got, err := New(fake.SockPath).Monitors()
	if err != nil {
		t.Fatalf("Monitors() error = %v", err)
	}

	want := []Monitor{
		{ID: 0, Name: "HDMI-A-1", ActiveWorkspace: WorkspaceRef{ID: 3}},
		{ID: 1, Name: "DP-1", ActiveWorkspace: WorkspaceRef{ID: 5}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Monitors() mismatch (-want +got):\n%s", diff)
	}

	checkRequests(t, fake, "[j]/monitors")
}
