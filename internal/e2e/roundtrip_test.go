package e2e

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRoundTrip(t *testing.T) {
	hr := setup(t)

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
