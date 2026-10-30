package script

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestHistory keeps entries in a file, several lines of one included,
// leaving out blank and repeated ones, and keeps the last HistoryLimit.
func TestHistory(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "sub", "history")
	h, err := LoadHistory(path)
	if err != nil || len(h.Entries()) != 0 {
		t.Fatalf("a missing file: %v %v", h.Entries(), err)
	}
	for _, e := range []string{"a := 1", "a := 1", "  ", "for {\n\t\"x\"\n}"} {
		if err := h.Add(e); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"a := 1", "for {\n\t\"x\"\n}"}
	if !slices.Equal(h.Entries(), want) {
		t.Errorf("entries %q", h.Entries())
	}
	again, err := LoadHistory(path)
	if err != nil || !slices.Equal(again.Entries(), want) {
		t.Errorf("read back %q, %v", again.Entries(), err)
	}

	// A file grown past twice the limit is cut back to it.
	var b strings.Builder
	b.WriteString("not quoted\n")
	for i := range 2*HistoryLimit + 1 {
		b.WriteString(strconv.Quote(strconv.Itoa(i)) + "\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	long, err := LoadHistory(path)
	if err != nil || len(long.Entries()) != HistoryLimit || long.Entries()[HistoryLimit-1] != strconv.Itoa(2*HistoryLimit) {
		t.Fatalf("a long file: %d entries, %v", len(long.Entries()), err)
	}
	if err := long.Add("last"); err != nil || len(long.Entries()) != HistoryLimit {
		t.Fatalf("adding to a full history: %d, %v", len(long.Entries()), err)
	}
	data, _ := os.ReadFile(path)
	if n := strings.Count(string(data), "\n"); n != HistoryLimit+1 {
		t.Errorf("the file has %d lines after it was cut", n)
	}

	// Without a file the history lasts as long as the program.
	mem, err := LoadHistory("")
	if err != nil || mem.Add("x") != nil || !slices.Equal(mem.Entries(), []string{"x"}) {
		t.Errorf("a history without a file: %v", err)
	}
	if DefaultHistory() == "" {
		t.Log("no configuration folder")
	}

	// A file that can't be read, or written, is an error.
	if _, err := LoadHistory(t.TempDir()); err == nil {
		t.Error("a folder read as a history")
	}
	blocked := &History{path: filepath.Join(path, "x")}
	if err := blocked.Add("y"); err == nil {
		t.Error("a history written under a file")
	}
}
