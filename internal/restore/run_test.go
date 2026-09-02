package restore

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/google/go-cmp/cmp"
)

type fakeHypr struct {
	evaled  []string
	failOn  string
	clients []hypr.Client
	err     error

	// listings, when set, is returned one entry per Clients() call with the
	// last entry repeating, modelling windows that map over time.
	listings [][]hypr.Client
	calls    int
}

func (f *fakeHypr) Eval(lua string) error {
	f.evaled = append(f.evaled, lua)

	if f.failOn != "" && strings.Contains(lua, f.failOn) {
		return errors.New("boom")
	}

	return nil
}

func (f *fakeHypr) Clients() ([]hypr.Client, error) {
	f.calls++

	if f.err != nil {
		return nil, f.err
	}
	if f.listings != nil {
		return f.listings[min(f.calls-1, len(f.listings)-1)], nil
	}

	return f.clients, nil
}

// noProc stands in for the procfs lookup, so the Run tests never read /proc.
func noProc(int) ([]string, error) { return nil, errors.New("no procfs in tests") }

// testRunner never touches procfs and never waits long. Tests that are about
// the waiting itself set timeout and poll themselves.
func testRunner(h hyprland) Runner {
	return Runner{Hypr: h, command: noProc, timeout: 10 * time.Millisecond, poll: time.Millisecond}
}

var oneWindow = snapshot.Snapshot{
	Windows: []snapshot.Window{{
		Class:     "foot",
		Workspace: 3,
		Monitor:   "DP-1",
		Command:   []string{"foot"},
	}},
}

// The point of merging Run and Reconcile: one call spawns and then fixes up
// whatever the spawn rules failed to place.
func TestRunSpawnsThenMovesStragglers(t *testing.T) {
	h := &fakeHypr{clients: []hypr.Client{
		{Address: "0x1", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 1}},
	}}

	if err := testRunner(h).Run(oneWindow); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []string{
		`hl.workspace_rule({workspace = "3", monitor = "DP-1"})`,
		`hl.exec_cmd("foot", {workspace = "3 silent", no_initial_focus = true})`,
		`hl.dispatch(hl.dsp.window.move({window = "address:0x1", workspace = 3}))`,
	}

	if diff := cmp.Diff(want, h.evaled); diff != "" {
		t.Errorf("evaluated Lua mismatch (-want +got):\n%s", diff)
	}
}

func TestRunSkipsMovesWhenEverythingLanded(t *testing.T) {
	h := &fakeHypr{clients: []hypr.Client{
		{Address: "0x1", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 3}},
	}}

	if err := testRunner(h).Run(oneWindow); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, lua := range h.evaled {
		if strings.Contains(lua, "window.move") {
			t.Errorf("Run() moved a window that was already on workspace 3: %s", lua)
		}
	}
}

// A window that refuses to spawn must not cost the rest of the session, and
// must not skip the move pass either.
func TestRunReportsFailedStepsAndKeepsGoing(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 3, Command: []string{"foot"}},
			{Class: "Aseprite", Workspace: 4, Command: []string{"aseprite"}},
		},
	}

	h := &fakeHypr{
		failOn: "foot",
		clients: []hypr.Client{
			{Address: "0x2", Class: "Aseprite", Workspace: hypr.WorkspaceRef{ID: 1}},
		},
	}

	err := testRunner(h).Run(snap)
	if err == nil || !strings.Contains(err.Error(), "spawn foot") {
		t.Errorf("Run() error = %v, want it to name the failed step", err)
	}

	if len(h.evaled) != 3 {
		t.Errorf("Run() evaluated %d statements, want 3 (both spawns and the move): %v", len(h.evaled), h.evaled)
	}
}

func TestRunReportsSpawnErrorWhenClientsFail(t *testing.T) {
	h := &fakeHypr{failOn: "foot", err: errors.New("socket closed")}

	err := testRunner(h).Run(oneWindow)
	if err == nil || !strings.Contains(err.Error(), "spawn foot") || !strings.Contains(err.Error(), "socket closed") {
		t.Errorf("Run() error = %v, want both the spawn failure and the listing failure", err)
	}
}

// A dry run must describe the whole restore, move pass included, and must not
// touch hyprland at all - hence the nil Hypr.
func TestRunDryRunAnnouncesTheMovePass(t *testing.T) {
	var out strings.Builder

	r := Runner{timeout: 5 * time.Second, Out: &out, DryRun: true}
	if err := r.Run(oneWindow); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, want := range []string{
		"-- spawn foot",
		`hl.exec_cmd("foot", {workspace = "3 silent", no_initial_focus = true})`,
		"wait up to 5s",
		"move any window that missed its workspace",
		"foot x1 -> workspace 3",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--dry-run output missing %q:\n%s", want, out.String())
		}
	}
}

