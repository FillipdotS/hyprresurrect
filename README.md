# hyprresurrect

hyprresurrect saves your currently open applications in [Hyprland](https://hypr.land/) and restores them on reboot (or at any point). Workspaces, groups, tile sizes, layout are all preserved.

## Why?

I started using hyprland. I wanted a way to restore my tiles and layout. I tried a few existing tools but wasn't happy, some would dump everything onto one workspace, or not care about multiple monitors, etc. So the natural course of action was to build my own :)

If you're reading this then this is still WIP! But mostly done. Just need the daemon, ✅~~group support~~, ✅~~persisting tile size/layout~~, ✅~~terminal contents~~

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

## Terminal contents

A terminal launched from a menu reports the same command whatever is running
inside it, and ghostty's single instance serves every one of its windows from
one process — so seven windows report one identical command between them. What
tells them apart is the shell behind each window, with its own directory and its
own foreground program, found by pairing the windows in the order hyprland
created them with the shells in the order they started.

A restore reopens each terminal **in the directory it was in**, through
ghostty's own `+new-window`, so the windows come back on the one instance just
as they were. Nothing is started for you: what was running is recorded, and
`restore --dry-run` names it, but putting it back is your call.

```
$ hyprresurrect restore --dry-run
...
-- these were running, and are not started again:
   workspace 2: btop
   workspace 3: herdr
```

Two things are worth knowing. Ghostty splits and tabs are not covered — they put
several shells behind one window, and rather than guess which one the window
should reopen with, hyprresurrect records nothing for that window and restores
it as it always did. And an instance that is running but has lost every window
will accept the request to open one and then not open it; a restore says so
rather than reporting success.

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

- ✅ **E2E testing hyprland via a nested session**

- ⬜ **Autosave daemon**

- ⬜ Auto restore on reboot

- ✅ **Groups.** Membership, tab order and the raised tab all round trip

- ✅ **Terminal contents.** i.e. "btop" or "herdr"

- ✅ **Remember tile sizes and layout.**

- ⬜ **Named workspaces.** Only numeric workspace ids are captured. Not sure this is needed

- ⬜ **Flatpak support.**
