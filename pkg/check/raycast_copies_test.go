package check_test

import (
	"errors"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// agrees checks the oracle of the active group against a volume, its
// grids of up to most rays a side, and, ray by ray, against the
// kernel's mesh.
func agrees(t *testing.T, name string, d *api.Doc, want, rel float64, most int) {
	t.Helper()
	o := oracleOf(t, d)
	m := o.Measure(rel/2, most)
	if !near(m.Volume, want, rel) {
		t.Errorf("%s: volume %g, want %g (spread %g at %d rays)", name, m.Volume, want, m.Spread(), m.Rays)
	}
	if diffs := o.Diff(meshOf(d), 16, 0.1); len(diffs) != 0 {
		t.Errorf("%s: %d rays disagree with the mesh, as %+v", name, len(diffs), diffs[0])
	}
}

// A plate with a hole, the hole repeated along x: the copies are cut
// from the plate as the hole is, and the step and repeat's own solid is
// the holes alone.
func TestOracleTranslate(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(60, 20, 0))
	d.Extrude(10, "plate")
	d.SketchIn(d.Workplane("XY"), "")
	d.Circle(api.V(10, 10, 0), 4)
	d.Extrude(10, "hole").SetCombine(api.Difference)
	d.Translate(3, api.V(15, 0, 0), "")
	agrees(t, "translate", d, 60*20*10-3*math.Pi*16*10, 2e-3, 128)
}

// A block off the z axis turned four times a quarter turn about it:
// four blocks, overlapping none, and joined to a hub they reach into.
func TestOracleRotate(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(-6, -6, 0), api.V(6, 6, 0))
	d.Extrude(10, "hub")
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(api.V(4, -2, 0), api.V(20, 2, 0))
	d.Extrude(5, "spoke")
	d.Rotate(4, d.Origin(), d.Workplane("XY").Normal(), 90, "")
	// Each spoke is 4 by 16 by 5, 2 of its length inside the hub; the
	// one turned onto itself four times counts once.
	agrees(t, "rotate", d, 12*12*10+4*(4*14*5), 5e-3, 256)
}

// A block reflected in the YZ plane, keeping the original, then without
// it; and the solid so far, a plate and the block on it, reflected whole.
func TestOracleMirror(t *testing.T) {
	t.Parallel()
	block := func() *api.Doc {
		d := api.New()
		d.Rect(api.V(5, 0, 0), api.V(15, 10, 0))
		d.Extrude(10, "block")
		return d
	}
	d := block()
	m := d.Mirror(d.Workplane("YZ"), "")
	agrees(t, "mirror", d, 2*1000, 5e-3, 256)
	m.SetKeepOriginal(false)
	agrees(t, "the reflection alone", d, 1000, 5e-3, 256)
	o := oracleOf(t, d)
	if o.Inside(api.V(10, 5, 5)) || !o.Inside(api.V(-10, 5, 5)) {
		t.Error("the reflection alone: the original is still there, or the copy isn't")
	}

	d = api.New()
	d.Rect(api.V(0, 0, 0), api.V(20, 20, 0))
	d.Extrude(5, "plate")
	d.SketchIn(d.Workplane("XY"), "")
	d.Rect(api.V(5, 5, 0), api.V(10, 10, 0))
	d.Extrude(15, "boss")
	whole := d.Mirror(d.Workplane("YZ"), "")
	whole.SetMirrorSolid(true)
	agrees(t, "the solid so far", d, 2*(20*20*5+5*5*10), 5e-3, 256)
}

// A post repeated along a line segment, five copies over 40: the last at
// the far end.
func TestOraclePathPattern(t *testing.T) {
	t.Parallel()
	d := api.New()
	path := d.Line(api.V(0, 0, 0), api.V(40, 0, 0))
	path.SetConstruction(true)
	d.SketchIn(d.Workplane("XY"), "post")
	d.Rect(api.V(-2, -2, 0), api.V(2, 2, 0))
	d.Extrude(6, "post")
	d.PatternAlongPath([]api.Entity{path}, 5, "")
	agrees(t, "pattern", d, 5*16*6, 5e-3, 256)
}

// A copy group whose placements can't be had is refused, naming it.
func TestOracleCopiesRefused(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	ex := d.Extrude(10, "")
	d.ChamferSolid(ex.Solid().Edges()[:1], 1, "")
	d.Translate(2, api.V(20, 0, 0), "")
	sk := d.Host().Sketch()
	sk.GenerateAll()
	if _, err := check.NewOracle(sk, sk.ActiveGroup()); !errors.Is(err, check.ErrUnsupported) {
		t.Errorf("copies of a chamfer: %v", err)
	}
	if _, _, err := sk.Group(ex.H).CopyPlacements(sk); !errors.Is(err, sketch.ErrNotCopies) {
		t.Errorf("an extrusion's placements: %v", err)
	}
}

// A box's inertia about its centre is V (b² + c²) / 12 and the rest; a
// lathed cylinder's about its axis V r² / 2, and V (3 r² + h²) / 12
// across it. The principal moments are those, least first.
func TestOracleInertia(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	d.Extrude(10, "")
	m := oracleOf(t, d).Measure(1e-9, 64)
	v := 6000.0
	want := [3]float64{v * (20*20 + 10*10) / 12, v * (30*30 + 10*10) / 12, v * (30*30 + 20*20) / 12}
	for k := range 3 {
		if !near(m.Inertia[k][k], want[k], 1e-6) {
			t.Errorf("box: I%d%d %g, want %g", k, k, m.Inertia[k][k], want[k])
		}
		for j := range 3 {
			if j != k && math.Abs(m.Inertia[k][j]) > 1e-6*want[0] {
				t.Errorf("box: I%d%d %g, want 0", k, j, m.Inertia[k][j])
			}
		}
	}
	if p := m.PrincipalMoments(); !near(p[0], want[0], 1e-6) || !near(p[2], want[2], 1e-6) {
		t.Errorf("box: principal %v, want %v", p, want)
	}

	d = api.New()
	d.SketchIn(d.Workplane("ZX"), "")
	axis := d.Line(api.V(0, 0, -1), api.V(0, 0, 21))
	axis.SetConstruction(true)
	d.Polyline(true, api.V(0, 0, 0), api.V(5, 0, 0), api.V(5, 0, 20), api.V(0, 0, 20))
	d.Lathe(axis, "")
	m = oracleOf(t, d).Measure(1e-4, 256)
	v = math.Pi * 25 * 20
	axial, across := v*25/2, v*(3*25+400)/12
	if !near(m.Inertia[2][2], axial, 3e-3) || !near(m.Inertia[0][0], across, 3e-3) || !near(m.Inertia[1][1], across, 3e-3) {
		t.Errorf("cylinder: %v, want %g across and %g about the axis", m.Inertia, across, axial)
	}
	if p := m.PrincipalMoments(); !near(p[0], axial, 3e-3) || !near(p[2], across, 3e-3) {
		t.Errorf("cylinder: principal %v", p)
	}
	if c := m.CenterOfMass; c.Minus(geom.Vector{Z: 10}).Magnitude() > 0.01 {
		t.Errorf("cylinder: centre %v", c)
	}
}
