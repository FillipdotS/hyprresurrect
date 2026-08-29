package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	fmt.Println("Getting Hyprland Instance Signature (HIS)...")
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

	raddr := &net.UnixAddr{Name: address, Net: "unix"}
	conn, err := net.DialUnix("unix", nil, raddr)
	if err != nil {
		panic(err)
	}
	defer func() { _ = conn.Close() }()

	command := fmt.Appendf(nil, "/notify 2 10000 %s %s", "rgb(ff1ea3)", "Hello world!")

	_, err = conn.Write(command)
	if err != nil {
		panic(err)
	}

	r, err := io.ReadAll(conn)
	if err != nil {
		panic(err)
	}

	resp := string(r)
	if resp != "ok" {
		panic(resp)
	}

	fmt.Println(resp)

	err = conn.Close()
	if err != nil {
		panic(err)
	}
}
