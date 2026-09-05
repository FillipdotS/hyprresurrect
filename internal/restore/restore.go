// Package restore turns a snapshot back into a running session.
package restore

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/FillipdotS/hyprresurrect/internal/apps"
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
	details func(clients []hypr.Client) map[string]apps.Detail
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

		if tabs := groupTargets(snap); len(tabs) > 0 {
			_, _ = fmt.Fprintf(r.Out, "\n-- and rebuild the groups:\n")

			for _, t := range tabs {
				_, _ = fmt.Fprintf(r.Out, "   %s on workspace %d\n", strings.Join(t.classes, " + "), t.workspace)
			}
		}

		if tiles := layoutTargets(snap); len(tiles) > 0 {
			_, _ = fmt.Fprintf(r.Out, "\n-- and rebuild the tile layout:\n")

			for _, t := range tiles {
				_, _ = fmt.Fprintf(r.Out, "   workspace %d: %s\n", t.workspace, t.tree)
			}
		}

		if running := runningTargets(snap); len(running) > 0 {
			_, _ = fmt.Fprintf(r.Out, "\n-- these were running, and are not started again:\n")

			for _, t := range running {
				_, _ = fmt.Fprintf(r.Out, "   workspace %d: %s\n", t.workspace, t.program)
			}
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

	resolved := r.settleDetails(live)
	claimed := claim(resolved, snap)

	// Groups only after the moves: members have to share a workspace first.
	placement := append(moves(resolved, snap, claimed), regroup(resolved, snap, claimed)...)
	if len(placement) == 0 {
		_, _ = fmt.Fprintf(r.Out, "everything landed where it should\n")
	}

	placeErr := r.apply(placement)

	settled, listErr := r.Hypr.Clients()
	if listErr != nil {
		return errors.Join(spawnErr, placeErr, listErr)
	}

	tiled := refresh(resolved, settled)

	steps = append(relayout(tiled, snap, claimed), raiseTabs(tiled, snap, claimed)...)

	return errors.Join(spawnErr, placeErr, r.apply(steps))
}

func refresh(windows []liveWindow, clients []hypr.Client) []liveWindow {
	byAddress := make(map[string]hypr.Client, len(clients))
	for _, c := range clients {
		byAddress[c.Address] = c
	}

	updated := slices.Clone(windows)
	for i, w := range updated {
		if c, ok := byAddress[w.Address]; ok {
			updated[i].Client = c
		}
	}

	return updated
}

// settleDetails resolves the live windows, waiting for the ones an app can look
// into to become readable.
//
// A window maps before the process behind it has finished with it: a terminal's
// shell is forked a moment later, and until it is there is no directory to
// read. That directory is the only thing separating the windows of a single
// instance, which report one identical argv between them, so the move pass
// would otherwise place them by class alone and scatter them.
func (r Runner) settleDetails(clients []hypr.Client) []liveWindow {
	deadline := time.Now().Add(r.timeout)

	for {
		resolved := r.resolve(clients)

		if readable(resolved) || !time.Now().Before(deadline) {
			return resolved
		}

		time.Sleep(cmp.Or(r.poll, defaultPoll))
	}
}

// readable reports whether every window an app owns has given up its detail.
func readable(live []liveWindow) bool {
	for _, w := range live {
		if apps.Owns(w.Class) && w.Cwd == "" {
			return false
		}
	}

	return true
}

// resolve tries to read back the command behind every live window, and what is
// going on inside the ones an app can look into.
func (r Runner) resolve(clients []hypr.Client) []liveWindow {
	commandOf := r.command
	if commandOf == nil {
		commandOf = snapshot.Command
	}

	detailsOf := r.details
	if detailsOf == nil {
		detailsOf = func(clients []hypr.Client) map[string]apps.Detail {
			return apps.Inspect(clients, "/proc")
		}
	}

	details := detailsOf(clients)

	live := make([]liveWindow, len(clients))
	for i, c := range clients {
		live[i] = liveWindow{Client: c, Detail: details[c.Address]}

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

		if allAppeared(live, base, want) {
			return live, nil
		}

		if !time.Now().Before(deadline) {
			r.reportMissing(live, base, want)

			return live, nil
		}

		time.Sleep(cmp.Or(r.poll, defaultPoll))
	}
}

// reportMissing names the windows that never turned up. Saying nothing would
// leave a restore that quietly did nothing looking exactly like one that
// worked: an app can accept a request to open a window and simply not, and
// what follows here only ever works with the windows that did appear.
func (r Runner) reportMissing(live []hypr.Client, existing, want map[string]int) {
	have := clientClasses(live)

	for _, class := range slices.Sorted(maps.Keys(want)) {
		if short := want[class] - (have[class] - existing[class]); short > 0 {
			_, _ = fmt.Fprintf(r.Out, "warning: %d %s window(s) never appeared\n", short, class)
		}
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
				luaString(fmt.Sprintf("%d", b.workspace)), luaString(b.monitor)),
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
	// The apps we can look inside build their own command line: a terminal
	// reopens in the directory it was in, with what was running left at the
	// prompt for the user to accept.
	argv := apps.Launch(w.Class, w.Command, apps.Detail{Cwd: w.Cwd, Program: w.Program})

	// "silent" puts the window on the workspace without making that workspace visible
	rules := []string{"workspace = " + luaString(fmt.Sprintf("%d silent", w.Workspace))}

	if w.Floating {
		rules = append(rules,
			"float = true",
			"size = "+luaString(fmt.Sprintf("%d %d", w.Size[0], w.Size[1])),
			"move = "+luaString(fmt.Sprintf("%d %d", w.At[0], w.At[1])),
		)
	}

	rules = append(rules, "no_initial_focus = true")

	return fmt.Sprintf("hl.exec_cmd(%s, {%s})",
		luaString(shellCommand(argv)), strings.Join(rules, ", "))
}
