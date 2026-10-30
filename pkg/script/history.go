package script

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// This file is the history of what was typed in a console, kept
// between runs, which the window's console and telesapr-cli repl share.

// HistoryLimit is how many entries a history keeps.
const HistoryLimit = 1000

// DefaultHistory is the file a console's history is kept in:
// telesapr/script_history in the user's configuration folder, "" if
// there is none.
func DefaultHistory() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "telesapr", "script_history")
}

// History is what was entered in a console, oldest first, and the file
// it is kept in, which may be "" to keep it only while the program
// runs. An entry may be of several lines; in the file each is a line,
// quoted as a Go string.
type History struct {
	path    string
	entries []string
}

// LoadHistory reads the history kept in path. A file that isn't there
// is an empty history; one with more than HistoryLimit entries keeps
// the last ones, and is rewritten so when it has twice as many.
func LoadHistory(path string) (*History, error) {
	h := &History{path: path}
	if path == "" {
		return h, nil
	}
	// The file is read whole and closed before it may be rewritten:
	// Windows refuses to rename a file over one that is open.
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return h, nil
	}
	if err != nil {
		return h, fmt.Errorf("reading history: %w", err)
	}
	read := 0
	for line := range strings.Lines(string(data)) {
		entry, err := strconv.Unquote(strings.TrimRight(line, "\r\n"))
		if err != nil {
			continue
		}
		read++
		h.entries = append(h.entries, entry)
	}
	if len(h.entries) > HistoryLimit {
		h.entries = h.entries[len(h.entries)-HistoryLimit:]
	}
	if read > 2*HistoryLimit {
		if err := h.rewrite(); err != nil {
			return h, err
		}
	}
	return h, nil
}

// Entries are the history's entries, oldest first; the slice is the
// history's own.
func (h *History) Entries() []string { return h.entries }

// Add adds an entry at the end of the history and of its file. An
// entry of nothing but space, or the same as the last, isn't added.
func (h *History) Add(entry string) error {
	if strings.TrimSpace(entry) == "" || len(h.entries) > 0 && h.entries[len(h.entries)-1] == entry {
		return nil
	}
	h.entries = append(h.entries, entry)
	if len(h.entries) > HistoryLimit {
		h.entries = h.entries[len(h.entries)-HistoryLimit:]
	}
	if h.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return fmt.Errorf("writing history: %w", err)
	}
	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("writing history: %w", err)
	}
	_, err = fmt.Fprintln(f, strconv.Quote(entry))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("writing history: %w", err)
	}
	return nil
}

// rewrite writes the file anew with the entries the history keeps.
func (h *History) rewrite() error {
	var b strings.Builder
	for _, entry := range h.entries {
		b.WriteString(strconv.Quote(entry))
		b.WriteByte('\n')
	}
	tmp := h.path + ".new"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("writing history: %w", err)
	}
	if err := os.Rename(tmp, h.path); err != nil {
		return fmt.Errorf("writing history: %w", err)
	}
	return nil
}
