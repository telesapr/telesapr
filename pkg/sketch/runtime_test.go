package sketch_test

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func fixturesPath(t *testing.T, sub ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	dir := filepath.Dir(file)      // .../pkg/sketch
	pkgDir := filepath.Dir(dir)    // .../pkg
	goRoot := filepath.Dir(pkgDir) // .../go
	parts := append([]string{goRoot, "testdata"}, sub...)
	return filepath.Join(parts...)
}

func TestRuntimeLookup(t *testing.T) {
	path := fixturesPath(t, "request", "line_segment", "normal.slvs")

	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	// The file's saved actPoint/actNormal are what C++ computed; check
	// the regenerated entities against them.
	saved := make(map[core.HEntity]core.Entity, len(doc.Sketch.Entities))
	for _, e := range doc.Sketch.Entities {
		saved[e.H] = e
	}
	doc.Sketch.GenerateAll()

	rt := sketch.NewRuntime(doc.Sketch)

	if len(doc.Sketch.Entities) == 0 {
		t.Fatalf("fixture has no entities")
	}

	firstEntity := doc.Sketch.Entities[0]
	e, err := rt.Entity(firstEntity.H)
	if err != nil {
		t.Fatalf("runtime entity lookup: %v", err)
	}
	if e != &doc.Sketch.Entities[0] {
		t.Fatalf("runtime returned unexpected entity pointer")
	}

	// Verify point and normal calculations align with stored values.
	for _, e := range doc.Sketch.Entities {
		ent, ok := saved[e.H]
		if !ok {
			t.Fatalf("entity %v not in the file", e.H)
		}
		switch ent.Type {
		case core.EntityPointIn3D, core.EntityPointIn2D:
			got, err := rt.PointValue(ent.H)
			if err != nil {
				t.Fatalf("point value: %v", err)
			}
			if got.DistanceTo(ent.ActPoint) > 1e-8 {
				t.Fatalf("point mismatch: got %v want %v", got, ent.ActPoint)
			}
		case core.EntityNormalIn3D:
			got, err := rt.NormalQuaternion(ent.H)
			if err != nil {
				t.Fatalf("normal quaternion: %v", err)
			}
			want := ent.ActNormal.Normal()
			if !sketch.AlmostEqualQuaternion(got, want, 1e-8) {
				t.Fatalf("normal mismatch: got %v want %v", got, want)
			}
		}
	}

	// Mutate sketch and rebuild indexes.
	newGroup := sketch.Group{}
	h := doc.Sketch.AddGroup(newGroup)
	rt.Rebuild()
	g, err := rt.Group(h)
	if err != nil {
		t.Fatalf("runtime group lookup: %v", err)
	}
	if g == nil {
		t.Fatalf("expected group lookup to succeed")
	}
}

func TestRuntimeConstraintDistance(t *testing.T) {
	path := fixturesPath(t, "constraint", "pt_pt_distance", "normal.slvs")

	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	doc.Sketch.GenerateAll()

	rt := sketch.NewRuntime(doc.Sketch)
	found := false
	for _, c := range doc.Sketch.Constraints {
		if c.Type != core.ConstraintPtPtDistance {
			continue
		}
		found = true
		pa, err := rt.PointValue(c.PtA)
		if err != nil {
			t.Fatalf("point A value: %v", err)
		}
		pb, err := rt.PointValue(c.PtB)
		if err != nil {
			t.Fatalf("point B value: %v", err)
		}
		dist := pa.DistanceTo(pb)
		if math.Abs(dist-c.ValA) > 1e-8 {
			t.Fatalf("constraint distance mismatch: got %f want %f", dist, c.ValA)
		}
	}
	if !found {
		t.Fatalf("expected pt-pt distance constraint in fixture")
	}
}

func TestRuntimeConstraintPointOnCircle(t *testing.T) {
	path := fixturesPath(t, "constraint", "pt_on_circle", "normal.slvs")

	doc, err := slvs.LoadFile(path)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	doc.Sketch.GenerateAll()

	rt := sketch.NewRuntime(doc.Sketch)
	found := false
	for _, c := range doc.Sketch.Constraints {
		if c.Type != core.ConstraintPtOnCircle {
			continue
		}
		found = true
		pt, err := rt.PointValue(c.PtA)
		if err != nil {
			t.Fatalf("point value: %v", err)
		}
		circle, err := rt.Entity(c.EntityA)
		if err != nil {
			t.Fatalf("circle entity: %v", err)
		}
		if circle == nil {
			t.Fatalf("circle entity missing")
		}
		center, err := rt.PointValue(circle.Point[0])
		if err != nil {
			t.Fatalf("circle center: %v", err)
		}
		radius, err := rt.DistanceValue(circle.Distance)
		if err != nil {
			t.Fatalf("circle radius: %v", err)
		}
		dist := pt.DistanceTo(center)
		if math.Abs(dist-radius) > 1e-8 {
			t.Fatalf("point not on circle: got %f want %f", dist, radius)
		}
	}
	if !found {
		t.Fatalf("expected pt-on-circle constraint in fixture")
	}
}

