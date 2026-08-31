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
		Address:   "0x55d0768fdfd0",
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

func TestEval(t *testing.T) {
	fake := newFakeHypr(t, map[string]string{"/eval return 1+1": "ok"})

	if err := New(fake.SockPath).Eval("return 1+1"); err != nil {
		t.Fatalf("Eval() error = %v", err)
	}

	checkRequests(t, fake, "/eval return 1+1")
}

func TestDispatch(t *testing.T) {
	const expr = `hl.dsp.focus({window="address:0x1"})`

	fake := newFakeHypr(t, map[string]string{"/dispatch " + expr: "ok"})

	if err := New(fake.SockPath).Dispatch(expr); err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	checkRequests(t, fake, "/dispatch "+expr)
}

// Replies verified against hyprland 0.56.2.
func TestRequestErrors(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		wantErr bool
	}{
		{
			name:  "ok",
			reply: "ok",
		},
		{
			name:    "unknown request",
			reply:   "unknown request",
			wantErr: true,
		},
		{
			name:    "lua syntax error",
			reply:   `error: [string "this is not lua"]:1: syntax error near 'is'`,
			wantErr: true,
		},
		{
			// The statement ran but did nothing, which is a failed restore.
			name:    "warning is a failure",
			reply:   "warning: =[C]:-1: hl.focus: window not found",
			wantErr: true,
		},
		{
			name:  "only the prefix counts",
			reply: "ok: no error here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeHypr(t, map[string]string{"/eval x": tt.reply})

			err := New(fake.SockPath).Eval("x")
			if (err != nil) != tt.wantErr {
				t.Fatalf("Eval() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
