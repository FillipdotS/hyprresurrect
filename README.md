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

## Todo (Roughly in order)

- [ ] **Integration testing hyprland via a nested session**

- [ ] **Autosave daemon**

- [ ] Auto restore on reboot

- [ ] **Groups.** Grouped windows come back as separate tiles

- [ ] **Terminal contents.** i.e. "btop" or "herdr"

- [ ] **Remember tile sizes and layout.**

- [ ] **Named workspaces.** Only numeric workspace ids are captured. Not sure this is needed

- [ ] **Flatpak support.**
