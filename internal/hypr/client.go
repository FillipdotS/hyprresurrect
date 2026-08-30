// Package hypr deals with connecting to the hyprland UNIX socket
package hypr

import (
	"fmt"
	"io"
	"net"
	"os"
)

type Client struct {
	addr *net.UnixAddr
}

func New() *Client {
	his := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	xdg := os.Getenv("XDG_RUNTIME_DIR")

	if his == "" {
		panic("HYPRLAND_INSTANCE_SIGNATURE not found as env var!")
	}
	if xdg == "" {
		panic("XDG_RUNTIME_DIR not found as env var!")
	}

	a := net.UnixAddr{Name: fmt.Sprintf("%s/hypr/%s/.socket.sock", xdg, his), Net: "unix"}

	return &Client{
		addr: &a,
	}
}

// sends one command and returns hyprland's raw reply
func (c *Client) request(request string) (string, error) {
	conn, err := net.DialUnix("unix", nil, c.addr)
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

func (c *Client) Notify(message string) error {
	response, err := c.request(fmt.Sprintf("/notify 2 10000 %s %s", "rgb(ff1ea3)", message))
	if err != nil {
		return err
	}
	if response != "ok" {
		return fmt.Errorf("notify rejected: %s", response)
	}

	return nil
}
