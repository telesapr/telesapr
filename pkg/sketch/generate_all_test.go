package sketch

import (
	"testing"
)

// TestSketchParamsToMap verifies the helper that converts a
// sketch's param list to a handle-indexed map.
func TestSketchParamsToMap(t *testing.T) {
	sk := NewSketch()
	sk.AddParam(coreParam(1, 10))
	sk.AddParam(coreParam(2, 20))
	sk.AddParam(coreParam(3, 30))
	m := sk.ParamsToMap()
	if len(m) != 3 {
		t.Fatalf("expected 3 params, got %d", len(m))
	}
	if m[hparam(1)] != 10 || m[hparam(2)] != 20 || m[hparam(3)] != 30 {
		t.Fatalf("unexpected values: %v", m)
	}
}

// TestSketchEntitiesToMap verifies the helper that converts a
// sketch's entity list to a handle-indexed map.
func TestSketchEntitiesToMap(t *testing.T) {
	sk := NewSketch()
	h1 := sk.AddEntity(coreEntityPoint(hentity(1)))
	h2 := sk.AddEntity(coreEntityPoint(hentity(2)))
	m := sk.EntitiesToMap()
	if len(m) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(m))
	}
	if _, ok := m[h1]; !ok {
		t.Fatalf("missing entity %v", h1)
	}
	if _, ok := m[h2]; !ok {
		t.Fatalf("missing entity %v", h2)
	}
}

// TestGenerateAllRunsWithoutCrash verifies that GenerateAll on an
// empty sketch completes without crashing.
func TestGenerateAllRunsWithoutCrash(t *testing.T) {
	sk := NewSketch()
	sk.AddGroup(Group{Type: GroupDrawing3D})
	sk.GenerateAll()
}
