package apps

import (
	"slices"
	"strconv"
	"strings"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/procfs"
)

const ghosttyClass = "com.mitchellh.ghostty"

// ghostty reads which directory each terminal window is open in, and reopens it
// there.
//
// Under a single instance one process serves every window, so every window
// reports the same pid and the same argv. What tells them apart is one shell
// per window, each on its own pty with its own cwd and its own foreground
// process. Nothing in the protocol says which shell belongs to which window,
// but both sides are ordered: hyprland numbers windows as they are created, and
// procfs records when each shell started. Sorting each and pairing them off
// recovers the mapping.
type ghostty struct{}

func (ghostty) Owns(class string) bool { return class == ghosttyClass }

func (g ghostty) Inspect(windows []hypr.Client, procRoot string) map[string]Detail {
	details := make(map[string]Detail, len(windows))

	byPID := make(map[int][]hypr.Client)
	for _, w := range windows {
		byPID[w.PID] = append(byPID[w.PID], w)
	}

	for pid, group := range byPID {
		shells := shellsOf(procRoot, pid)

		// A window per shell or nothing. Ghostty splits and tabs put several
		// shells behind one window, and there is no honest way to tell which
		// of them the window should reopen with - better to record nothing
		// than to pick one and be wrong.
		if len(shells) != len(group) || !ordered(group) {
			continue
		}

		slices.SortFunc(group, func(a, b hypr.Client) int {
			return strings.Compare(stableID(a), stableID(b))
		})

		for i, w := range group {
			details[w.Address] = inside(procRoot, shells[i])
		}
	}

	return details
}

// shellsOf returns the terminal's shells, oldest first: the direct children of
// the ghostty process that own a terminal.
func shellsOf(procRoot string, pid int) []int {
	children, err := procfs.Children(procRoot, pid)
	if err != nil {
		return nil
	}

	type shell struct {
		pid   int
		start uint64
	}

	var shells []shell

	for _, child := range children {
		// Tpgid is -1 for a process with no controlling terminal, which is
		// anything ghostty spawned that is not one of its windows' shells.
		if stat, ok := procfs.ReadStat(procRoot, child); ok && stat.Tpgid > 0 {
			shells = append(shells, shell{pid: child, start: stat.StartTime})
		}
	}

	slices.SortFunc(shells, func(a, b shell) int {
		if a.start != b.start {
			return int(a.start) - int(b.start)
		}

		// Two shells started in the same tick still have to be ordered the
		// same way twice.
		return a.pid - b.pid
	})

	pids := make([]int, len(shells))
	for i, s := range shells {
		pids[i] = s.pid
	}

	return pids
}

// inside reads the directory a shell is in and whatever it is running.
func inside(procRoot string, shell int) Detail {
	cwd, err := procfs.Cwd(procRoot, shell)
	if err != nil {
		return Detail{}
	}

	detail := Detail{Cwd: cwd}

	// The foreground process group of the pty. The shell's own pid means it is
	// sitting at its prompt with nothing to record.
	stat, ok := procfs.ReadStat(procRoot, shell)
	if !ok || stat.Tpgid <= 0 || stat.Tpgid == shell {
		return detail
	}

	if argv, err := procfs.Cmdline(procRoot, stat.Tpgid); err == nil {
		detail.Program = argv
	}

	return detail
}

// ordered reports whether every window carries a creation id to sort on. One
// that does not would take an arbitrary place in the ordering and silently pair
// the wrong shell with it.
func ordered(windows []hypr.Client) bool {
	for _, w := range windows {
		if _, err := strconv.ParseUint(w.StableID, 16, 64); err != nil {
			return false
		}
	}

	return true
}

// stableID left-pads hyprland's hex creation counter so that string order is
// numeric order.
func stableID(w hypr.Client) string {
	return strings.Repeat("0", max(0, 16-len(w.StableID))) + w.StableID
}

// Launch reopens the window through the running ghostty rather than by
// starting one, which is the path a desktop keybind takes and the only one
// ghostty supports for this. Just three settings cross that interface -
// --working-directory, --command and --title - so the rest of the saved argv
// is dropped: it configured a process that is already running, and passing it
// again would not change anything.
//
// The command that was running is deliberately not passed on. It is recorded,
// and a --dry-run will name it, but reopening a session should not start
// anything.
func (g ghostty) Launch(argv []string, detail Detail) []string {
	if len(argv) == 0 || detail.Cwd == "" {
		return argv
	}

	out := []string{argv[0], "+new-window"}

	// --class is not one of the three, but it is not for the new window: it
	// picks which running ghostty to ask, so a window that had one needs it.
	for _, arg := range argv[1:] {
		if strings.HasPrefix(arg, "--class=") {
			out = append(out, arg)
		}
	}

	return append(out, "--working-directory="+detail.Cwd)
}
