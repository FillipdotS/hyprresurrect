package e2e

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Two windows of one class, separable only by command: on class alone reconcile
// could claim them the wrong way round and swap their workspaces. The title is
// part of the command, so hyprctl shows which one came back where.
func TestSameClassDifferentCommands(t *testing.T) {
	hr := setup(t)

	want := []string{"hrtest-dup/alpha@ws1", "hrtest-dup/beta@ws3"}

	nested.FocusWorkspace(t, 1)
	nested.SpawnTitled(t, "hrtest-dup", "alpha")

	nested.FocusWorkspace(t, 3)
	nested.SpawnTitled(t, "hrtest-dup", "beta")

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, summarize(t, titled)); diff != "" {
		t.Errorf("windows of one class were not told apart (-want +got):\n%s", diff)
	}
}

func titled(c client) string {
	return fmt.Sprintf("%s/%s@ws%d", c.Class, c.Title, c.Workspace.ID)
}
