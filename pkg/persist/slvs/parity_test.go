package slvs

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// TestParityConstraintFixtures walks every non-boolean constraint
// fixture through LoadFile + Sketch::GenerateAll and asserts that the
// load succeeds, every group reports Clean=true, and every group's
// Solved.How is either SolveOkay or SolveRedundantOkay. This is the
// parity gate for Phase E.
func TestParityConstraintFixtures(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	constraintsRoot := filepath.Join(root, "testdata/constraint")
	entries, err := os.ReadDir(constraintsRoot)
	if err != nil {
		t.Fatalf("readDir: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var solved, notSolved, skipped int
	var failing []string
	for _, dir := range entries {
		if !dir.IsDir() {
			continue
		}
		dirName := dir.Name()
		// large_dimensions uses unusual field ordering that the Go
		// round-trip tests already cover.
		if dirName == "large_dimensions" {
			skipped++
			continue
		}
		fixturePath := filepath.Join(constraintsRoot, dirName, "normal.slvs")
		if _, err := os.Stat(fixturePath); err != nil {
			// Fixture has no normal.slvs; skip.
			skipped++
			continue
		}
		doc, err := LoadFile(fixturePath)
		if err != nil {
			notSolved++
			failing = append(failing, dirName)
			continue
		}
		doc.Sketch.GenerateAll()
		allClean := true
		anySolveFailed := false
		for i := range doc.Sketch.Groups {
			g := &doc.Sketch.Groups[i]
			if !g.Clean {
				allClean = false
			}
			if g.Solved.How != core.SolveOkay && g.Solved.How != core.SolveRedundantOkay {
				anySolveFailed = true
			}
		}
		if allClean && !anySolveFailed {
			solved++
		} else {
			notSolved++
			failing = append(failing, dirName)
		}
	}
	t.Logf("constraint parity: solved=%d failed=%d skipped=%d", solved, notSolved, skipped)
	if notSolved > 0 {
		t.Errorf("constraint fixtures failed to solve: %v", failing)
	}
	if solved < 25 {
		t.Errorf("expected at least 25 constraint fixtures to solve, got %d", solved)
	}
}
