package e2e

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWindowsReturnToTheirMonitor(t *testing.T) {
	hr := setup(t)

	// The nested instance starts with one output, so the headless one added
	// here is monitor 1.
	want := []string{"hrtest-mon-a on mon0", "hrtest-mon-b on mon1"}

	primary, headless := nested.Monitor(t, 0), nested.AddHeadlessMonitor(t)

	nested.FocusMonitor(t, primary.Name)
	nested.Spawn(t, "hrtest-mon-a")

	nested.FocusMonitor(t, headless.Name)
	nested.Spawn(t, "hrtest-mon-b")

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, summarize(t, onMonitor)); diff != "" {
		t.Errorf("windows did not come back on their monitors (-want +got):\n%s", diff)
	}
}

func onMonitor(c client) string {
	return fmt.Sprintf("%s on mon%d", c.Class, c.Monitor)
}
