# hyprresurrect

## Roadmap

- [ ] **Flatpak support.** A flatpak app runs in its own mount namespace, so
      `/proc/<pid>/cmdline` reports an in-sandbox path (`/app/bin/foo`) that
      doesn't exist on the host and can't be relaunched. Detect the sandbox via
      `/proc/<pid>/root/.flatpak-info` and use `flatpak run <app-id>` from its
      `[Application] name=` key. See
      [`internal/capture/resolver.go`](internal/capture/resolver.go).

- [ ] **Terminal contents.** A terminal's own argv says nothing about what's
      running inside it: seven ghostty windows share one pid and all report the
      same command. The shell child of each pty knows its cwd and its foreground
      program, so the windows can come back as `nvim`, `btop` and friends rather
      than as bare shells. See
      [`internal/capture/resolver.go`](internal/capture/resolver.go).

- [ ] **Named workspaces.** Only numeric workspace ids are captured.

## License

MIT — see [LICENSE](LICENSE).

- [ ] **Fullscreen state.** Fullscreen windows come back windowed.
