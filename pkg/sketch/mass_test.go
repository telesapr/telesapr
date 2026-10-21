package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func nearRel(got, want, rel float64) bool { return math.Abs(got-want) <= rel*math.Abs(want) }

// refused reports whether fn fails, as an api call does by panicking
// with an error.
func refused(fn func()) (failed bool) {
	defer func() { _, failed = recover().(error) }()
	fn()
	return false
}

// diagonal checks that inertia has the moments want on its diagonal,
// within rel, and products small beside them.
func diagonal(t *testing.T, what string, in [3][3]float64, want [3]float64, rel float64) {
	t.Helper()
	for a := range 3 {
		for b := range 3 {
			if a == b && !nearRel(in[a][a], want[a], rel) {
				t.Errorf("%s: moment %d %g, want %g", what, a, in[a][a], want[a])
			}
			if a != b && math.Abs(in[a][b]) > rel*want[0] {
				t.Errorf("%s: product %d%d %g, want 0", what, a, b, in[a][b])
			}
		}
	}
}

// A steel box has its closed-form mass, centre and moments, exactly:
// its mesh is the box.
func TestMassBox(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 20, 0))
	box := d.Features.Extrude(30, "")
	box.SetDensity(7.85)
	r := box.Analyze().MassProperties()
	m := 6000 * 7.85 / 1000
	if len(r.Parts) != 1 || !nearRel(r.Mass, m, 1e-9) || !nearRel(r.Volume, 6000, 1e-9) || r.DensityUnset() {
		t.Fatalf("mass %g, volume %g, %d parts, want %g, 6000, 1", r.Mass, r.Volume, len(r.Parts), m)
	}
	// The extrusion runs down the workplane's normal or up it; either
	// way the centre is mid-height.
	if c := r.CenterOfMass; math.Abs(c.X-5) > 1e-9 || math.Abs(c.Y-10) > 1e-9 || math.Abs(math.Abs(c.Z)-15) > 1e-9 {
		t.Errorf("centre %v", c)
	}
	diagonal(t, "box", r.Inertia, [3]float64{m * (400 + 900) / 12, m * (100 + 900) / 12, m * (100 + 400) / 12}, 1e-9)
	if r.Inertia != r.Parts[0].Inertia {
		t.Errorf("one part's inertia %v is not the whole's %v", r.Parts[0].Inertia, r.Inertia)
	}
}

// A lathed cylinder and a lathed sphere come within their meshes'
// chord tolerance of the closed forms: m r²/2 about a cylinder's axis
// and m (3 r² + h²)/12 across it, 2 m r²/5 for a sphere.
func TestMassTurned(t *testing.T) {
	t.Parallel()
	const r, h = 5.0, 20.0
	d := api.New()
	axis := d.Sketch.Line(api.V(0, -1, 0), api.V(0, 21, 0))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(r, 0, 0), api.V(r, h, 0), api.V(0, h, 0))
	cyl := d.Features.Lathe(axis, "")
	cyl.SetDensity(2.7)
	got := cyl.Analyze().MassProperties()
	v := math.Pi * r * r * h
	m := v * 2.7 / 1000
	if !nearRel(got.Volume, v, 0.01) || !nearRel(got.Mass, m, 0.01) {
		t.Errorf("cylinder: volume %g, mass %g, want %g, %g", got.Volume, got.Mass, v, m)
	}
	if c := got.CenterOfMass; c.Minus(geom.Vector{Y: h / 2}).Magnitude() > 1e-3 {
		t.Errorf("cylinder: centre %v", c)
	}
	across := m * (3*r*r + h*h) / 12
	diagonal(t, "cylinder", got.Inertia, [3]float64{across, m * r * r / 2, across}, 0.02)

	d = api.New()
	d.Sketch.Arc(api.V(0, 0, 0), api.V(0, -r, 0), api.V(0, r, 0))
	axis = d.Sketch.Line(api.V(0, r, 0), api.V(0, -r, 0))
	ball := d.Features.Lathe(axis, "")
	got = ball.Analyze().MassProperties()
	v = 4 * math.Pi * r * r * r / 3
	if !got.DensityUnset() || !nearRel(got.Mass, v/1000, 0.01) {
		t.Errorf("sphere: mass %g at the default density, want %g", got.Mass, v/1000)
	}
	in := 2 * got.Mass * r * r / 5
	diagonal(t, "sphere", got.Inertia, [3]float64{in, in, in}, 0.02)
}

