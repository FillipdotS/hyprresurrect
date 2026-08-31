// Package restore turns a snapshot back into a running session.
package restore

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/FillipdotS/hyprresurrect/internal/util"
)

// A Step is one Lua statement to send, with a label for --dry-run output and
// for reporting which window failed.
type Step struct {
	What string
	Lua  string
}

type hyprland interface {
	Eval(lua string) error
	Clients() ([]hypr.Client, error)
}

// A Runner performs a restore: spawn every window, then move the ones the
// spawn rules could not place.
type Runner struct {
	Hypr   hyprland // may be nil, i.e. dry run
	Settle time.Duration
	Out    io.Writer // progress; nil is silent
	DryRun bool
}

// Run restores snap. Spawning and reconciling are one operation, not two: the
// rules attached to a spawn are best-effort, so a restore is only finished once
// the windows that ignored them have been moved into place.
//
// A step that fails is reported and skipped rather than aborting: one window
// that refuses to spawn must not cost the rest of the session.
func (r Runner) Run(snap snapshot.Snapshot) error {
	// Value receiver: this only normalises our own copy.
	if r.Out == nil {
		r.Out = io.Discard
	}

	steps := Plan(snap)
	if len(steps) == 0 {
		_, _ = fmt.Fprintf(r.Out, "nothing to restore\n")

		return nil
	}

	err := r.apply(steps)

	if r.DryRun {
		_, _ = fmt.Fprintf(r.Out, "\n-- then wait %s and move any window that missed its workspace:\n", r.Settle)
		for _, t := range targets(snap) {
			_, _ = fmt.Fprintf(r.Out, "   %s x%d -> workspace %d\n", t.class, t.count, t.workspace)
		}

		return err
	}

	_, _ = fmt.Fprintf(r.Out, "\nwaiting %s for windows to appear\n", r.Settle)
	if r.Settle > 0 {
		time.Sleep(r.Settle)
	}

	live, listErr := r.Hypr.Clients()
	if listErr != nil {
		return errors.Join(err, listErr)
	}

	moves := reconcile(live, snap)
	if len(moves) == 0 {
		_, _ = fmt.Fprintf(r.Out, "everything landed where it should\n")

		return err
	}

	return errors.Join(err, r.apply(moves))
}

func (r Runner) apply(steps []Step) error {
	var errs []error

	for _, step := range steps {
		if r.DryRun {
			_, _ = fmt.Fprintf(r.Out, "\n-- %s\n%s\n", step.What, step.Lua)

			continue
		}

		_, _ = fmt.Fprintf(r.Out, "%s\n", step.What)

		if err := r.Hypr.Eval(step.Lua); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", step.What, err))
		}
	}

	return errors.Join(errs...)
}

// Plan turns a snapshot into the ordered statements that spawn it: every
// workspace binding first, since a workspace rule only applies at creation
// time and the spawns are what create the workspaces, then one spawn per
// window.
func Plan(snap snapshot.Snapshot) []Step {
	windows := restorable(snap)

	steps := make([]Step, 0, len(windows))

	for _, b := range bindings(windows) {
		steps = append(steps, Step{
			What: fmt.Sprintf("bind workspace %d to %s", b.workspace, b.monitor),
			Lua: fmt.Sprintf("hl.workspace_rule({workspace = %s, monitor = %s})",
				luaString(strconv.Itoa(b.workspace)), luaString(b.monitor)),
		})
	}

	for _, w := range windows {
		steps = append(steps, Step{
			What: "spawn " + w.Class,
			Lua:  spawn(w),
		})
	}

	return steps
}

// restorable drops the windows whose command could not be read: there is
// nothing to relaunch, and nothing to match a live window against either.
func restorable(snap snapshot.Snapshot) []snapshot.Window {
	return util.List[snapshot.Window](snap.Windows).Filter(func(w snapshot.Window) bool {
		return len(w.Command) > 0
	})
}

type binding struct {
	workspace int
	monitor   string
}

// bindings returns the workspace-to-monitor pairs the windows need, deduped and
// in a stable order so that --dry-run output and tests don't shuffle.
func bindings(windows []snapshot.Window) []binding {
	seen := make(map[binding]struct{}, len(windows))

	var out []binding
	for _, w := range windows {
		if w.Monitor == "" {
			continue
		}

		b := binding{workspace: w.Workspace, monitor: w.Monitor}
		if _, dup := seen[b]; dup {
			continue
		}
		seen[b] = struct{}{}

		out = append(out, b)
	}

	slices.SortFunc(out, func(a, b binding) int {
		if c := cmp.Compare(a.workspace, b.workspace); c != 0 {
			return c
		}

		return strings.Compare(a.monitor, b.monitor)
	})

	return out
}

func spawn(w snapshot.Window) string {
	// "silent" puts the window on the workspace without making that workspace visible
	rules := []string{"workspace = " + luaString(strconv.Itoa(w.Workspace)+" silent")}

	if w.Floating {
		rules = append(rules,
			"float = true",
			"size = "+luaString(fmt.Sprintf("%d %d", w.Size[0], w.Size[1])),
			"move = "+luaString(fmt.Sprintf("%d %d", w.At[0], w.At[1])),
		)
	}

	rules = append(rules, "no_initial_focus = true")

	return fmt.Sprintf("hl.exec_cmd(%s, {%s})",
		luaString(shellCommand(w.Command)), strings.Join(rules, ", "))
}
