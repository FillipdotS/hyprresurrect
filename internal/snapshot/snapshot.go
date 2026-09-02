// Package snapshot builds the record of a session that restore later replays.
package snapshot

import (
	"fmt"
	"time"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
)

const Version = 1

type Snapshot struct {
	Version    int       `json:"version"`
	CapturedAt time.Time `json:"capturedAt"`
	Monitors   []Monitor `json:"monitors"`
	Windows    []Window  `json:"windows"`
}

type Monitor struct {
	Name            string `json:"name"`
	ActiveWorkspace int    `json:"activeWorkspace"`
}

type Window struct {
	Class     string   `json:"class"`
	Workspace int      `json:"workspace"`
	Monitor   string   `json:"monitor"`
	At        [2]int   `json:"at"`
	Size      [2]int   `json:"size"`
	Floating  bool     `json:"floating"`
	Command   []string `json:"command"` // possibly empty

	Group       int  `json:"group,omitempty"`       // from 1, 0 ungrouped; members follow in tab order
	GroupActive bool `json:"groupActive,omitempty"` // was the raised tab
}

type source interface {
	Clients() ([]hypr.Client, error)
	Monitors() ([]hypr.Monitor, error)
}

// Capture reads the current session and turns it into a Snapshot.
func Capture(src source) (Snapshot, error) {
	return capture(src, "/proc")
}

func capture(src source, procRoot string) (Snapshot, error) {
	monitors, err := src.Monitors()
	if err != nil {
		return Snapshot{}, fmt.Errorf("monitors: %w", err)
	}

	clients, err := src.Clients()
	if err != nil {
		return Snapshot{}, fmt.Errorf("clients: %w", err)
	}

	snap := Snapshot{
		Version:    Version,
		CapturedAt: time.Now(),
		Monitors:   make([]Monitor, 0, len(monitors)),
		Windows:    make([]Window, 0, len(clients)),
	}

	monitorNames := make(map[int]string, len(monitors))
	for _, m := range monitors {
		monitorNames[m.ID] = m.Name

		snap.Monitors = append(snap.Monitors, Monitor{
			Name:            m.Name,
			ActiveWorkspace: m.ActiveWorkspace.ID,
		})
	}

	ordered, groups := groupOrder(clients)

	for _, c := range ordered {
		cmd, err := command(procRoot, c.PID)
		if err != nil {
			// TOOD: Log somewhere what we failed to capture
			continue
		}

		w := Window{
			Class:     c.Class,
			Workspace: c.Workspace.ID,
			Monitor:   monitorNames[c.MonitorID],
			At:        c.At,
			Size:      c.Size,
			Floating:  c.Floating,
			Command:   cmd,
		}

		if id := groups[c.Address]; id > 0 {
			w.Group = id
			w.GroupActive = c.Visible
		}

		snap.Windows = append(snap.Windows, w)
	}

	return snap, nil
}

// groupOrder reorders clients so that the members of a group sit together and
// in tab order, and numbers the groups. Hyprland lists a group's members
// wherever they happen to fall in the client list; keeping them contiguous is
// what lets the snapshot carry tab order in its own window order.
func groupOrder(clients []hypr.Client) ([]hypr.Client, map[string]int) {
	// By index rather than by value: two windows can only share an address if
	// something upstream is broken, and dropping one of them silently is worse
	// than emitting both.
	index := make(map[string]int, len(clients))
	for i, c := range clients {
		if _, dup := index[c.Address]; !dup {
			index[c.Address] = i
		}
	}

	var (
		ordered = make([]hypr.Client, 0, len(clients))
		groups  = make(map[string]int)
		placed  = make([]bool, len(clients))
		next    = 1
	)

	place := func(i, group int) {
		placed[i] = true

		if group > 0 {
			groups[clients[i].Address] = group
		}

		ordered = append(ordered, clients[i])
	}

	for i, c := range clients {
		if placed[i] {
			continue
		}

		if len(c.Grouped) == 0 {
			place(i, 0)

			continue
		}

		for _, address := range c.Grouped {
			if member, ok := index[address]; ok && !placed[member] {
				place(member, next)
			}
		}

		// A group that doesn't name its own member back is malformed, but the
		// window still has to make it into the snapshot.
		if !placed[i] {
			place(i, next)
		}

		next++
	}

	return ordered, groups
}
