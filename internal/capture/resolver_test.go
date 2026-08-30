package capture

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
	"github.com/google/go-cmp/cmp"
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

	r := &Resolver{ProcRoot: proc.root}

	tests := []struct {
		name   string
		client hypr.Client
		want   []string
		errIs  error
	}{
		{
			name:   "plain binary",
			client: hypr.Client{PID: 1000, Class: "foot"},
			want:   []string{"foot"},
		},
		{
			name:   "interpreter keeps both args",
			client: hypr.Client{PID: 2000, Class: "net.lutris.Lutris"},
			want:   []string{"/bin/python3", "/usr/bin/lutris"},
		},
		{
			name:   "browser process used as is",
			client: hypr.Client{PID: 3000, Class: "chromium"},
			want:   []string{"/usr/lib/chromium/chromium", "--ozone-platform=wayland", "--app=https://discord.com/channels/@me"},
		},
		{
			name:   "renderer climbs to its browser",
			client: hypr.Client{PID: 3001, Class: "chromium"},
			want:   []string{"/usr/lib/chromium/chromium", "--ozone-platform=wayland", "--app=https://discord.com/channels/@me"},
		},
		{
			name:   "dead pid",
			client: hypr.Client{PID: 9999, Class: "foot"},
			errIs:  ErrProcessGone,
		},
		{
			name:   "empty cmdline",
			client: hypr.Client{PID: 5000, Class: "foot"},
			errIs:  ErrNoCommand,
		},
		{
			name:   "helper whose parent is another user",
			client: hypr.Client{PID: 6000, Class: "chromium"},
			errIs:  ErrNoCommand,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.Command(tt.client)

			if tt.errIs != nil {
				if !errors.Is(err, tt.errIs) {
					t.Fatalf("Command() error = %v, want %v", err, tt.errIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("Command() error = %v", err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Command() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
