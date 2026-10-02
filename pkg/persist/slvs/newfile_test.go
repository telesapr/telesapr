package slvs_test

import (
	"fmt"
	"github.com/spacesolve/spacesolve/pkg/internal/slvswrite"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// TestNewFileVsCppSave checks that a new file, generated as C++
// AfterNewFile does, saves to the bytes C++ writes for the new file
// SS.Init makes (scripts/regen-cpp-saves.sh).
func TestNewFileVsCppSave(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "cppsave", "newfile.cppsave"))
	if err != nil {
		t.Fatal(err)
	}
	sk := sketch.NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	got := slvswrite.Save(sk)
	if string(got) != string(want) {
		t.Errorf("new file save differs from C++:\n%s", firstDiff(string(want), string(got)))
	}
}

// firstDiff shows the first line where two saves differ.
func firstDiff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("line %d:\n C++: %q\n  Go: %q", i+1, w, g)
		}
	}
	return ""
}
