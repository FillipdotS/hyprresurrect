package hypr

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

type fakeHypr struct {
	SockPath string

	mu       sync.Mutex
	requests []string
}

func newFakeHypr(t *testing.T, routes map[string]string) *fakeHypr {
	t.Helper()

	// Deliberately not t.TempDir(): that embeds the test name in the path, and
	// unix socket paths are capped at 107 bytes.
	dir, err := os.MkdirTemp("", "hyprtest")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	f := &fakeHypr{SockPath: filepath.Join(dir, ".socket.sock")}

	l, err := net.Listen("unix", f.SockPath)
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return // listener closed by cleanup; not a failure
			}
			f.serve(conn, routes)
		}
	}()

	return f
}

func (f *fakeHypr) serve(conn net.Conn, routes map[string]string) {
	// The client reads until EOF, so the reply is only complete once we close.
	defer func() { _ = conn.Close() }()

	// A single Read, not io.ReadAll: the client never closes its write side, so
	// reading to EOF here would deadlock against it.
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	req := string(buf[:n])

	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()

	reply, ok := routes[req]
	if !ok {
		reply = "unknown request"
	}
	_, _ = io.WriteString(conn, reply)
}

func (f *fakeHypr) Requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.requests)
}
