package apps

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/FillipdotS/hyprresurrect/internal/procfs"
)

const ghosttyClass = "com.mitchellh.ghostty"

// ghostty pairs each window with its shell. Under a single instance every
// window shares one pid and argv, and nothing links a shell to its window, but
// both sides are ordered: hyprland's stableId counts windows as they are
// created, and each shell has a start time.
type ghostty struct{}

func (ghostty) Owns(class string) bool { return class == ghosttyClass }

func (ghostty) Inspect(windows []hypr.Client, procRoot string) (map[string]Detail, bool) {
	details := make(map[string]Detail, len(windows))
	ready := true

	byPID := make(map[int][]hypr.Client)
	for _, w := range windows {
		byPID[w.PID] = append(byPID[w.PID], w)
	}

	for pid, group := range byPID {
		if argv, err := procfs.Cmdline(procRoot, pid); err != nil || runsCommand(argv) || !ordered(group) {
			continue
		}

		shells := shellsOf(procRoot, pid)

		// Fewer shells than windows: a new window's shell is forked a moment
		// after it maps. More: tabs or splits, so which shell belongs to which
		// window is unknowable and the whole instance is skipped.
		if len(shells) < len(group) {
			ready = false
			continue
		}
		if len(shells) > len(group) {
			continue
		}

		slices.SortFunc(group, func(a, b hypr.Client) int {
			return strings.Compare(stableID(a), stableID(b))
		})

		for i, w := range group {
			details[w.Address] = inside(procRoot, shells[i])
		}
	}

	return details, ready
}

// shellsOf returns pid's children that own a terminal, oldest first.
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
		if stat, ok := procfs.ReadStat(procRoot, child); ok && stat.Tpgid > 0 {
			shells = append(shells, shell{pid: child, start: stat.StartTime})
		}
	}

	slices.SortFunc(shells, func(a, b shell) int {
		return cmp.Or(cmp.Compare(a.start, b.start), cmp.Compare(a.pid, b.pid))
	})

	pids := make([]int, len(shells))
	for i, s := range shells {
		pids[i] = s.pid
	}

	return pids
}

func inside(procRoot string, shell int) Detail {
	cwd, err := procfs.Cwd(procRoot, shell)
	if err != nil {
		return Detail{}
	}

	detail := Detail{Cwd: cwd}

	stat, ok := procfs.ReadStat(procRoot, shell)
	if !ok || stat.Tpgid <= 0 || stat.Tpgid == shell {
		return detail
	}

	if argv, err := procfs.Cmdline(procRoot, stat.Tpgid); err == nil {
		detail.Program = argv
	}

	return detail
}

// ordered reports whether every window has a stableId; one without would pair
// with an arbitrary shell.
func ordered(windows []hypr.Client) bool {
	for _, w := range windows {
		if _, err := strconv.ParseUint(w.StableID, 16, 64); err != nil {
			return false
		}
	}

	return true
}

// stableID left-pads the hex id so string order is numeric order.
func stableID(w hypr.Client) string {
	return strings.Repeat("0", max(0, 16-len(w.StableID))) + w.StableID
}

func runsCommand(argv []string) bool {
	return slices.ContainsFunc(argv[min(1, len(argv)):], func(arg string) bool {
		return arg == "-e" || strings.HasPrefix(arg, "--command=")
	})
}

// Launch opens the window through the running ghostty, as a keybind would:
// replaying a single instance's own argv, often --initial-window=false, need not
// open a window at all.
// Only --class carries over, since it picks which instance to ask. What was
// running is deliberately not restarted.
//
// A ghostty started with -e is its own process, and its argv reopens it as is.
func (ghostty) Launch(argv []string, detail Detail) []string {
	if len(argv) == 0 || runsCommand(argv) {
		return argv
	}

	out := []string{argv[0], "+new-window"}

	for _, arg := range argv[1:] {
		if strings.HasPrefix(arg, "--class=") {
			out = append(out, arg)
		}
	}

	if detail.Cwd != "" {
		out = append(out, "--working-directory="+detail.Cwd)
	}

	return out
}
