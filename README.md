# hyprresurrect

## Roadmap

- [ ] **Chromium argv.** Chromium and Electron overwrite their own argv area to
      set the process title, so `/proc/<pid>/cmdline` comes back as a single
      element holding the entire command line instead of NUL-separated
      arguments. Shell-quoting turns that into one impossible filename and the
      window never comes back at all. Detect the shape - exactly one element,
      containing spaces, whose first token resolves to an executable - and
      re-split it on whitespace. See
      [`internal/snapshot/proc.go`](internal/snapshot/proc.go).

- [ ] **Flatpak support.** A flatpak app runs in its own mount namespace, so
      `/proc/<pid>/cmdline` reports an in-sandbox path (`/app/bin/foo`) that
      doesn't exist on the host and can't be relaunched. Detect the sandbox via
      `/proc/<pid>/root/.flatpak-info` and use `flatpak run <app-id>` from its
      `[Application] name=` key. See
      [`internal/snapshot/proc.go`](internal/snapshot/proc.go).

- [ ] **Terminal contents.** A terminal's own argv says nothing about what's
      running inside it: seven ghostty windows share one pid and all report the
      same command, so a terminal that had `cliamp` playing comes back as a bare
      shell. The shell child of each pty knows its cwd and its foreground
      program, so the windows can come back as `nvim`, `btop` and friends
      instead. See [`internal/snapshot/proc.go`](internal/snapshot/proc.go).

- [ ] **Tiled positions.** Windows come back on the right workspace and monitor,
      but their arrangement within a workspace is whatever the layout produces
      from the spawn order - two windows land 50/50 regardless of the split you
      had, and left/right may swap. A dwindle workspace is a binary space
      partition, so the split tree is recoverable from the saved rectangles:
      find the line that cleanly separates the windows into two groups, recurse.
      Replay it with `hl.dsp.layout("dwindle", "preselect <dir>")` before each
      spawn. Layout-specific; master would need its own logic.

- [ ] **Remembered sizes.** Follows from the above - split ratios are captured
      in `at`/`size` but discarded, so every tile comes back at whatever size
      the layout chooses. Needs `splitratio` alongside the tree reconstruction.
      Floating windows already keep their exact geometry.

- [ ] **Groups.** Grouped and tabbed windows come back as separate windows.
      `hl.dsp.group.*` exists, and `grouped` is in `hyprctl clients -j`, but
      neither is captured or replayed yet.

- [ ] **Named workspaces.** Only numeric workspace ids are captured.

- [ ] **Fullscreen state.** Fullscreen windows come back windowed. Restoring it
      needs the `fullscreen_state` dispatcher, which needs a live window
      address — the one thing the rule-at-spawn restore otherwise never has to
      look up.

- [ ] **Focus.** Hyprland leaves focus unset after a restore; click a window.

## License

MIT — see [LICENSE](LICENSE).
