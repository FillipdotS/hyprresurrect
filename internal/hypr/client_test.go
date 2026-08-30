package hypr

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestClients(t *testing.T) {
	payload, err := os.ReadFile("testdata/clients.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	fake := newFakeHypr(t, map[string]string{"[j]/clients": string(payload)})

	got, err := New(fake.SockPath).Clients()
	if err != nil {
		t.Fatalf("Clients() error = %v", err)
	}

	if len(got) != 11 {
		t.Fatalf("Clients() returned %d clients, want 11", len(got))
	}

	want := Client{
		Address:      "0x55d0768fdfd0",
		Class:        "foot",
		InitialClass: "foot",
		Title:        "becoming you [slow lofi] - snoozy beats | cliamp",
		InitialTitle: "foot",
		Workspace:    Workspace{ID: 3, Name: "3"},
		MonitorID:    0,
		At:           [2]int{3628, 30},
		Size:         [2]int{590, 516},
		Floating:     false,
		PID:          2035226,
		StableID:     "18000c28",
	}
	if diff := cmp.Diff(want, got[0]); diff != "" {
		t.Errorf("Clients()[0] mismatch (-want +got):\n%s", diff)
	}

	wantReqs := []string{"[j]/clients"}
	if diff := cmp.Diff(wantReqs, fake.Requests()); diff != "" {
		t.Errorf("requests mismatch (-want +got):\n%s", diff)
	}
}
