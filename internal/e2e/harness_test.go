// Package e2e drives hyprresurrect against a real Hyprland, nested inside the
// live session as an ordinary wayland client. Off by default
// run via `HR_E2E=1 go test ./internal/e2e/`.
package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
)

const (
	startTimeout = 10 * time.Second
	waitTimeout  = 5 * time.Second

	// announceFor just has to outlast the suite: each test replaces the banner.
	announceFor = 30 * time.Minute
)

// slow lingers after each step so a run can be watched: HR_E2E_SLOW=500ms.
var slow, _ = time.ParseDuration(os.Getenv("HR_E2E_SLOW"))

func pause() {
	time.Sleep(slow)
}

// nested is the compositor under test, ready by the time any test runs.
var nested *compositor

type compositor struct {
	cmd     *exec.Cmd
	dir     string        // its XDG_RUNTIME_DIR
	config  string        // an empty XDG_CONFIG_HOME for the windows we spawn
	sock    string        // .socket.sock, once it exists
	display string        // the wayland socket it serves, for the windows we spawn
	exited  chan struct{} // closed once cmd has been reaped
}

func TestMain(m *testing.M) {
	if os.Getenv("HR_E2E") == "" {
		fmt.Println("skipping e2e: set HR_E2E=1 to run against a nested Hyprland")

		return
	}

	os.Exit(run(m))
}

func run(m *testing.M) int {
	// Deliberately not t.TempDir(): unix socket paths cap at 108 bytes
	dir, err := os.MkdirTemp("", "hr-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: runtime dir: %v\n", err)

		return 1
	}
	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: leaving %s behind: %v\n", dir, err)
		}
	}
	defer cleanup()

	if binary, err = build(dir); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		return 1
	}

	c, err := start(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		return 1
	}
	defer c.stop()

	// Ctrl-C would otherwise leave the compositor running
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interrupted
		c.stop()
		cleanup()
		os.Exit(1)
	}()

	if err := c.await(startTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)

		return 1
	}

	if c.signature() == os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") {
		fmt.Fprintf(os.Stderr, "e2e: resolved the live session (%s); refusing to run\n", c.signature())

		return 1
	}

	nested = c

	return m.Run()
}

// setup names the test in the nested session and takes its windows down again.
func setup(t *testing.T) cli {
	t.Helper()

	nested.Announce(t)
	t.Cleanup(func() { nested.CloseAllWindows(t) })

	return nested.CLI(t)
}

func baseEnv(runtimeDir string, extra ...string) []string {
	return append([]string{
		"HOME=" + os.Getenv("HOME"),
		"PATH=" + os.Getenv("PATH"),
		"XDG_RUNTIME_DIR=" + runtimeDir,
	}, extra...)
}

func start(dir string) (*compositor, error) {
	conf, err := filepath.Abs("test.lua")
	if err != nil {
		return nil, fmt.Errorf("test.lua: %w", err)
	}

	host, err := hostDisplayPath()
	if err != nil {
		return nil, err
	}

	config := filepath.Join(dir, "config")
	if err := os.Mkdir(config, 0o700); err != nil {
		return nil, fmt.Errorf("config dir: %w", err)
	}

	log, err := os.Create(filepath.Join(dir, "hyprland.out"))
	if err != nil {
		return nil, fmt.Errorf("log: %w", err)
	}

	cmd := exec.Command("hyprland", "--config", conf)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.Env = baseEnv(dir,
		// Absolute: XDG_RUNTIME_DIR now points at the nested instance's own
		// directory, which is not where the host's socket lives.
		"WAYLAND_DISPLAY="+host,
	)
	// Own process group: the windows the tests spawn are Hyprland's children,
	// and stop() has to take the whole tree with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start hyprland: %w", err)
	}

	c := &compositor{cmd: cmd, dir: dir, config: config, exited: make(chan struct{})}

	go func() {
		_ = cmd.Wait()
		close(c.exited)
	}()

	return c, nil
}

// await blocks until the compositor is ready to be driven. A launch that fails
// dies in well under a second, so watch for that rather than waiting out the
// whole deadline for an answer we already have.
func (c *compositor) await(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	if err := c.poll(deadline, "socket", func() bool {
		socks, _ := filepath.Glob(filepath.Join(c.dir, "hypr", "*", ".socket.sock"))
		if len(socks) == 0 {
			return false
		}

		c.sock = socks[0]

		return true
	}); err != nil {
		return err
	}

	if err := c.resolveDisplay(); err != nil {
		return err
	}

	// The socket answers before the compositor has a monitor, and a workspace
	// dispatch against that gap fails with "Bad workspace".
	return c.poll(deadline, "monitor", func() bool {
		monitors, err := c.Socket().Monitors()

		return err == nil && len(monitors) > 0
	})
}

