package hypr

type Client struct {
	Class     string       `json:"class"`
	Workspace WorkspaceRef `json:"workspace"`
	MonitorID int          `json:"monitor"`
	At        [2]int       `json:"at"`
	Size      [2]int       `json:"size"`
	Floating  bool         `json:"floating"`
	PID       int          `json:"pid"`
}

type WorkspaceRef struct {
	ID int `json:"id"`
}

type Monitor struct {
	ID              int          `json:"id"`
	Name            string       `json:"name"`
	ActiveWorkspace WorkspaceRef `json:"activeWorkspace"`
}
