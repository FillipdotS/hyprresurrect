package snapshot

import (
	"github.com/google/go-cmp/cmp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakeProc builds a procfs-shaped tree under a temp dir. Real cmdline files are
// NUL separated, which is why these are generated rather than committed to
// testdata.
type fakeProc struct{ root string }

func newFakeProc(t *testing.T) *fakeProc {
	t.Helper()

	return &fakeProc{root: t.TempDir()}
}

func (p *fakeProc) addProcess(t *testing.T, pid int, ppid, uid int, argv ...string) {
	t.Helper()

	dir := filepath.Join(p.root, strconv.Itoa(pid))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	// Trailing NUL, as the kernel writes it.
	cmdline := ""
	if len(argv) > 0 {
		cmdline = strings.Join(argv, "\x00") + "\x00"
	}
	p.write(t, filepath.Join(dir, "cmdline"), cmdline)

	status := "Name:\tfake\nPid:\t" + strconv.Itoa(pid) +
		"\nPPid:\t" + strconv.Itoa(ppid) +
		"\nUid:\t" + strconv.Itoa(uid) + "\t" + strconv.Itoa(uid) + "\t" + strconv.Itoa(uid) + "\t" + strconv.Itoa(uid) + "\n"
	p.write(t, filepath.Join(dir, "status"), status)
}

// setExe points /proc/<pid>/exe at target, as the kernel does. Unlike cmdline,
// a process cannot rewrite it.
func (p *fakeProc) setExe(t *testing.T, pid int, target string) {
	t.Helper()

	if err := os.Symlink(target, filepath.Join(p.root, strconv.Itoa(pid), "exe")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
}

func (p *fakeProc) write(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestCommand(t *testing.T) {
	const uid = 1000

	proc := newFakeProc(t)
	proc.addProcess(t, 1000, 1, uid, "foot")
	proc.addProcess(t, 2000, 1, uid, "/bin/python3", "/usr/bin/lutris")
	proc.addProcess(t, 3000, 1, uid, "/usr/lib/chromium/chromium", "--ozone-platform=wayland", "--app=https://discord.com/channels/@me")
	proc.addProcess(t, 3001, 3000, uid, "/usr/lib/chromium/chromium", "--type=renderer", "--channel-token=abc")
	proc.addProcess(t, 5000, 1, uid) // zombie: exists, empty cmdline

	// A helper whose parent belongs to another user: climbing would leave the
	// app entirely, so resolution has to fail instead.
	proc.addProcess(t, 6000, 6001, uid, "/usr/lib/chromium/chromium", "--type=gpu-process")
	proc.addProcess(t, 6001, 1, 0, "/usr/lib/systemd/systemd", "--user")

	// Electron overwrites its own argv area to set the process title, so the
	// whole command line arrives as one element. Shape taken from a real
	// discord capture.
	const discord = "/home/u/.config/discord/app-1.0.155/Discord"
	proc.addProcess(t, 7000, 1, uid, discord+" --url --")
	proc.setExe(t, 7000, discord)

	// A single argument that genuinely contains spaces must survive intact.
	const spaced = "/home/u/My Apps/thing"
	proc.addProcess(t, 7001, 1, uid, spaced)
	proc.setExe(t, 7001, spaced)

	// A mangled helper hides its --type= behind the same blob, so unmangling is
	// also what lets the climb out of it work.
	proc.addProcess(t, 7002, 7003, uid, "/usr/lib/electron/electron --type=renderer")
	proc.setExe(t, 7002, "/usr/lib/electron/electron")
	proc.addProcess(t, 7003, 1, uid, "/usr/lib/electron/electron", "--app=/opt/thing")

	tests := []struct {
		name    string
		pid     int
		want    []string
		wantErr bool
	}{
		{
			name: "plain binary",
			pid:  1000,
			want: []string{"foot"},
		},
		{
			name: "interpreter keeps both args",
			pid:  2000,
			want: []string{"/bin/python3", "/usr/bin/lutris"},
		},
		{
			name: "browser process used as is",
			pid:  3000,
			want: []string{"/usr/lib/chromium/chromium", "--ozone-platform=wayland", "--app=https://discord.com/channels/@me"},
		},
		{
			name: "renderer climbs to its browser",
			pid:  3001,
			want: []string{"/usr/lib/chromium/chromium", "--ozone-platform=wayland", "--app=https://discord.com/channels/@me"},
		},
		{
			name:    "dead pid",
			pid:     9999,
			wantErr: true,
		},
		{
			name:    "empty cmdline",
			pid:     5000,
			wantErr: true,
		},
		{
			name:    "helper whose parent is another user",
			pid:     6000,
			wantErr: true,
		},
		{
			name: "electron blob is split at the real binary",
			pid:  7000,
			want: []string{"/home/u/.config/discord/app-1.0.155/Discord", "--url", "--"},
		},
		{
			name: "a genuine path with spaces is left whole",
			pid:  7001,
			want: []string{"/home/u/My Apps/thing"},
		},
		{
			name: "mangled helper still climbs to its app",
			pid:  7002,
			want: []string{"/usr/lib/electron/electron", "--app=/opt/thing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := command(proc.root, tt.pid)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("command() error = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("command() error = %v", err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("command() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