// poll runs ready until it holds, the compositor dies, or the deadline passes.
func (c *compositor) poll(deadline time.Time, what string, ready func() bool) error {
	for {
		if ready() {
			return nil
		}

		select {
		case <-c.exited:
			return fmt.Errorf("compositor exited during startup, waiting for %s%s", what, c.diagnostics())
		default:
		}

		if !time.Now().Before(deadline) {
			return fmt.Errorf("no %s before the deadline%s", what, c.diagnostics())
		}

		time.Sleep(50 * time.Millisecond)
	}
}

// signature is the nested instance's HYPRLAND_INSTANCE_SIGNATURE: hyprland
// names the socket's directory after it.
func (c *compositor) signature() string {
	if c.sock == "" {
		return ""
	}

	return filepath.Base(filepath.Dir(c.sock))
}

// diagnostics is whatever the compositor managed to say before giving up: our
// captured stdout, plus hyprland's own log if it got far enough to open one.
func (c *compositor) diagnostics() string {
	var b strings.Builder

	for _, pattern := range []string{
		filepath.Join(c.dir, "hyprland.out"),
		filepath.Join(c.dir, "hypr", "*", "hyprland.log"),
	} {
		paths, _ := filepath.Glob(pattern)

		for _, path := range paths {
			fmt.Fprintf(&b, "\n--- %s\n%s", path, tail(path, 25))
		}
	}

	return b.String()
}

func tail(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(unreadable: %v)\n", err)
	}

	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return strings.Join(lines, "\n") + "\n"
}

// stop takes down the compositor and every window it spawned. Hyprland does
// not reliably die on SIGTERM -- one sent early in its startup is ignored
// outright -- so escalate, and confirm rather than assume.
func (c *compositor) stop() {
	pgid := c.cmd.Process.Pid

	for _, sig := range []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL} {
		if err := syscall.Kill(-pgid, sig); err != nil {
			return // ESRCH: the group is already gone
		}

		if processGone(pgid, 5*time.Second) {
			return
		}
	}

	fmt.Fprintf(os.Stderr, "e2e: nested compositor (pgid %d) survived SIGKILL\n", pgid)
}

// processGone reports whether every process in the group has exited.
func processGone(pgid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if err := syscall.Kill(-pgid, 0); err != nil {
			return true
		}

		time.Sleep(50 * time.Millisecond)
	}

	return false
}

// hostDisplayPath returns an absolute path to the live session's wayland socket,
// which is what the nested instance connects to.
func hostDisplayPath() (string, error) {
	display := os.Getenv("WAYLAND_DISPLAY")
	if display == "" {
		return "", errors.New("WAYLAND_DISPLAY is not set; the e2e suite nests inside a running wayland session")
	}

	if !filepath.IsAbs(display) {
		dir := os.Getenv("XDG_RUNTIME_DIR")
		if dir == "" {
			return "", errors.New("XDG_RUNTIME_DIR is not set; cannot find the host wayland socket")
		}

		display = filepath.Join(dir, display)
	}

	if _, err := os.Stat(display); err != nil {
		return "", fmt.Errorf("host wayland socket: %w", err)
	}

	return display, nil
}

// resolveDisplay finds the wayland socket the nested instance serves, which is
// what its clients connect to - not to be confused with c.sock, hyprland's own
// IPC socket.
func (c *compositor) resolveDisplay() error {
	names, err := filepath.Glob(filepath.Join(c.dir, "wayland-*"))
	if err != nil {
		return fmt.Errorf("looking for the wayland socket: %w", err)
	}

	for _, name := range names {
		if !strings.HasSuffix(name, ".lock") {
			c.display = filepath.Base(name)

			return nil
		}
	}

	return fmt.Errorf("no wayland socket in %s", c.dir)
}

// Socket returns a hypr for the nested instance
func (c *compositor) Socket() *hypr.Socket {
	return hypr.New(c.sock)
}

func (c *compositor) Spawn(t *testing.T, class string, args ...string) hypr.Client {
	t.Helper()

	return c.SpawnTitled(t, class, "", args...)
}

func (c *compositor) SpawnTitled(t *testing.T, class, title string, args ...string) hypr.Client {
	t.Helper()

	existing := make(map[string]bool)
	for _, client := range c.clients(t) {
		existing[client.Address] = true
	}

	if len(args) == 0 {
		args = []string{"sleep", "infinity"}
	}

	flags := []string{"--app-id=" + class}
	if title != "" {
		flags = append(flags, "--title="+title)
	}

	cmd := exec.Command("foot", append(append(flags, "-e"), args...)...)
	cmd.Env = c.clientEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		t.Fatalf("spawn %s: %v", class, err)
	}

	// Ours, not the compositor's: stop() would never reach it.
	t.Cleanup(func() {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		_ = cmd.Wait()
	})

	// By address, not by class: two windows of one class are the case the
	// single-instance tests are about, and counting classes would match the
	// wrong one.
	var spawned hypr.Client

	c.WaitFor(t, func(clients []hypr.Client) bool {
		for _, client := range clients {
			if client.Class == class && !existing[client.Address] {
				spawned = client

				return true
			}
		}

		return false
	})

	pause()

	return spawned
}

