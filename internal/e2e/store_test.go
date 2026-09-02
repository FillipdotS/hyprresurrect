package e2e

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRestoreUsesTheNewestSnapshot(t *testing.T) {
	hr := setup(t)

	want := []string{"hrtest-new@ws1"}

	nested.FocusWorkspace(t, 1)

	nested.Spawn(t, "hrtest-old")
	hr.Run("save")
	nested.CloseAllWindows(t)

	nested.Spawn(t, "hrtest-new")
	hr.Run("save")
	nested.CloseAllWindows(t)

	hr.Run("restore")

	if diff := cmp.Diff(want, placed(t)); diff != "" {
		t.Errorf("restore did not replay the newest snapshot (-want +got):\n%s", diff)
	}
}

func TestSaveKeepsOnlyTheNewestSnapshots(t *testing.T) {
	hr := setup(t)

	nested.Spawn(t, "hrtest-prune")

	for range 7 {
		hr.Run("save")
	}

	if got := hr.Snapshots(t); len(got) != 5 {
		t.Errorf("snapshots on disk = %d, want 5:\n%v", len(got), got)
	}
}

func (h cli) Snapshots(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(h.state, "hyprresurrect", "*.json"))
	if err != nil {
		t.Fatalf("listing snapshots: %v", err)
	}

	return paths
}
