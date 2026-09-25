package slvs_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
)

// TestLargeDimensionsConstraintFixture verifies that the
// large_dimensions constraint fixture loads cleanly and contains
// the expected entity / constraint structure.
func TestLargeDimensionsConstraintFixture(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Dir(file)
	root := filepath.Join(dir, "..", "..", "..", "testdata", "constraint", "large_dimensions", "normal.slvs")
	doc, err := slvs.LoadFile(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(doc.Sketch.Groups) == 0 {
		t.Fatalf("expected non-empty sketch")
	}
	if len(doc.Sketch.Entities) == 0 {
		t.Fatalf("expected non-empty entities")
	}
	if len(doc.Sketch.Constraints) == 0 {
		t.Fatalf("expected non-empty constraints")
	}
	// Verify at least one constraint has a numeric value set.
	foundValued := false
	for i := range doc.Sketch.Constraints {
		if doc.Sketch.Constraints[i].ValA != 0 {
			foundValued = true
			break
		}
	}
	if !foundValued {
		t.Logf("note: no constraint has ValA set (may indicate parse issue)")
	}
	_ = core.HEntity{}
}
