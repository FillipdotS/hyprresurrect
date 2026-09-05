// TODO: flatpak apps report in-sandbox argv (/app/bin/foo) that won't launch
// from the host; detect via /proc/<pid>/root/.flatpak-info and use `flatpak run <id>`.

package snapshot

import (
	"fmt"
	"strings"

	"github.com/FillipdotS/hyprresurrect/internal/procfs"
)

// Command returns the argv that would relaunch pid, read from the live procfs.
// Restore uses it to tell two windows of one class apart: a live window is the
// one a snapshot window describes if it would be captured the same way.
func Command(pid int) ([]string, error) {
	return command("/proc", pid)
}

// command returns the argv that would relaunch pid. It reads procfs, so it only
// works while that process is alive: at restore time the saved pid means
// nothing. procRoot is "/proc" outside of tests.
func command(procRoot string, pid int) ([]string, error) {
	argv, err := procfs.Cmdline(procRoot, pid)
	if err != nil {
		return nil, err
	}

	// maxParentHops bounds the walk out of a browser helper process. Real trees are
	// two deep; a longer one means the assumption is wrong, not that we should keep
	// climbing.
	const maxParentHops = 5

	// Chromium and Electron spawn renderers, zygotes and GPU processes whose
	// argv is full of per-run fds and tokens. Hyprland usually reports the
	// top-level process, but an XWayland window sets its own pid, so climb until
	// we reach a process that isn't a helper.
	for hops := 0; isHelper(argv) && hops < maxParentHops; hops++ {
		parent, ok := procfs.ParentOf(procRoot, pid)
		if !ok {
			break
		}

		parentArgv, err := procfs.Cmdline(procRoot, parent)
		if err != nil {
			break
		}

		pid, argv = parent, parentArgv
	}

	if isHelper(argv) {
		return nil, fmt.Errorf("pid %d: still a browser helper", pid)
	}

	return argv, nil
}

// Reports whether argv belongs to a chromium/electron child process
// rather than to the browser itself.
func isHelper(argv []string) bool {
	for _, arg := range argv {
		if strings.HasPrefix(arg, "--type=") {
			return true
		}
	}

	return false
}
