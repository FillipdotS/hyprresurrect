package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/snapshot"
	"github.com/google/go-cmp/cmp"
)

const ghosttyClass = "com.mitchellh.ghostty"

func TestGhostty(t *testing.T) {
	hr := setup(t)

	if _, err := exec.LookPath("ghostty"); err != nil {
		t.Fatalf("ghostty is not installed, so the terminal round trip cannot be tested: %v", err)
	}

	host := ghosttyPids(t)

	var (
		busy  = tempDir(t)
		other = tempDir(t)
		idle  = tempDir(t)
	)

	// Jobs get their own process group, so killing ghostty does not reach them.
	t.Cleanup(func() { killMarkers(t, "sleep 999901", "sleep 999902") })

	// The restored windows belong to a bus-started instance that asks before
	// closing a surface, so the whole instance is killed instead.
	t.Cleanup(func() { killGhostty(t, host) })

	nested.FocusWorkspace(t, 1)
	first := nested.SpawnGhosttyServer(t, idle)

	nested.FocusWorkspace(t, 2)
	second := nested.SpawnGhosttyWindow(t, busy, "sleep", "999901")

	nested.FocusWorkspace(t, 3)
	third := nested.SpawnGhosttyWindow(t, other, "sleep", "999902")

	if first.PID != second.PID || second.PID != third.PID {
		t.Fatalf("the three windows report pids %d, %d and %d, want one shared process",
			first.PID, second.PID, third.PID)
	}

	if host[first.PID] {
		t.Fatalf("pid %d was already running before the suite started: the nested ghostty "+
			"reached the desktop's single-instance server instead of our private bus", first.PID)
	}

	awaitForeground(t, first.PID, "sleep 999901", "sleep 999902")

	want := []string{
		"ws1 " + idle + " ",
		"ws2 " + busy + " sleep 999901",
		"ws3 " + other + " sleep 999902",
	}

	hr.Run("save")

	if diff := cmp.Diff(want, terminals(hr.Saved())); diff != "" {
		t.Fatalf("the save did not describe what was in each window (-want +got):\n%s", diff)
	}

	plan := hr.Run("restore", "--dry-run")
	for _, fragment := range []string{
		"+new-window --working-directory=" + busy,
		"+new-window --working-directory=" + idle,
		"these were running, and are not started again",
		"workspace 2: sleep 999901",
	} {
		if !strings.Contains(plan, fragment) {
			t.Errorf("the restore plan is missing %q:\n%s", fragment, plan)
		}
	}

	if strings.Contains(plan, "-e sleep") || strings.Contains(plan, "--input") {
		t.Errorf("the restore plan would put the saved command back:\n%s", plan)
	}

	nested.CloseAllWindows(t)

	// An instance left running with no windows ignores +new-window, so restore
	// from cold, as after a reboot.
	killGhostty(t, host)

	hr.Run("restore")

	awaitShells(t, 3)

	hr.Run("save")

	wantBack := []string{
		"ws1 " + idle + " ",
		"ws2 " + busy + " ",
		"ws3 " + other + " ",
	}

	if diff := cmp.Diff(wantBack, terminals(hr.Saved())); diff != "" {
		t.Fatalf("the terminals did not survive the round trip (-want +got):\n%s", diff)
	}

	restored := onWorkspace(t, 2)

	if restored.PID != onWorkspace(t, 1).PID || restored.PID != onWorkspace(t, 3).PID {
		t.Errorf("the restored windows are on separate processes, want the one instance")
	}

	if fg := foreground(t, restored.PID); len(fg) != 0 {
		t.Errorf("a restored window is running something: %v", fg)
	}
}

func awaitShells(t *testing.T, want int) {
	t.Helper()

	deadline := time.Now().Add(waitTimeout)

	for time.Now().Before(deadline) {
		windows := 0
		ready := 0

		clients, err := nested.Socket().Clients()
		if err != nil {
			t.Fatalf("clients: %v", err)
		}

		for _, c := range clients {
			if c.Class != ghosttyClass {
				continue
			}

			windows++

			if shells(t, c.PID) > 0 {
				ready++
			}
		}

		if windows == want && ready == want {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("after %v there are not %d ghostty windows each with a shell", waitTimeout, want)
}

func shells(t *testing.T, pid int) int {
	t.Helper()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0
	}

	count := 0

	for _, e := range entries {
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}

		if parent, tpgid, ok := shellStat(e.Name()); ok && parent == pid && tpgid > 0 {
			count++
		}
	}

	return count
}

