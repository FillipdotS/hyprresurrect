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

// Ghostty is the case terminal restoration exists for: with a single instance
// one process serves every window, so every window reports the same argv and
// the snapshot cannot tell one running btop from an idle one. What separates
// them is the shell behind each window, with its own pty, cwd and foreground
// process.
//
// A restore reopens each window in the directory it was in, through the running
// ghostty's own IPC, and starts nothing: what was running is recorded and named
// by --dry-run, but putting it back is left to whoever reads it.
func TestGhostty(t *testing.T) {
	hr := setup(t)

	// Not a skip: nothing runs this suite in CI, so a missing ghostty means
	// the question went unanswered on a machine that could not answer it, and
	// saying nothing would read as a pass.
	if _, err := exec.LookPath("ghostty"); err != nil {
		t.Fatalf("ghostty is not installed, so the terminal round trip cannot be tested: %v", err)
	}

	host := ghosttyPids(t)

	var (
		busy  = tempDir(t)
		other = tempDir(t)
		idle  = tempDir(t)
	)

	// A shell puts its job in a process group of its own, out of reach of the
	// group kill that takes the terminal down, so a test that fails part way
	// through would leave these behind.
	t.Cleanup(func() { killMarkers(t, "sleep 999901", "sleep 999902") })

	// The windows a restore opens belong to an instance started by the bus,
	// which has its own idea about asking before closing a surface and no way
	// to be told otherwise over the IPC. Taking the whole instance down is what
	// leaves the compositor empty for the next test.
	t.Cleanup(func() { killGhostty(t, host) })

	nested.FocusWorkspace(t, 1)
	first := nested.SpawnGhosttyServer(t, idle)

	nested.FocusWorkspace(t, 2)
	second := nested.SpawnGhosttyWindow(t, busy, "sleep", "999901")

	nested.FocusWorkspace(t, 3)
	third := nested.SpawnGhosttyWindow(t, other, "sleep", "999902")

	// The premise. If ghostty ever stops sharing a process between windows this
	// feature is pointless, and this test should be what says so.
	if first.PID != second.PID || second.PID != third.PID {
		t.Fatalf("the three windows report pids %d, %d and %d, want one shared process",
			first.PID, second.PID, third.PID)
	}

	if host[first.PID] {
		t.Fatalf("pid %d was already running before the suite started: the nested ghostty "+
			"reached the desktop's single-instance server instead of our private bus", first.PID)
	}

	// A shell reports its job as the foreground one only once it has forked it,
	// which is a moment after the window maps.
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

	// Each window reopens where it was, and the plan says what it will not be
	// starting again.
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

	// The state a restore actually runs in: nothing of ours left running, so
	// the bus has to start ghostty again. An instance that is up but has lost
	// every window accepts the request to open one and then does not, so
	// restoring into one would prove nothing.
	killGhostty(t, host)

	hr.Run("restore")

	// A restored window maps before ghostty has forked the shell behind it, and
	// until it has there is nothing to read a directory out of.
	awaitShells(t, 3)

	// Saving again is what shows the pairing survived: every window back on its
	// own workspace in its own directory, with nothing running in any of them.
	hr.Run("save")

	wantBack := []string{
		"ws1 " + idle + " ",
		"ws2 " + busy + " ",
		"ws3 " + other + " ",
	}

	if diff := cmp.Diff(wantBack, terminals(hr.Saved())); diff != "" {
		t.Fatalf("the terminals did not survive the round trip (-want +got):\n%s", diff)
	}

	// The windows came back through the running ghostty rather than as processes
	// of their own, which is the point of going through its IPC.
	restored := onWorkspace(t, 2)

	if restored.PID != onWorkspace(t, 1).PID || restored.PID != onWorkspace(t, 3).PID {
		t.Errorf("the restored windows are on separate processes, want the one instance")
	}

	if fg := foreground(t, restored.PID); len(fg) != 0 {
		t.Errorf("a restored window is running something: %v", fg)
	}
}

// awaitShells waits until there are want ghostty windows and every one of them
// has its shell.
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

// shells counts the children of pid that own a terminal.
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

// awaitForeground waits until every one of want is running in one of pid's
// shells.
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

// SpawnGhosttyServer opens the window that starts ghostty, which every later
// window is then served by. Its shell is left idle: a server started with -e
// answers the new-window IPC without opening anything, so the windows running
// something have to come from SpawnGhosttyWindow.
func (c *compositor) SpawnGhosttyServer(t *testing.T, dir string) hypr.Client {
	t.Helper()

	// Single instance forced on: it is off by default when ghostty is started
	// from a terminal, and one process behind every window is the whole point.
	//
	// The windows opened later inherit this configuration, which is the only
	// way to reach them: ghostty asks before closing a window with something
	// running in it, and a test cannot answer.
	return c.spawnGhostty(t, []string{
		"--gtk-single-instance=true",
		"--confirm-close-surface=false",
		"--working-directory=" + dir,
	})
}

// SpawnGhosttyWindow opens a window in the running ghostty over its IPC, the
// path a desktop keybind takes.
func (c *compositor) SpawnGhosttyWindow(t *testing.T, dir string, args ...string) hypr.Client {
	t.Helper()

	return c.spawnGhostty(t, append([]string{"+new-window"}, ghosttyArgs(dir, args)...))
}

func (c *compositor) spawnGhostty(t *testing.T, args []string) hypr.Client {
	t.Helper()

	return c.spawn(t, ghosttyClass, exec.Command("ghostty", args...))
}

// ghosttyArgs runs args as the foreground job of an interactive shell, or an
// idle shell when there are none.
//
// bash -c execs over itself given a single command, leaving no shell and no job
// control, and without job control the program shares the shell's process group
// so the pty never names it as the foreground one. The trailing `true` is what
// stops that.
func ghosttyArgs(dir string, args []string) []string {
	shell := []string{"bash", "-i"}
	if len(args) > 0 {
		shell = append(shell, "-c", strings.Join(args, " ")+"; true")
	}

	return append([]string{"--working-directory=" + dir, "-e"}, shell...)
}

// terminals describes every ghostty window in a snapshot as "wsN <cwd>
// <program>", so a diff names what changed rather than which struct field did.
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

// foreground is what is running in each of pid's shells, read straight out of
// procfs rather than through the code under test.
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

// state is the single-letter run state from /proc/<pid>/stat.
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

// shellStat reads the parent pid and foreground process group out of
// /proc/<pid>/stat, whose second field is a comm in parens that may itself hold
// spaces and parens - so the fields are counted after the last one.
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

// killGhostty takes down every ghostty except the ones that were already
// running before the suite started.
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

// killMarkers kills whatever is still running one of the test's own commands.
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

// ghosttyPids is every ghostty already running, which the ones we spawn must
// not turn out to be.
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

		// A killed process the suite has not reaped yet keeps its name in
		// procfs, and counting those as running means waiting for something
		// that has already gone.
		if state(e.Name()) == "Z" {
			continue
		}

		pids[pid] = true
	}

	return pids
}

// tempDir is t.TempDir() with its symlinks resolved, because a cwd read back
// out of procfs is always fully resolved.
func tempDir(t *testing.T) string {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving temp dir: %v", err)
	}

	return dir
}
