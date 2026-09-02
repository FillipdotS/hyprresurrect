package e2e

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
)

// binary is the real CLI under test, built once by TestMain
var binary string

func build(dir string) (string, error) {
	bin := filepath.Join(dir, "hyprresurrect")

	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "../.."

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("go build: %w\n%s", err, out)
	}

	return bin, nil
}

type cli struct {
	t     *testing.T
	state string
	env   []string
}

func (c *compositor) CLI(t *testing.T) cli {
	t.Helper()

	state := t.TempDir()

	return cli{
		t:     t,
		state: state,
		env: baseEnv(c.dir,
			"HYPRLAND_INSTANCE_SIGNATURE="+c.signature(),
			"XDG_STATE_HOME="+state,
		),
	}
}

// Run fails the test unless the command exits cleanly.
func (h cli) Run(args ...string) string {
	h.t.Helper()

	cmd := exec.Command(binary, args...)
	cmd.Env = h.env

	out, err := cmd.CombinedOutput()
	if err != nil {
		h.t.Fatalf("hyprresurrect %s: %v\n%s", strings.Join(args, " "), err, out)
	}

	pause()

	return string(out)
}

// Saved reads back the newest snapshot the CLI wrote, off disk.
func (h cli) Saved() snapshot.Snapshot {
	h.t.Helper()

	// The store's own directory name, which snapshot keeps unexported.
	store := snapshot.NewAt(filepath.Join(h.state, "hyprresurrect"))

	entries, err := store.List()
	if err != nil {
		h.t.Fatalf("list snapshots: %v", err)
	}

	if len(entries) == 0 {
		h.t.Fatalf("no snapshots under %s", h.state)
	}

	snap, err := store.Load(entries[0].Path)
	if err != nil {
		h.t.Fatalf("load %s: %v", entries[0].Path, err)
	}

	return snap
}
