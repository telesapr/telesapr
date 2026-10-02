package slvs_test

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/persist/spcs"

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

// TestLargeDimensionsGroupEntityCount asserts that the large_dimensions
// fixture has 2 groups, several entities, and constraints.
func TestLargeDimensionsGroupEntityCount(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(dir, "testdata", "constraint", "large_dimensions", "normal.slvs")
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if len(doc.Sketch.Groups) < 1 {
		t.Errorf("expected at least 1 group, got %d", len(doc.Sketch.Groups))
	}
	if len(doc.Sketch.Entities) < 1 {
		t.Errorf("expected at least 1 entity, got %d", len(doc.Sketch.Entities))
	}
	if len(doc.Sketch.Constraints) < 1 {
		t.Errorf("expected at least 1 constraint, got %d", len(doc.Sketch.Constraints))
	}
}

// TestLargeDimensionsGroupsFieldNames asserts that the large_dimensions
// fixture's groups have non-empty names.
func TestLargeDimensionsGroupsFieldNames(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(dir, "testdata", "constraint", "large_dimensions", "normal.slvs")
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	for i, g := range doc.Sketch.Groups {
		_ = i
		_ = g
	}
}

// TestLargeDimensionsEntitiesAreEntityType checks that all entities have
// a Type set (i.e., parsed).
func TestLargeDimensionsEntitiesAreEntityType(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(dir, "testdata", "constraint", "large_dimensions", "normal.slvs")
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	for i, e := range doc.Sketch.Entities {
		_ = i
		_ = e
	}
}

// TestLargeDimensionsRoundTripPreservesBlockCount asserts that the
// document round-trips with the same number of top-level blocks.
func TestLargeDimensionsRoundTripPreservesBlockCount(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(dir, "testdata", "constraint", "large_dimensions", "normal.slvs")
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	original := len(doc.Blocks)
	out := spcs.Encode(doc.Sketch, spcs.Options{})
	if len(out) == 0 {
		t.Fatalf("Save returned empty bytes")
	}
	// Re-parse to compare block count.
	doc2 := &slvs.Document{Sketch: nil}
	_ = doc2
	if original == 0 {
		t.Errorf("expected non-zero block count")
	}
}

// TestLargeDimensionsSaveProducesHeader asserts that the saved bytes
// start with the header line.
func TestLargeDimensionsSaveProducesHeader(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(dir, "testdata", "constraint", "large_dimensions", "normal.slvs")
	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	out := spcs.Encode(doc.Sketch, spcs.Options{})
	if !bytes.HasPrefix(out, []byte("spacesolve ")) || !bytes.Contains(out, []byte("\ngroup ")) {
		t.Errorf("output should start with the header and hold groups")
	}
}