func (c *compositor) clients(t *testing.T) []hypr.Client {
	t.Helper()

	clients, err := c.Socket().Clients()
	if err != nil {
		t.Fatalf("clients: %v", err)
	}

	return clients
}

// WaitFor polls the client list until pred holds, the way Runner.settle does.
func (c *compositor) WaitFor(t *testing.T, pred func([]hypr.Client) bool) []hypr.Client {
	t.Helper()

	sock := c.Socket()
	deadline := time.Now().Add(waitTimeout)

	var (
		clients []hypr.Client
		err     error
	)

	for time.Now().Before(deadline) {
		if clients, err = sock.Clients(); err == nil && pred(clients) {
			return clients
		}

		time.Sleep(50 * time.Millisecond)
	}

	out := make([]string, 0, len(clients))
	for _, c := range clients {
		out = append(out, fmt.Sprintf("%s@ws%d", c.Class, c.Workspace.ID))
	}

	t.Fatalf("condition unmet after %v; clients %s, last error %v", waitTimeout, "["+strings.Join(out, " ")+"]", err)

	return nil
}

func (c *compositor) FocusWorkspace(t *testing.T, workspace int) {
	t.Helper()

	lua := fmt.Sprintf(`hl.dispatch(hl.dsp.focus({workspace = "%d"}))`, workspace)
	if err := c.Socket().Eval(lua); err != nil {
		t.Fatalf("focus workspace %d: %v", workspace, err)
	}

	pause()
}

// Hyprctl runs the real hyprctl against the nested instance
func (c *compositor) Hyprctl(t *testing.T, args ...string) string {
	t.Helper()

	cmd := exec.Command("hyprctl", args...)
	cmd.Env = baseEnv(c.dir, "HYPRLAND_INSTANCE_SIGNATURE="+c.signature())

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("hyprctl %s: %v\n%s", strings.Join(args, " "), err, out)
	}

	return string(out)
}

func (c *compositor) CloseAllWindows(t *testing.T) {
	t.Helper()

	sock := c.Socket()

	for _, client := range c.clients(t) {
		lua := fmt.Sprintf("hl.dispatch(hl.dsp.window.close({window = %q}))", "address:"+client.Address)
		if err := sock.Eval(lua); err != nil {
			t.Errorf("close %s: %v", client.Class, err)
		}
	}

	c.WaitFor(t, func(clients []hypr.Client) bool { return len(clients) == 0 })

	pause()
}

// Announce names the running test across the top of the nested session.
func (c *compositor) Announce(t *testing.T) {
	t.Helper()

	c.Eval(t, fmt.Sprintf(
		`if hrbanner then hrbanner:dismiss() end `+
			`hrbanner = hl.notification.create({text = %q, timeout = %d, font_size = 26})`,
		t.Name(), announceFor.Milliseconds()))

	pause()
}

func (c *compositor) clientEnv() []string {
	return baseEnv(c.dir,
		"WAYLAND_DISPLAY="+c.display,
		"XDG_CONFIG_HOME="+c.config,
		"XDG_CONFIG_DIRS="+c.config,
	)
}

func (c *compositor) Eval(t *testing.T, lua string) {
	t.Helper()

	if err := c.Socket().Eval(lua); err != nil {
		t.Fatalf("eval %s: %v", lua, err)
	}
}

func (c *compositor) FocusMonitor(t *testing.T, monitor string) {
	t.Helper()

	c.Eval(t, fmt.Sprintf(`hl.dispatch(hl.dsp.focus({monitor = %q}))`, monitor))

	pause()
}

func (c *compositor) Monitor(t *testing.T, id int) monitor {
	t.Helper()

	for _, m := range hyprctlMonitors(t) {
		if m.ID == id {
			return m
		}
	}

	t.Fatalf("no monitor %d; have %v", id, hyprctlMonitors(t))

	return monitor{}
}

func (c *compositor) AddHeadlessMonitor(t *testing.T) monitor {
	t.Helper()

	existing := make(map[int]bool)
	for _, m := range hyprctlMonitors(t) {
		existing[m.ID] = true
	}

	c.Hyprctl(t, "output", "create", "headless")

	var added monitor

	deadline := time.Now().Add(startTimeout)
	for time.Now().Before(deadline) && added.Name == "" {
		for _, m := range hyprctlMonitors(t) {
			if !existing[m.ID] {
				added = m

				break
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	if added.Name == "" {
		t.Fatalf("no new monitor after %v; have %v", startTimeout, hyprctlMonitors(t))
	}

	t.Cleanup(func() { c.Hyprctl(t, "output", "remove", added.Name) })

	return added
}
