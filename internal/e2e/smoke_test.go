package e2e

import (
	"slices"
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
)

func TestSaveCapturesWindow(t *testing.T) {
	t.Cleanup(func() { nested.CloseAllWindows(t) })

	hr := nested.CLI(t)

	nested.Spawn(t, "hrtest-a")
	hr.Run("save")

	snap := hr.Saved()

	i := slices.IndexFunc(snap.Windows, func(w snapshot.Window) bool {
		return w.Class == "hrtest-a"
	})
	if i < 0 {
		t.Fatalf("saved windows = %+v, want one of class hrtest-a", snap.Windows)
	}

	want := []string{"foot", "--app-id=hrtest-a", "-e", "sleep", "infinity"}
	if got := snap.Windows[i].Command; !slices.Equal(got, want) {
		t.Errorf("Command = %q, want %q", got, want)
	}
}
