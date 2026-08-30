// Package hypr deals with connecting to the hyprland UNIX socket
package hypr

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

type Socket struct {
	addr *net.UnixAddr
}

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
func (s *Socket) request(request string) (string, error) {
	conn, err := net.DialUnix("unix", nil, s.addr)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	_, err = conn.Write([]byte(request))
	if err != nil {
		return "", err
	}

	responseBytes, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}

	return string(responseBytes), nil
}

func (s *Socket) Clients() (string, error) {
	response, err := s.request("[j]/clients")
	if err != nil {
		return "", err
	}
	if response == "unknown request" {
		return "", errors.New(response)
	}

	return response, nil
}

func (s *Socket) Notify(message string) error {
	response, err := s.request(fmt.Sprintf("/notify 2 10000 %s %s", "rgb(ff1ea3)", message))
	if err != nil {
		return err
	}
	if response != "ok" {
		return fmt.Errorf("notify rejected: %s", response)
	}

	return nil
}
