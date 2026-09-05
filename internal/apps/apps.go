// Package apps looks inside the windows of the applications we know something
// about, and builds the commands that reopen them.
//
// An app's argv is often all a snapshot needs, but not always: a terminal
// launched from a menu reports the same argv whatever is running inside it, and
// under a single instance every one of its windows reports the same pid too. An
// App knows where the rest of the story is kept for one class of window.
package apps

import (
	"maps"

	"github.com/FillipdotS/hyprresurrect/internal/hypr"
)

// A Detail is what a snapshot records about the inside of a window, over and
// above the argv of the process serving it.
type Detail struct {
	Cwd     string
	Program []string
}

// An App handles one class of window. Inspect takes every window of that class
// at once rather than one at a time, because pairing the windows of a single
// instance with the processes behind them needs all of them together.
type App interface {
	Owns(class string) bool
	Inspect(windows []hypr.Client, procRoot string) map[string]Detail
	Launch(argv []string, detail Detail) []string
}

var registry = []App{ghostty{}}

// Inspect returns what is going on inside every window an app knows how to look
// into, keyed by window address. Windows of any other class are left out, which
// is not a failure: most apps say everything they need to in their argv.
func Inspect(clients []hypr.Client, procRoot string) map[string]Detail {
	details := make(map[string]Detail)

	for _, app := range registry {
		var owned []hypr.Client

		for _, c := range clients {
			if app.Owns(c.Class) {
				owned = append(owned, c)
			}
		}

		if len(owned) > 0 {
			maps.Copy(details, app.Inspect(owned, procRoot))
		}
	}

	return details
}

// Owns reports whether any app can look inside this class of window, and so
// whether there is anything to wait for before reading one back.
func Owns(class string) bool {
	for _, app := range registry {
		if app.Owns(class) {
			return true
		}
	}

	return false
}

// Launch returns the argv that reopens a window of class, given what was
// captured about it. Unchanged argv when no app owns the class.
func Launch(class string, argv []string, detail Detail) []string {
	for _, app := range registry {
		if app.Owns(class) {
			return app.Launch(argv, detail)
		}
	}

	return argv
}
