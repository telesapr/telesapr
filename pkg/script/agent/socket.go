package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// This file is where the program with the drawing and the one that
// reaches it meet: a Unix socket in a directory only the user can
// open, named after the process that listens on it.

// maxSocketPath is the longest name a socket can have on the system
// with the shortest limit, macOS, less the byte that ends it.
const maxSocketPath = 103

// SocketDir is the directory the sockets are in: of the user alone,
// and gone with the session where the system has such a place.
func SocketDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// The profile is the user's own; mode bits mean nothing here.
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, "telesapr"), nil
		}
		dir, err := os.UserCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "telesapr"), nil
	case "darwin":
		// TMPDIR is each user's own there.
		return filepath.Join(os.TempDir(), "telesapr"), nil
	}
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "telesapr"), nil
	}
	return filepath.Join(os.TempDir(), "telesapr-"+strconv.Itoa(os.Getuid())), nil
}

const (
	socketPrefix = "agent-"
	socketSuffix = ".sock"
)

// socketName is the socket of a process in a directory.
func socketName(dir string, pid int) string {
	return filepath.Join(dir, socketPrefix+strconv.Itoa(pid)+socketSuffix)
}

// sockets are the sockets in a directory, the newest first.
func sockets(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	type socket struct {
		path string
		made time.Time
	}
	var found []socket
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, socketPrefix) || !strings.HasSuffix(name, socketSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		found = append(found, socket{filepath.Join(dir, name), info.ModTime()})
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].made.After(found[j].made) })
	paths := make([]string, len(found))
	for i, s := range found {
		paths[i] = s.path
	}
	return paths
}

// Listen listens on this process's socket in dir, which it makes if
// need be, for the user alone. It returns the socket's path; closing
// the listener removes the socket.
func Listen(dir string) (net.Listener, string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	// A directory that was there may be anybody's.
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, "", err
	}
	path := socketName(dir, os.Getpid())
	if len(path) > maxSocketPath {
		return nil, "", fmt.Errorf("the socket's name, %s, is %d bytes long; a socket's can be %d at most", path, len(path), maxSocketPath)
	}
	// One left by a process with this id that didn't end well.
	_ = os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, "", err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		return nil, "", err
	}
	return l, path, nil
}

// Connect connects to the program that listens in dir: to the newest
// socket that answers. A socket nothing listens on was left by a
// program that didn't end well, and is removed. The error is
// ErrNotRunning when no socket answers.
func Connect(dir string) (net.Conn, error) {
	for _, path := range sockets(dir) {
		conn, err := net.DialTimeout("unix", path, 2*time.Second)
		if err == nil {
			return conn, nil
		}
		// One that only didn't answer in time may still be listened on.
		var ne net.Error
		if !errors.As(err, &ne) || !ne.Timeout() {
			_ = os.Remove(path)
		}
	}
	return nil, ErrNotRunning
}

// Accept serves b on every connection l takes, until l is closed; it
// then ends the connections it has, and returns when they have ended.
func Accept(ctx context.Context, l net.Listener, b Backend) {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		wg.Go(func() {
			stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
			defer stop()
			_ = Serve(ctx, conn, b)
			_ = conn.Close()
		})
	}
}

// ConnectTo connects to the socket at path. The error wraps
// ErrNotRunning when nothing listens there.
func ConnectTo(path string) (net.Conn, error) {
	conn, err := net.DialTimeout("unix", path, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("%w (no answer at %s)", ErrNotRunning, path)
	}
	return conn, nil
}