// An assembly's parts each take their own density, and the solid
// before the first assembled group the density last set among its
// groups; the whole's centre and inertia combine theirs.
func TestMassAssembly(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	base := d.Features.Extrude(10, "base")
	base.SetDensity(1.0)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	top := d.Features.Extrude(-10, "top") // joins the base, taller
	top.SetDensity(2.0)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(20, 0, 0), api.V(30, 10, 0))
	block := d.Features.Extrude(10, "block")
	block.SetCombine(api.Assemble)
	block.SetDensity(7.85)

	r := block.Analyze().MassProperties()
	if len(r.Parts) != 2 {
		t.Fatalf("%d parts, want 2", len(r.Parts))
	}
	body, other := r.Parts[0], r.Parts[1]
	if body.Group != top.H || body.Density != 2.0 || !nearRel(body.Volume, 2000, 1e-9) {
		t.Errorf("the body: %+v", body)
	}
	if other.Group != block.H || other.Density != 7.85 || !nearRel(other.Mass, 7.85, 1e-9) {
		t.Errorf("the block: %+v", other)
	}
	if !nearRel(r.Mass, body.Mass+other.Mass, 1e-12) {
		t.Errorf("mass %g, parts %g and %g", r.Mass, body.Mass, other.Mass)
	}
	wantX := (body.Mass*5 + other.Mass*25) / r.Mass
	if math.Abs(r.CenterOfMass.X-wantX) > 1e-9 {
		t.Errorf("centre %v, want x %g", r.CenterOfMass, wantX)
	}
	// About z, each block's own moment plus its mass times its distance
	// from the whole's centre squared.
	wantZ := body.Inertia[2][2] + other.Inertia[2][2] +
		body.Mass*sq(5-wantX) + other.Mass*sq(25-wantX)
	if !nearRel(r.Inertia[2][2], wantZ, 1e-9) {
		t.Errorf("about z %g, want %g", r.Inertia[2][2], wantZ)
	}
}

func sq(x float64) float64 { return x * x }

// A density is set and cleared in one undo step each, kept by redo,
// and refused where it has no meaning.
func TestDensityUndoAndMisuse(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	box := d.Features.Extrude(10, "")
	step(d)
	box.SetDensity(7.85)
	step(d)
	box.SetDensity(0)
	if box.Density() != 0 {
		t.Fatalf("cleared density %g", box.Density())
	}
	d.Edit.Undo()
	if box.Density() != 7.85 {
		t.Errorf("after undo %g, want 7.85", box.Density())
	}
	d.Edit.Undo()
	if box.Density() != 0 {
		t.Errorf("after a second undo %g, want 0", box.Density())
	}
	d.Edit.Redo()
	if box.Density() != 7.85 {
		t.Errorf("after redo %g, want 7.85", box.Density())
	}

	sk := sketch.NewSketch()
	if err := sk.SetGroupDensity(box.H, 1); err == nil {
		t.Error("a density for a group that isn't there")
	}
	for _, bad := range []float64{-1, math.NaN(), math.Inf(1)} {
		if !refused(func() { box.SetDensity(bad) }) {
			t.Errorf("density %g taken", bad)
		}
	}
	rot := d.Features.Translate(2, api.V(20, 0, 0), "")
	if !refused(func() { rot.SetDensity(1) }) {
		t.Error("a step and repeat took a density")
	}
	if box.Density() != 7.85 {
		t.Errorf("a refused density changed it to %g", box.Density())
	}
}
