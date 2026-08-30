package hypr

type Client struct {
	Address          string       `json:"address"`
	Class            string       `json:"class"`
	InitialClass     string       `json:"initialClass"`
	Title            string       `json:"title"`
	InitialTitle     string       `json:"initialTitle"`
	Workspace        WorkspaceRef `json:"workspace"`
	MonitorID        int          `json:"monitor"`
	At               [2]int       `json:"at"`
	Size             [2]int       `json:"size"`
	Floating         bool         `json:"floating"`
	Fullscreen       int          `json:"fullscreen"`
	FullscreenClient int          `json:"fullscreenClient"`
	PID              int          `json:"pid"`
	StableID         string       `json:"stableId"`
	XDGTag           string       `json:"xdgTag"`
}

type WorkspaceRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Workspace struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Monitor    string `json:"monitor"`
	MonitorID  int    `json:"monitorID"`
	Windows    int    `json:"windows"`
	Persistent bool   `json:"ispersistent"`
}

type Monitor struct {
	ID              int          `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	ActiveWorkspace WorkspaceRef `json:"activeWorkspace"`
}
