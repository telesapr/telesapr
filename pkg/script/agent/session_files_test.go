package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
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

// A session whose scripts' files are guarded lists only the folders
// they may read: list_files is no way around the guard.
func TestFilesGuarded(t *testing.T) {
	t.Parallel()
	allowed, other := t.TempDir(), t.TempDir()
	sk := sketch.NewSketch()
	sk.NewFile()
	s, _, err := Open(script.Options{Host: api.NewSketchHost(sk, ""), Files: &script.Files{Write: []string{allowed}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Files(context.Background(), allowed); err != nil {
		t.Errorf("listing an allowed folder: %v", err)
	}
	if _, err := s.Files(context.Background(), other); !errors.Is(err, script.ErrNotAllowed) {
		t.Errorf("listing a folder outside: %v", err)
	}
}
