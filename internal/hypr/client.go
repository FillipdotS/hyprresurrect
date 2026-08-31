// Package hypr deals with connecting to the hyprland UNIX socket
package hypr

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type Socket struct {
	addr *net.UnixAddr
}

var replyErrorPrefixes = []string{"unknown request", "error:", "warning:"}

// New returns a Socket that talks to the hyprland socket at sockPath.
func New(sockPath string) *Socket {
	return &Socket{
		addr: &net.UnixAddr{Name: sockPath, Net: "unix"},
	}
}

func NewFromEnv() (*Socket, error) {
	his := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	if his == "" {
		return nil, errors.New("HYPRLAND_INSTANCE_SIGNATURE is not set; is hyprland running?")
	}

	xdg := os.Getenv("XDG_RUNTIME_DIR")
	if xdg == "" {
		return nil, errors.New("XDG_RUNTIME_DIR is not set; required to find hyprland socket")
	}

	return New(filepath.Join(xdg, "hypr", his, ".socket.sock")), nil
}

// sends one command and returns hyprland's raw reply
// format: "[flag(s)]/command args" (i.e. "[j]clients"). See https://wiki.hypr.land/IPC/
func (s *Socket) request(request string) ([]byte, error) {
	conn, err := net.DialUnix("unix", nil, s.addr)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	_, err = conn.Write([]byte(request))
	if err != nil {
		return nil, err
	}

	responseBytes, err := io.ReadAll(conn)
	if err != nil {
		return nil, err
	}

	if err := replyError(responseBytes); err != nil {
		return nil, err
	}

	return responseBytes, nil
}

func replyError(reply []byte) error {
	for _, prefix := range replyErrorPrefixes {
		if bytes.HasPrefix(reply, []byte(prefix)) {
			return fmt.Errorf("hyprland: %s", strings.TrimSpace(string(reply)))
		}
	}

	return nil
}

func (s *Socket) Clients() ([]Client, error) {
	return s.requestList[Client]("[j]/clients")
}

func (s *Socket) Monitors() ([]Monitor, error) {
	return s.requestList[Monitor]("[j]/monitors")
}

func (s *Socket) requestList[T any](request string) ([]T, error) {
	response, err := s.request(request)
	if err != nil {
		return nil, err
	}

	var list []T
	if err := json.Unmarshal(response, &list); err != nil {
		return nil, err
	}

	return list, nil
}

// Eval runs arbitrary Lua against hyprland's `hl` API.
func (s *Socket) Eval(lua string) error {
	_, err := s.request("/eval " + lua)

	return err
}

// Dispatch evaluates a Lua expression that must produce an HL.Dispatcher, such
// as `hl.dsp.focus({window="address:0x1"})`.
func (s *Socket) Dispatch(expr string) error {
	_, err := s.request("/dispatch " + expr)

	return err
}
