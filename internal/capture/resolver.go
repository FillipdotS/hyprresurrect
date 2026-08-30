// Package capture deals with capturing the specific process launched
//
// TODO: flatpak apps report in-sandbox argv (/app/bin/foo) that won't launch
// from the host; detect via /proc/<pid>/root/.flatpak-info and use `flatpak run <id>`.
//
// TODO: terminals need their contents captured, not just their argv. A terminal
// launched from a menu reports something like `ghostty --working-directory=~`
// no matter what is running inside it, and with --gtk-single-instance every
// window shares one pid, so seven windows resolve to one identical command.
// The information is in the process subtree:
//
//   - each terminal window has its own shell on its own pty; collect the
//     subtree of the window's pid and keep the processes with a controlling tty
//   - /proc/<shell>/cwd is the directory to reopen in
//   - /proc/<shell>/stat field 8 (tpgid) is the process group in the
//     foreground; equal to the shell pid means an idle shell, otherwise
//     /proc/<tpgid>/cmdline is the program to relaunch. Field 2 is the comm in
//     parens and may itself contain spaces and parens, so cut after the LAST
//     ')' before counting fields
//   - nothing links a pty to a window, so when one pid owns several windows
//     they have to be matched on title: the window title usually holds the
//     foreground program's name or the cwd with $HOME as ~. Score the pairs,
//     assign greedily, and pair off the leftovers arbitrarily - windows that
//     tie are running the same thing in the same place anyway
//   - relaunch flags differ per terminal (ghostty -e, foot, kitty --directory),
//     so keep a class -> template table and skip enrichment for unknown classes
//     rather than guessing
//
// Matching needs every window of a pid at once, so this can't hang off Command;
// it wants its own entry point taking the full client list. Refuse to relaunch
// a foreground program on a denylist (sudo, su, pacman) and fall back to the
// cwd alone - an unattended restore must not start a package manager. Store the
// cwd and the enriched command separately from the raw argv in the snapshot, so
// a bad guess stays debuggable.
package capture

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
)

var (
	// ErrProcessGone means the pid was not in procfs at all; the window closed
	// between listing the clients and reading /proc.
	ErrProcessGone = errors.New("process is gone")

	// ErrNoCommand means the process exists but nothing replayable came out of
	// it. The window is still worth saving, just without a command.
	ErrNoCommand = errors.New("no launch command resolved")
)

type Resolver struct {
	ProcRoot string
}

// Command returns the argv that would relaunch the process behind c. It reads
// procfs, so it only works while that process is alive: at restore time the
// saved pid means nothing.
func (r *Resolver) Command(c hypr.Client) ([]string, error) {
	pid := c.PID

	argv, err := r.cmdline(pid)
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
		parent, ok := r.parentOf(pid)
		if !ok {
			break
		}

		parentArgv, err := r.cmdline(parent)
		if err != nil {
			break
		}

		pid, argv = parent, parentArgv
	}

	if isHelper(argv) {
		return nil, fmt.Errorf("pid %d: still a browser helper: %w", pid, ErrNoCommand)
	}

	return argv, nil
}

// cmdline reads /proc/<pid>/cmdline, whose args are NUL separated.
func (r *Resolver) cmdline(pid int) ([]string, error) {
	b, err := os.ReadFile(r.procPath(pid, "cmdline"))

	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("pid %d: %w", pid, ErrProcessGone)
	}
	if err != nil {
		return nil, err
	}

	// A trailing NUL is normal, so the final field is usually empty. Kernel
	// threads and zombies have an entirely empty cmdline.
	var argv []string
	for arg := range bytes.SplitSeq(b, []byte{0}) {
		if len(arg) > 0 {
			argv = append(argv, string(arg))
		}
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("pid %d: empty cmdline: %w", pid, ErrNoCommand)
	}

	return argv, nil
}

// parentOf returns the parent pid, refusing to climb past init or out of the
// uid we started in: a uid change means we've left the app for something like a
// session manager, whose argv would be nonsense to replay.
func (r *Resolver) parentOf(pid int) (int, bool) {
	parent, uid, ok := r.statusOf(pid)
	if !ok || parent <= 1 {
		return 0, false
	}

	_, parentUID, ok := r.statusOf(parent)
	if !ok || parentUID != uid {
		return 0, false
	}

	return parent, true
}

// statusOf returns the parent pid and real uid from /proc/<pid>/status.
func (r *Resolver) statusOf(pid int) (parent, uid int, ok bool) {
	f, err := os.Open(r.procPath(pid, "status"))
	if err != nil {
		return 0, 0, false
	}
	defer func() { _ = f.Close() }()

	var haveParent, haveUID bool

	scan := bufio.NewScanner(f)
	for scan.Scan() {
		field, value, found := strings.Cut(scan.Text(), ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)

		switch field {
		case "PPid":
			parent, err = strconv.Atoi(value)
			haveParent = err == nil
		case "Uid":
			// "Uid: real effective saved fs"; the real uid is the one we want.
			real, _, _ := strings.Cut(value, "\t")
			uid, err = strconv.Atoi(real)
			haveUID = err == nil
		}
	}
	if scan.Err() != nil {
		return 0, 0, false
	}

	return parent, uid, haveParent && haveUID
}

func (r *Resolver) procPath(pid int, parts ...string) string {
	root := r.ProcRoot
	if root == "" {
		root = "/proc"
	}

	return filepath.Join(append([]string{root, strconv.Itoa(pid)}, parts...)...)
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
