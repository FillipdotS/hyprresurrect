// Package procfs reads process facts. root is "/proc" outside of tests.
package procfs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func Path(root string, pid int, parts ...string) string {
	return filepath.Join(append([]string{root, strconv.Itoa(pid)}, parts...)...)
}

// Cmdline reads /proc/<pid>/cmdline, whose args are NUL separated.
func Cmdline(root string, pid int) ([]string, error) {
	b, err := os.ReadFile(Path(root, pid, "cmdline"))

	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("pid %d: process is gone", pid)
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
		return nil, fmt.Errorf("pid %d: empty cmdline", pid)
	}

	return unmangle(root, pid, argv), nil
}

// unmangle restores the argv of a process that overwrote its own argv area to
// set its process title, which chromium and electron both do: the whole command
// line comes back as one element instead of NUL separated arguments. Quoting
// that as a single argument builds a filename that cannot exist, so the window
// never comes back at all.
//
// /proc/<pid>/exe is the kernel's record of the binary and no process can
// rewrite it, so it says exactly where the path ends rather than guessing at the
// first space - which matters, because the path may contain one. Splitting the
// arguments that follow is still a guess, but by then it is the only
// information left.
func unmangle(root string, pid int, argv []string) []string {
	if len(argv) != 1 {
		return argv
	}

	// A deleted binary reads back as "<path> (deleted)", which fails the prefix
	// check below and is left alone rather than mangled further.
	exe, err := os.Readlink(Path(root, pid, "exe"))
	if err != nil {
		return argv
	}

	args, ok := strings.CutPrefix(argv[0], exe+" ")
	if !ok {
		return argv
	}

	return append([]string{exe}, strings.Fields(args)...)
}

// ParentOf returns the parent pid, refusing to climb past init or out of the
// uid we started in: a uid change means we've left the app for something like a
// session manager, whose argv would be nonsense to replay.
func ParentOf(root string, pid int) (int, bool) {
	parent, uid, ok := Status(root, pid)
	if !ok || parent <= 1 {
		return 0, false
	}

	_, parentUID, ok := Status(root, parent)
	if !ok || parentUID != uid {
		return 0, false
	}

	return parent, true
}

// Status returns the parent pid and real uid from /proc/<pid>/status.
func Status(root string, pid int) (parent, uid int, ok bool) {
	f, err := os.Open(Path(root, pid, "status"))
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

type Stat struct {
	Parent    int
	Tpgid     int // terminal's foreground process group: own pid for an idle shell, -1 with no terminal
	StartTime uint64
}

// ReadStat parses /proc/<pid>/stat. The comm field may contain spaces and
// parens, so fields are counted after the last ')'.
func ReadStat(root string, pid int) (Stat, bool) {
	b, err := os.ReadFile(Path(root, pid, "stat"))
	if err != nil {
		return Stat{}, false
	}

	end := bytes.LastIndexByte(b, ')')
	if end < 0 {
		return Stat{}, false
	}

	// Field N of stat(5) is at index N-3.
	fields := strings.Fields(string(b[end+1:]))
	if len(fields) < 20 {
		return Stat{}, false
	}

	parent, errParent := strconv.Atoi(fields[1])
	tpgid, errTpgid := strconv.Atoi(fields[5])
	start, errStart := strconv.ParseUint(fields[19], 10, 64)

	if errParent != nil || errTpgid != nil || errStart != nil {
		return Stat{}, false
	}

	return Stat{Parent: parent, Tpgid: tpgid, StartTime: start}, true
}

func Children(root string, pid int) ([]int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var children []int

	for _, e := range entries {
		child, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}

		if stat, ok := ReadStat(root, child); ok && stat.Parent == pid {
			children = append(children, child)
		}
	}

	return children, nil
}

func Cwd(root string, pid int) (string, error) {
	return os.Readlink(Path(root, pid, "cwd"))
}
