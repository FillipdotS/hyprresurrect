# hyprresurrect

hyprresurrect saves your currently open applications in [hyprland](https://hypr.land/) and restores them on reboot (or at any point)

## Why?

I started using hyprland. I wanted a way to restore my tiles and layout. I tried a few existing tools but wasn't happy, some would dump everything onto one workspace, or not care about multiple monitors, etc. So the natural course of action was to build my own :)

If you're reading this then this is still WIP! But mostly done. Just need the daemon, group support, and persisting tile size/layout

## Installation

### AUR

```
not yet! sorry!
```

### Go

Requires Go 1.27+

```sh
go install github.com/FillipdotS/hyprresurrect@latest
```

## Usage

```sh
hyprresurrect save              # snapshot the current windows/layout
hyprresurrect restore           # restore the most recent snapshot
hyprresurrect restore --dry-run # prints the restore plan without actually executing it 
hyprresurrect version           # prints the installed version
```

## Contributing

Feel free to contribute anything and everything! Just make sure it's tested and makes sense for this tool. Preferably make an issue beforehand so there's alignment.

### Testing

Regular unit tests can be run via `go test`

#### E2E Tests

These require you to be running hyprland. They spawn a nested hyprland session that uses the real `go build` binary to run commands and checks via `hyprctl clients -j` if we got the intended results

```sh
HR_E2E=1 go test ./internal/e2e/
```

The nested session shows the name of the running test. To actually watch one go
by, slow every step down:

```sh
HR_E2E=1 HR_E2E_SLOW=500ms go test ./internal/e2e/ -run TestRoundTrip
```

## Todo (Roughly in order)

- [ ] **E2E testing hyprland via a nested session**

- [ ] **Autosave daemon**

- [ ] Auto restore on reboot

- [ ] **Groups.** Grouped windows come back as separate tiles

- [ ] **Terminal contents.** i.e. "btop" or "herdr"

- [ ] **Remember tile sizes and layout.**

- [ ] **Named workspaces.** Only numeric workspace ids are captured. Not sure this is needed

- [ ] **Flatpak support.**
