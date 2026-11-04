package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
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
	slices.SortStableFunc(found, func(a, b socket) int {
		less := func(a, b socket) bool { return a.made.After(b.made) }
		switch {
		case less(a, b):
			return -1
		case less(b, a):
			return 1
		}
		return 0
	})
	paths := make([]string, len(found))
	for i, s := range found {
		paths[i] = s.path
	}
	return paths
}
