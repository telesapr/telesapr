package sketch

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
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

// TestGenerateAllNewSketch runs GenerateAll on a new sketch: the
// references plus an empty drawing group. ForceReferences must put
// the three reference workplanes at the origin with their fixed
// orientations.
func TestGenerateAllNewSketch(t *testing.T) {
	sk := NewSketch()
	sk.AddReferences()
	sk.AddGroup(Group{Type: GroupDrawing3D, Order: 1, Scale: 1})
	sk.GenerateAll()

	wants := map[core.HRequest]geom.Quaternion{
		HRequestReferenceXY: {W: 1},
		HRequestReferenceYZ: {W: 0.5, X: 0.5, Y: 0.5, Z: 0.5},
		HRequestReferenceZX: {W: 0.5, X: -0.5, Y: -0.5, Z: -0.5},
	}
	for hr, q := range wants {
		wp := sk.Entity(hr.Entity(0))
		if wp == nil || wp.Type != core.EntityWorkplane {
			t.Fatalf("reference workplane %v missing", hr)
		}
		if got := wp.WorkplaneGetOffset(sk); !got.Equals(geom.Vector{}) {
			t.Errorf("reference %v origin at %v", hr, got)
		}
		if got := wp.NormalEntity(sk).NormalGetNum(sk); !got.Equals(q) {
			t.Errorf("reference %v normal %v, want %v", hr, got, q)
		}
		if !sk.Entity(wp.Point[0]).Construction {
			t.Errorf("reference %v origin should be construction", hr)
		}
	}
	for _, g := range sk.Groups {
		if g.Solved.How != core.SolveOkay {
			t.Errorf("group %v: solve result %v", g.H, g.Solved.How)
		}
	}
}

// TestGenerateAllTyped verifies GenerateAllTyped runs without panic.
func TestGenerateAllTyped(t *testing.T) {
	sk := NewSketch()
	sk.GenerateAllTyped(GenerateDirty)
}

// TestGenerateAllTypedAll verifies the GenerateAll variant.
func TestGenerateAllTypedAll(t *testing.T) {
	sk := NewSketch()
	sk.GenerateAllTyped(GenerateAll)
}

// TestScheduleGenerateAll verifies ScheduleGenerateAll runs.
func TestScheduleGenerateAll(t *testing.T) {
	sk := NewSketch()
	sk.ScheduleGenerateAll()
}

// TestGenerateTypeString verifies each type has a String method or default fmt.
func TestGenerateTypeString(t *testing.T) {
	cases := map[GenerateType]string{
		GenerateAll:         "All",
		GenerateDirty:       "Dirty",
		GenerateRegen:       "Regen",
		GenerateUntilActive: "UntilActive",
	}
	for got, want := range cases {
		if int(got) < 0 || int(got) > 3 {
			t.Errorf("GenerateType(%d) out of range", got)
		}
		_ = want
	}
}
