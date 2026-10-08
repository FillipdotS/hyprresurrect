// Package apps reads what is going on inside the windows of apps whose argv
// alone can't tell their windows apart, and builds the commands that reopen them.
package apps

import (
	"maps"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
)

type Detail struct {
	Cwd     string
	Program []string
}

// Inspect takes every window of the app's class at once, because pairing the
// windows of a single instance with its processes needs all of them together.
// ready is false while a window's details may still appear.
type App interface {
	Owns(class string) bool
	Inspect(windows []hypr.Client, procRoot string) (details map[string]Detail, ready bool)
	Launch(argv []string, detail Detail) []string
}

var registry = []App{ghostty{}}

// Inspect returns details keyed by window address.
func Inspect(clients []hypr.Client, procRoot string) (map[string]Detail, bool) {
	details := make(map[string]Detail)
	ready := true

	for _, app := range registry {
		var owned []hypr.Client

		for _, c := range clients {
			if app.Owns(c.Class) {
				owned = append(owned, c)
			}
		}

		if len(owned) > 0 {
			d, ok := app.Inspect(owned, procRoot)
			maps.Copy(details, d)
			ready = ready && ok
		}
	}

	return details, ready
}

func Launch(class string, argv []string, detail Detail) []string {
	for _, app := range registry {
		if app.Owns(class) {
			return app.Launch(argv, detail)
		}
	}

	return argv
}
