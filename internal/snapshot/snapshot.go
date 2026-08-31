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

	for _, c := range clients {
		cmd, err := command(procRoot, c.PID)
		if err != nil {
			// TOOD: Log somewhere what we failed to capture
			continue
		}

		snap.Windows = append(snap.Windows, Window{
			Class:     c.Class,
			Workspace: c.Workspace.ID,
			Monitor:   monitorNames[c.MonitorID],
			At:        c.At,
			Size:      c.Size,
			Floating:  c.Floating,
			Command:   cmd,
		})
	}

	return snap, nil
}
