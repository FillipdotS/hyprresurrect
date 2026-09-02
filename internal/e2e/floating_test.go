package e2e

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFloatingWindowKeepsItsGeometry(t *testing.T) {
	hr := setup(t)

	want := []string{"hrtest-float floating 600x400 at 120,90 on ws1"}

	// A rule rather than a dispatch: the window has to be floating and placed
	// by the time it first maps.
	nested.Eval(t, `hl.window_rule({match = {class = "hrtest-float"}, `+
		`float = true, size = "600 400", move = "120 90"})`)

	nested.FocusWorkspace(t, 1)
	nested.Spawn(t, "hrtest-float")

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Fatalf("the window rule did not place the window (-want +got):\n%s", diff)
	}

	hr.Run("save")
	nested.CloseAllWindows(t)
	hr.Run("restore")

	if diff := cmp.Diff(want, summarize(t, geometry)); diff != "" {
		t.Errorf("floating geometry did not survive the round trip (-want +got):\n%s", diff)
	}
}

func geometry(c client) string {
	kind := "tiled"
	if c.Floating {
		kind = "floating"
	}

	return fmt.Sprintf("%s %s %dx%d at %d,%d on ws%d",
		c.Class, kind, c.Size[0], c.Size[1], c.At[0], c.At[1], c.Workspace.ID)
}
