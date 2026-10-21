package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// A listing leaves out hidden entries, marks directories, gives files
// their sizes in name order, and stops at MaxEntries, counting the
// rest.
func TestListFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for i := range MaxEntries + 2 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%04d.stl", i)), []byte("abc"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "a-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := ListFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if l.Dir != dir || len(l.Entries) != MaxEntries || l.More != 3 {
		t.Fatalf("%s: %d entries and %d more", l.Dir, len(l.Entries), l.More)
	}
	if e := l.Entries[0]; e.Name != "a-dir" || !e.Dir || e.Size != 0 {
		t.Errorf("the first entry is %+v", e)
	}
	if e := l.Entries[1]; e.Name != "f0000.stl" || e.Dir || e.Size != 3 {
		t.Errorf("the second entry is %+v", e)
	}
	if _, err := ListFiles(filepath.Join(dir, "none")); err == nil {
		t.Errorf("a directory that isn't there is listed")
	}
}