func TestRuntimeRecalculateEntities(t *testing.T) {
	sk := sketch.NewSketch()

	px := core.Param{H: core.HParam{V: 0x00010001}, Val: 3}
	py := core.Param{H: core.HParam{V: 0x00010002}, Val: 4}
	pz := core.Param{H: core.HParam{V: 0x00010003}, Val: 5}
	sk.Params = append(sk.Params, px, py, pz)

	ent := core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: 0x00010000},
			Type:  core.EntityPointIn3D,
			Param: [8]core.HParam{px.H, py.H, pz.H},
		},
	}
	sk.Entities = append(sk.Entities, ent)

	rt := sketch.NewRuntime(sk)
	rt.RecalculateEntities(false)

	got := sk.Entities[0].ActPoint
	want := geom.NewVector(3, 4, 5)
	if got.DistanceTo(want) > 1e-9 {
		t.Fatalf("recalculate entities: got %v want %v", got, want)
	}
}

// TestRuntimeParamValueMissing verifies ParamValue returns ok=false for missing.
func TestRuntimeParamValueMissing(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	_, ok := rt.ParamValue(core.HParam{V: 0x999})
	if ok {
		t.Errorf("unknown param should return ok=false")
	}
}

// TestRuntimeEntityReturnsUnknown verifies Entity returns error for missing.
func TestRuntimeEntityReturnsUnknown(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	_, err := rt.Entity(core.HEntity{V: 0x999})
	if err == nil {
		t.Errorf("unknown entity should return error")
	}
}

// TestRuntimeDistanceValueMissing verifies DistanceValue handles missing entity.
func TestRuntimeDistanceValueMissing(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	_, err := rt.DistanceValue(core.HEntity{V: 0x999})
	if err == nil {
		t.Errorf("missing entity distance should error")
	}
}

// TestRuntimePointValueMissing verifies PointValue handles missing entity.
func TestRuntimePointValueMissing(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	_, err := rt.PointValue(core.HEntity{V: 0x999})
	if err == nil {
		t.Errorf("missing entity point should error")
	}
}

// TestRuntimeRecalculateEmpty verifies Recalculate on empty sketch.
func TestRuntimeRecalculateEmpty(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	rt.RecalculateEntities(false)
	if len(sk.Entities) != 0 {
		t.Errorf("empty sketch should remain empty")
	}
}

// TestRuntimeGroupAccessors verifies Runtime's accessors on a group.
func TestRuntimeGroupAccessors(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	if _, err := rt.Group(core.HGroup{V: 0x999}); err == nil {
		t.Errorf("missing group should return nil")
	}
}

// TestRuntimeConstraintAccessors verifies Constraint accessors.
func TestRuntimeConstraintAccessors(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	if _, err := rt.Constraint(core.HConstraint{V: 0x999}); err == nil {
		t.Errorf("missing constraint should return nil")
	}
}

// TestRuntimeRequestAccessors verifies Request accessors.
func TestRuntimeRequestAccessors(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	if _, err := rt.Request(core.HRequest{V: 0x999}); err == nil {
		t.Errorf("missing request should return nil")
	}
}

// TestRuntimeParamStringValue verifies StringValue for params.
func TestRuntimeParamStringValue(t *testing.T) {
	sk := sketch.NewSketch()
	sk.Params = append(sk.Params, core.Param{H: core.HParam{V: 1}, Val: 5.0})
	rt := sketch.NewRuntime(sk)
	v, ok := rt.ParamValue(core.HParam{V: 1})
	if !ok || v != 5.0 {
		t.Errorf("got (%v, %v), want (5.0, true)", v, ok)
	}
}

// TestRuntimeParamStringValueMissing verifies StringValue for missing.
func TestRuntimeParamStringValueMissing(t *testing.T) {
	sk := sketch.NewSketch()
	rt := sketch.NewRuntime(sk)
	_, ok := rt.ParamValue(core.HParam{V: 0x999})
	if ok {
		t.Errorf("missing param should return ok=false")
	}
}
