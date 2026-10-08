package apps

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/google/go-cmp/cmp"
)

type fakeProc struct{ root string }

func newFakeProc(t *testing.T) *fakeProc {
	t.Helper()

	return &fakeProc{root: t.TempDir()}
}

func (p *fakeProc) addProcess(t *testing.T, pid, ppid, tpgid int, start uint64, cwd string, argv ...string) {
	t.Helper()

	dir := filepath.Join(p.root, strconv.Itoa(pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0] = "S"
	fields[1] = strconv.Itoa(ppid)
	fields[5] = strconv.Itoa(tpgid)
	fields[19] = strconv.FormatUint(start, 10)

	stat := strconv.Itoa(pid) + " (ba(sh) x) " + strings.Join(fields, " ")
	p.write(t, filepath.Join(dir, "stat"), stat)

	if len(argv) > 0 {
		p.write(t, filepath.Join(dir, "cmdline"), strings.Join(argv, "\x00")+"\x00")
	}

	if cwd != "" {
		if err := os.Symlink(cwd, filepath.Join(dir, "cwd")); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}
	}
}

func (p *fakeProc) write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func window(address, stableID string, pid int) hypr.Client {
	return hypr.Client{Address: address, Class: ghosttyClass, PID: pid, StableID: stableID}
}

func inspect(t *testing.T, windows []hypr.Client, root string, wantReady bool) map[string]Detail {
	t.Helper()

	got, ready := ghostty{}.Inspect(windows, root)
	if ready != wantReady {
		t.Errorf("Inspect() ready = %v, want %v", ready, wantReady)
	}

	return got
}

func TestGhosttyPairsWindowsWithShellsByAge(t *testing.T) {
	proc := newFakeProc(t)

	proc.addProcess(t, 100, 1, -1, 10, "", "/usr/bin/ghostty", "--gtk-single-instance=true")

	proc.addProcess(t, 201, 100, 300, 20, "/home/u/first", "/usr/bin/bash")
	proc.addProcess(t, 202, 100, 202, 30, "/home/u/second", "/usr/bin/bash")
	proc.addProcess(t, 203, 100, 301, 40, "/home/u/third", "/usr/bin/bash")

	proc.addProcess(t, 300, 201, 300, 21, "/home/u/first", "btop")
	proc.addProcess(t, 301, 203, 301, 41, "/home/u/third", "herdr", "--follow")

	// Hex ids: "f" sorts after "10" as text.
	windows := []hypr.Client{
		window("0xC", "ff", 100),
		window("0xA", "f", 100),
		window("0xB", "10", 100),
	}

	want := map[string]Detail{
		"0xA": {Cwd: "/home/u/first", Program: []string{"btop"}},
		"0xB": {Cwd: "/home/u/second"},
		"0xC": {Cwd: "/home/u/third", Program: []string{"herdr", "--follow"}},
	}

	if diff := cmp.Diff(want, inspect(t, windows, proc.root, true)); diff != "" {
		t.Errorf("Inspect() mismatch (-want +got):\n%s", diff)
	}
}

func TestGhosttySkipsAnInstanceWithTabsButNotTheOthers(t *testing.T) {
	proc := newFakeProc(t)

	proc.addProcess(t, 100, 1, -1, 10, "", "/usr/bin/ghostty")
	proc.addProcess(t, 201, 100, 201, 20, "/home/u/a", "/usr/bin/bash")
	proc.addProcess(t, 202, 100, 202, 30, "/home/u/b", "/usr/bin/bash")

	proc.addProcess(t, 500, 1, -1, 10, "", "/usr/bin/ghostty", "--class=com.mitchellh.ghostty")
	proc.addProcess(t, 501, 500, 501, 20, "/home/u/c", "/usr/bin/bash")

	windows := []hypr.Client{window("0xA", "1", 100), window("0xC", "2", 500)}
	want := map[string]Detail{"0xC": {Cwd: "/home/u/c"}}

	if diff := cmp.Diff(want, inspect(t, windows, proc.root, true)); diff != "" {
		t.Errorf("Inspect() mismatch (-want +got):\n%s", diff)
	}
}

func TestGhosttyIsNotReadyWhileAShellIsMissing(t *testing.T) {
	proc := newFakeProc(t)

	proc.addProcess(t, 100, 1, -1, 10, "", "/usr/bin/ghostty")
	proc.addProcess(t, 201, 100, 201, 20, "/home/u/a", "/usr/bin/bash")

	windows := []hypr.Client{window("0xA", "1", 100), window("0xB", "2", 100)}

	if got := inspect(t, windows, proc.root, false); len(got) != 0 {
		t.Errorf("Inspect() = %v, want nothing", got)
	}
}

