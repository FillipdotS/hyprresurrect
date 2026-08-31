# hyprresurrect

hyprresurrect saves your currently open applications and restore them on reboot (or at any point)

## Why?

I started using hyprland. I wanted a way to restore my tiles and layout. I tried a few existing tools but wasn't happy, some would dump everything onto one workspace, or not care about multiple monitors, etc.

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

- [ ] **Autosave daemon**

- [ ] **Groups.** Grouped windows come back as separate tiles

- [ ] **Terminal contents.** i.e. "btop" or "herdr"

- [ ] **Remember tile sizes and layout.**

- [ ] **Named workspaces.** Only numeric workspace ids are captured. Not sure this is needed

- [ ] **Flatpak support.**
