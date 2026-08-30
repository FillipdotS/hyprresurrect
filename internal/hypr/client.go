// Package hypr deals with connecting to the hyprland UNIX socket
package hypr

import (
	"fmt"
	"io"
	"net"
	"os"
)

type Hypr struct {
	addr *net.UnixAddr
}

var addr *net.UnixAddr

func init() {
	his := os.Getenv("HYPRLAND_INSTANCE_SIGNATURE")
	xdg := os.Getenv("XDG_RUNTIME_DIR")

	if his == "" {
		panic("HIS (Hyprland Instance Signature) not found as env var!")
	}
	if xdg == "" {
		panic("XDG_RUNTIME_DIR not found as env var!")
	}

	fmt.Println("HIS found:", his)
	fmt.Println("XDG_RUNTIME_DIR found:", his)

	address := fmt.Sprintf("%s/hypr/%s/.socket.sock", xdg, his)
	addr = &net.UnixAddr{Name: address, Net: "unix"}
}

func new() *net.UnixConn {
	c, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		panic(err)
	}

	return c
}

func Notify(message string) {
	c := new()

	command := fmt.Appendf(nil, "/notify 2 10000 %s %s", "rgb(ff1ea3)", message)

	_, err := c.Write(command)
	if err != nil {
		panic(err)
	}

	r, err := io.ReadAll(c)
	if err != nil {
		panic(err)
	}

	resp := string(r)
	if resp != "ok" {
		panic(resp)
	}

	fmt.Println(resp)

	err = c.Close()
	if err != nil {
		panic(err)
	}
}