func TestGhosttyIgnoresWindowsItCannotOrder(t *testing.T) {
	proc := newFakeProc(t)

	proc.addProcess(t, 100, 1, -1, 10, "", "/usr/bin/ghostty")
	proc.addProcess(t, 201, 100, 201, 20, "/home/u/a", "/usr/bin/bash")
	proc.addProcess(t, 202, 100, 202, 30, "/home/u/b", "/usr/bin/bash")

	windows := []hypr.Client{window("0xA", "1", 100), window("0xB", "", 100)}

	if got := inspect(t, windows, proc.root, true); len(got) != 0 {
		t.Errorf("Inspect() = %v, want nothing when a window has no stable id", got)
	}
}

func TestGhosttyIgnoresAProcessStartedWithACommand(t *testing.T) {
	proc := newFakeProc(t)

	proc.addProcess(t, 100, 1, -1, 10, "", "/usr/bin/ghostty", "-e", "ncspot")
	proc.addProcess(t, 201, 100, 201, 20, "/home/u", "ncspot")

	if got := inspect(t, []hypr.Client{window("0xA", "1", 100)}, proc.root, true); len(got) != 0 {
		t.Errorf("Inspect() = %v, want nothing for a ghostty -e", got)
	}
}

func TestGhosttySkipsChildrenWithNoTerminal(t *testing.T) {
	proc := newFakeProc(t)

	proc.addProcess(t, 100, 1, -1, 10, "", "/usr/bin/ghostty")
	proc.addProcess(t, 201, 100, 201, 20, "/home/u/a", "/usr/bin/bash")
	proc.addProcess(t, 202, 100, -1, 30, "/home/u/helper", "/usr/bin/some-helper")

	want := map[string]Detail{"0xA": {Cwd: "/home/u/a"}}

	if diff := cmp.Diff(want, inspect(t, []hypr.Client{window("0xA", "1", 100)}, proc.root, true)); diff != "" {
		t.Errorf("Inspect() mismatch (-want +got):\n%s", diff)
	}
}

func TestGhosttyLaunch(t *testing.T) {
	const saved = "/usr/bin/ghostty"

	tests := []struct {
		name   string
		argv   []string
		detail Detail
		want   []string
	}{
		{
			name:   "reopens through the running ghostty, in the saved directory",
			argv:   []string{saved, "--gtk-single-instance=true", "--font-size=9"},
			detail: Detail{Cwd: "/home/u/work"},
			want:   []string{saved, "+new-window", "--working-directory=/home/u/work"},
		},
		{
			name:   "the program that was running is not passed on",
			argv:   []string{saved},
			detail: Detail{Cwd: "/home/u/work", Program: []string{"btop"}},
			want:   []string{saved, "+new-window", "--working-directory=/home/u/work"},
		},
		{
			name:   "a custom class is kept, because it chooses the instance",
			argv:   []string{saved, "--class=com.example.term", "--font-size=9"},
			detail: Detail{Cwd: "/home/u/work"},
			want: []string{saved, "+new-window", "--class=com.example.term",
				"--working-directory=/home/u/work"},
		},
		{
			name:   "a window with no detail still opens a new window",
			argv:   []string{saved, "--gtk-single-instance=true", "--initial-window=false"},
			detail: Detail{},
			want:   []string{saved, "+new-window"},
		},
		{
			name:   "a ghostty started with -e is replayed as is",
			argv:   []string{saved, "--gtk-single-instance=true", "-e", "ncspot"},
			detail: Detail{},
			want:   []string{saved, "--gtk-single-instance=true", "-e", "ncspot"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, ghostty{}.Launch(tt.argv, tt.detail)); diff != "" {
				t.Errorf("Launch() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLaunchLeavesUnknownClassesAlone(t *testing.T) {
	argv := []string{"/usr/lib/firefox/firefox"}

	got := Launch("firefox", argv, Detail{Cwd: "/home/u"})

	if diff := cmp.Diff(argv, got); diff != "" {
		t.Errorf("Launch() mismatch (-want +got):\n%s", diff)
	}
}

func TestInspectIgnoresClassesNoAppOwns(t *testing.T) {
	proc := newFakeProc(t)
	proc.addProcess(t, 100, 1, -1, 10, "", "/usr/bin/foot")

	windows := []hypr.Client{{Address: "0xA", Class: "foot", PID: 100, StableID: "1"}}

	if got, _ := Inspect(windows, proc.root); len(got) != 0 {
		t.Errorf("Inspect() = %v, want nothing for a class no app owns", got)
	}
}
