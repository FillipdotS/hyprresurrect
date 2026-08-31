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
	Hypr   hyprland  // may be nil, i.e. dry run
	Out    io.Writer // progress; nil is silent
	DryRun bool

	timeout time.Duration
	poll    time.Duration
	command func(pid int) ([]string, error)
}

const (
	defaultTimeout = 10 * time.Second
	defaultPoll    = 250 * time.Millisecond
)

// Run restores given snapshot as best it can.
func (r Runner) Run(snap snapshot.Snapshot) error {
	// Value receiver: this only normalises our own copy.
	if r.Out == nil {
		r.Out = io.Discard
	}
	r.timeout = cmp.Or(r.timeout, defaultTimeout)

	steps := Plan(snap)
	if len(steps) == 0 {
		_, _ = fmt.Fprintf(r.Out, "nothing to restore\n")

		return nil
	}

	if r.DryRun {
		err := r.apply(steps)

		_, _ = fmt.Fprintf(r.Out, "\n-- then wait up to %s and move any window that missed its workspace:\n", r.timeout)
		for _, t := range targets(snap) {
			_, _ = fmt.Fprintf(r.Out, "   %s x%d -> workspace %d\n", t.class, t.count, t.workspace)
		}

		return err
	}

	before, err := r.Hypr.Clients()
	if err != nil {
		_, _ = fmt.Fprintf(r.Out, "warning: could not list windows before spawning: %v\n", err)
	}

	spawnErr := r.apply(steps)

	live, listErr := r.settle(snap, before)
	if listErr != nil {
		return errors.Join(spawnErr, listErr)
	}

	moves := reconcile(r.resolve(live), snap)
	if len(moves) == 0 {
		_, _ = fmt.Fprintf(r.Out, "everything landed where it should\n")

		return spawnErr
	}

	return errors.Join(spawnErr, r.apply(moves))
}

// resolve tries to read back the command behind every live window
func (r Runner) resolve(clients []hypr.Client) []liveWindow {
	commandOf := r.command
	if commandOf == nil {
		commandOf = snapshot.Command
	}

	live := make([]liveWindow, len(clients))
	for i, c := range clients {
		live[i] = liveWindow{Client: c}

		if argv, err := commandOf(c.PID); err == nil {
			live[i].Command = argv
		}
	}

	return live
}

// settle waits for the spawned windows to exist
func (r Runner) settle(snap snapshot.Snapshot, existing []hypr.Client) ([]hypr.Client, error) {
	_, _ = fmt.Fprintf(r.Out, "\nwaiting up to %s for windows to appear\n", r.timeout)

	want := windowClasses(snap.Windows)
	base := clientClasses(existing)
	deadline := time.Now().Add(r.timeout)

	for {
		live, err := r.Hypr.Clients()
		if err != nil {
			return nil, err
		}

		if allAppeared(live, base, want) || !time.Now().Before(deadline) {
			return live, nil
		}

		time.Sleep(cmp.Or(r.poll, defaultPoll))
	}
}

// allAppeared reports whether every class has as many windows as the snapshot
// expects, over and above the ones that were already open.
func allAppeared(live []hypr.Client, existing, want map[string]int) bool {
	have := clientClasses(live)

	for class, n := range want {
		if have[class]-existing[class] < n {
			return false
		}
	}

	return true
}

func clientClasses(clients []hypr.Client) map[string]int {
	counts := make(map[string]int, len(clients))
	for _, c := range clients {
		counts[c.Class]++
	}

	return counts
}

func windowClasses(windows []snapshot.Window) map[string]int {
	counts := make(map[string]int, len(windows))
	for _, w := range windows {
		counts[w.Class]++
	}

	return counts
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
	windows := snap.Windows

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