func awaitForeground(t *testing.T, pid int, want ...string) {
	t.Helper()

	deadline := time.Now().Add(waitTimeout)

	var running []string

	for time.Now().Before(deadline) {
		running = foreground(t, pid)

		missing := false
		for _, w := range want {
			missing = missing || !slices.Contains(running, w)
		}

		if !missing {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("after %v the shells of pid %d are running %v, want %v", waitTimeout, pid, running, want)
}

// SpawnGhosttyServer starts the single instance with an idle shell: a server
// started with -e ignores +new-window.
func (c *compositor) SpawnGhosttyServer(t *testing.T, dir string) hypr.Client {
	t.Helper()

	// Single instance is off by default when started from a terminal.
	// Later windows inherit confirm-close-surface; nothing else can set it.
	return c.spawnGhostty(t, []string{
		"--gtk-single-instance=true",
		"--confirm-close-surface=false",
		"--working-directory=" + dir,
	})
}

func (c *compositor) SpawnGhosttyWindow(t *testing.T, dir string, args ...string) hypr.Client {
	t.Helper()

	return c.spawnGhostty(t, append([]string{"+new-window"}, ghosttyArgs(dir, args)...))
}

func (c *compositor) spawnGhostty(t *testing.T, args []string) hypr.Client {
	t.Helper()

	return c.spawn(t, ghosttyClass, exec.Command("ghostty", args...))
}

// ghosttyArgs runs args as the foreground job of an interactive shell. The
// trailing `true` stops bash -c from exec'ing a single command, which would
// leave no shell for the job to be in the foreground of.
func ghosttyArgs(dir string, args []string) []string {
	shell := []string{"bash", "-i"}
	if len(args) > 0 {
		shell = append(shell, "-c", strings.Join(args, " ")+"; true")
	}

	return append([]string{"--working-directory=" + dir, "-e"}, shell...)
}

func terminals(snap snapshot.Snapshot) []string {
	var out []string

	for _, w := range snap.Windows {
		if w.Class != ghosttyClass {
			continue
		}

		out = append(out, fmt.Sprintf("ws%d %s %s", w.Workspace, w.Cwd, strings.Join(w.Program, " ")))
	}

	slices.Sort(out)

	return out
}

func onWorkspace(t *testing.T, workspace int) hypr.Client {
	t.Helper()

	clients, err := nested.Socket().Clients()
	if err != nil {
		t.Fatalf("clients: %v", err)
	}

	for _, c := range clients {
		if c.Class == ghosttyClass && c.Workspace.ID == workspace {
			return c
		}
	}

	t.Fatalf("no ghostty window on workspace %d", workspace)

	return hypr.Client{}
}

func foreground(t *testing.T, pid int) []string {
	t.Helper()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatalf("reading /proc: %v", err)
	}

	var running []string

	for _, e := range entries {
		child, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		parent, tpgid, ok := shellStat(e.Name())
		if !ok || parent != pid || tpgid <= 0 || tpgid == child {
			continue
		}

		argv, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(tpgid), "cmdline"))
		if err != nil {
			continue
		}

		running = append(running, strings.ReplaceAll(strings.TrimRight(string(argv), "\x00"), "\x00", " "))
	}

	return running
}

func state(pid string) string {
	stat, err := os.ReadFile(filepath.Join("/proc", pid, "stat"))
	if err != nil {
		return ""
	}

	end := strings.LastIndex(string(stat), ")")
	if end < 0 {
		return ""
	}

	fields := strings.Fields(string(stat)[end+1:])
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

func shellStat(pid string) (parent, tpgid int, ok bool) {
	stat, err := os.ReadFile(filepath.Join("/proc", pid, "stat"))
	if err != nil {
		return 0, 0, false
	}

	end := strings.LastIndex(string(stat), ")")
	if end < 0 {
		return 0, 0, false
	}

	fields := strings.Fields(string(stat)[end+1:])
	if len(fields) < 6 {
		return 0, 0, false
	}

	parent, errParent := strconv.Atoi(fields[1])
	tpgid, errTpgid := strconv.Atoi(fields[5])

	return parent, tpgid, errParent == nil && errTpgid == nil
}

func killGhostty(t *testing.T, keep map[int]bool) {
	t.Helper()

	for pid := range ghosttyPids(t) {
		if !keep[pid] {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}

	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		gone := true

		for pid := range ghosttyPids(t) {
			gone = gone && keep[pid]
		}

		if gone {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("a ghostty of ours was still running after %v", waitTimeout)
}

func killMarkers(t *testing.T, markers ...string) {
	t.Helper()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		argv, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}

		line := strings.ReplaceAll(strings.TrimRight(string(argv), "\x00"), "\x00", " ")
		if slices.Contains(markers, line) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}

func ghosttyPids(t *testing.T) map[int]bool {
	t.Helper()

	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatalf("reading /proc: %v", err)
	}

	pids := make(map[int]bool)

	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		comm, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != "ghostty" {
			continue
		}

		if state(e.Name()) == "Z" {
			continue
		}

		pids[pid] = true
	}

	return pids
}

// tempDir resolves symlinks, as a cwd read from procfs always is.
func tempDir(t *testing.T) string {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving temp dir: %v", err)
	}

	return dir
}
