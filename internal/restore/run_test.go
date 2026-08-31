package restore

import (
	"errors"
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
}

func (f *fakeHypr) Eval(lua string) error {
	f.evaled = append(f.evaled, lua)

	if f.failOn != "" && strings.Contains(lua, f.failOn) {
		return errors.New("boom")
	}

	return nil
}

func (f *fakeHypr) Clients() ([]hypr.Client, error) {
	return f.clients, f.err
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

	if err := (Runner{Hypr: h}).Run(oneWindow); err != nil {
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

	if err := (Runner{Hypr: h}).Run(oneWindow); err != nil {
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

	err := (Runner{Hypr: h}).Run(snap)
	if err == nil || !strings.Contains(err.Error(), "spawn foot") {
		t.Errorf("Run() error = %v, want it to name the failed step", err)
	}

	if len(h.evaled) != 3 {
		t.Errorf("Run() evaluated %d statements, want 3 (both spawns and the move): %v", len(h.evaled), h.evaled)
	}
}

func TestRunReportsSpawnErrorWhenClientsFail(t *testing.T) {
	h := &fakeHypr{failOn: "foot", err: errors.New("socket closed")}

	err := (Runner{Hypr: h}).Run(oneWindow)
	if err == nil || !strings.Contains(err.Error(), "spawn foot") || !strings.Contains(err.Error(), "socket closed") {
		t.Errorf("Run() error = %v, want both the spawn failure and the listing failure", err)
	}
}

// A dry run must describe the whole restore, move pass included, and must not
// touch hyprland at all - hence the nil Hypr.
func TestRunDryRunAnnouncesTheMovePass(t *testing.T) {
	var out strings.Builder

	r := Runner{Settle: 5 * time.Second, Out: &out, DryRun: true}
	if err := r.Run(oneWindow); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for _, want := range []string{
		"-- spawn foot",
		`hl.exec_cmd("foot", {workspace = "3 silent", no_initial_focus = true})`,
		"wait 5s",
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

	if err := (Runner{Hypr: h}).Run(snapshot.Snapshot{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(h.evaled) != 0 {
		t.Errorf("Run(empty) evaluated %v, want nothing", h.evaled)
	}
}