func TestRunEmptySnapshot(t *testing.T) {
	h := &fakeHypr{}

	if err := testRunner(h).Run(snapshot.Snapshot{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(h.evaled) != 0 {
		t.Errorf("Run(empty) evaluated %v, want nothing", h.evaled)
	}
}

func TestRunWaitsForSlowWindowsToMap(t *testing.T) {
	slow := []hypr.Client{{Address: "0x1", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 1}}}

	h := &fakeHypr{listings: [][]hypr.Client{
		{},   // baseline, before spawning
		{},   // still starting up
		slow, // mapped at last
	}}

	if err := (Runner{Hypr: h, command: noProc, timeout: time.Second, poll: time.Millisecond}).Run(oneWindow); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := `hl.dispatch(hl.dsp.window.move({window = "address:0x1", workspace = 3}))`
	if !slices.Contains(h.evaled, want) {
		t.Errorf("Run() evaluated %v, want it to move the window that mapped late", h.evaled)
	}
}

func TestRunStopsWaitingAtTheDeadline(t *testing.T) {
	h := &fakeHypr{}

	start := time.Now()
	if err := (Runner{Hypr: h, command: noProc, timeout: 30 * time.Millisecond, poll: time.Millisecond}).Run(oneWindow); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Run() waited %s, want it to give up after 30ms", elapsed)
	}
	if h.calls < 3 {
		t.Errorf("Run() listed clients %d times, want it to poll rather than sleep once", h.calls)
	}
}

func TestRunDoesNotCountPreexistingWindows(t *testing.T) {
	preexisting := []hypr.Client{{Address: "0x9", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 1}}}

	h := &fakeHypr{listings: [][]hypr.Client{preexisting}}

	if err := (Runner{Hypr: h, command: noProc, timeout: 30 * time.Millisecond, poll: time.Millisecond}).Run(oneWindow); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if h.calls < 3 {
		t.Errorf("Run() listed clients %d times, want it to keep waiting for the window it spawned", h.calls)
	}
}

func TestRunMovesEachCommandToItsOwnWorkspace(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "foot", Workspace: 3, Command: []string{"foot", "-e", "cliamp"}},
			{Class: "foot", Workspace: 5, Command: []string{"foot", "-e", "btop"}},
		},
	}

	h := &fakeHypr{clients: []hypr.Client{
		{Address: "0xBTOP", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 3}, PID: 200},
		{Address: "0xCLIAMP", Class: "foot", Workspace: hypr.WorkspaceRef{ID: 5}, PID: 100},
	}}

	byPID := map[int][]string{
		100: {"foot", "-e", "cliamp"},
		200: {"foot", "-e", "btop"},
	}

	r := testRunner(h)
	r.command = func(pid int) ([]string, error) { return byPID[pid], nil }
	if err := r.Run(snap); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, want := range []string{
		`hl.dispatch(hl.dsp.window.move({window = "address:0xCLIAMP", workspace = 3}))`,
		`hl.dispatch(hl.dsp.window.move({window = "address:0xBTOP", workspace = 5}))`,
	} {
		if !slices.Contains(h.evaled, want) {
			t.Errorf("Run() evaluated %v,\nwant it to contain %s", h.evaled, want)
		}
	}
}

// The order matters: a group can only be built once its members are on the same
// workspace, so the moves have to have gone out first.
func TestRunGroupsAfterMoving(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "alpha", Workspace: 3, Command: []string{"alpha"}, Group: 1},
			{Class: "beta", Workspace: 3, Command: []string{"beta"}, Group: 1, GroupActive: true},
			{Class: "gamma", Workspace: 3, Command: []string{"gamma"}, Group: 1},
		},
	}

	// beta ignored its spawn rule and landed on the active workspace.
	h := &fakeHypr{clients: []hypr.Client{
		{Address: "0xA", Class: "alpha", Workspace: hypr.WorkspaceRef{ID: 3}},
		{Address: "0xB", Class: "beta", Workspace: hypr.WorkspaceRef{ID: 1}},
		{Address: "0xG", Class: "gamma", Workspace: hypr.WorkspaceRef{ID: 3}},
	}}

	if err := testRunner(h).Run(snap); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []string{
		`hl.dispatch(hl.dsp.window.move({window = "address:0xB", workspace = 3}))`,
		`hl.dispatch(hl.dsp.group.toggle({window = "address:0xA"}))`,
		`hl.get_window("address:0xA").group:add(hl.get_window("address:0xB"))`,
		`hl.get_window("address:0xA").group:add(hl.get_window("address:0xG"))`,
		`hl.dispatch(hl.dsp.group.active({window = "address:0xA", index = 2}))`,
	}

	if diff := cmp.Diff(want, h.evaled[len(h.evaled)-len(want):]); diff != "" {
		t.Errorf("evaluated Lua mismatch (-want +got):\n%s", diff)
	}
}

func TestRunDryRunAnnouncesTheGroups(t *testing.T) {
	snap := snapshot.Snapshot{
		Windows: []snapshot.Window{
			{Class: "alpha", Workspace: 3, Command: []string{"alpha"}, Group: 1},
			{Class: "beta", Workspace: 3, Command: []string{"beta"}, Group: 1},
			{Class: "loner", Workspace: 3, Command: []string{"loner"}},
		},
	}

	var out strings.Builder

	r := Runner{timeout: 5 * time.Second, Out: &out, DryRun: true}
	if err := r.Run(snap); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if want := "alpha + beta on workspace 3"; !strings.Contains(out.String(), want) {
		t.Errorf("--dry-run output missing %q:\n%s", want, out.String())
	}
}
