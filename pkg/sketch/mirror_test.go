package sketch_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// block draws a rectangle in XY from (x0, y0) to (x1, y1) and extrudes
// it by depth.
func block(d *api.Doc, x0, y0, x1, y1, depth float64) api.Group {
	d.Rect(api.V(x0, y0, 0), api.V(x1, y1, 0))
	return d.Extrude(depth, "")
}

func TestMirrorApart(t *testing.T) {
	d := api.New()
	block(d, 10, 0, 20, 10, 5)
	m := d.Mirror(d.Workplane("YZ"), "")
	if p := m.Problem(); p != "" {
		t.Fatalf("problem: %s", p)
	}
	r := m.Check()
	if !r.Sound() || math.Abs(r.Volume-1000) > 1e-6 || r.Faces != 12 {
		t.Fatalf("mirror of a block apart: %s", r.String())
	}
	if lo, hi := m.Bounds(); math.Abs(lo.X+20) > 1e-9 || math.Abs(hi.X-20) > 1e-9 {
		t.Errorf("bounds %v %v", lo, hi)
	}
	if m.Dof() != 0 {
		t.Errorf("dof %d", m.Dof())
	}
}

// A block touching the plane joins its reflection into one block of six
// faces of one patch each. The two faces that met on the plane are
// left by the union with no trim, and dropped (roadmap 13.3); before,
// they made 8 patches for 6 faces.
func TestMirrorTouching(t *testing.T) {
	d := api.New()
	block(d, 0, 0, 10, 10, 5)
	m := d.Mirror(d.Workplane("YZ"), "")
	r := m.Check()
	if !r.Sound() || math.Abs(r.Volume-1000) > 1e-6 || r.Faces != 6 || r.Patches != 6 {
		t.Fatalf("mirror of a block on the plane: %s", r.String())
	}
	// Without the original, only the reflection is left.
	m.SetKeepOriginal(false)
	r = m.Check()
	if !r.Sound() || math.Abs(r.Volume-500) > 1e-6 || r.Faces != 6 || math.Abs(r.Max.X) > 1e-9 {
		t.Fatalf("the reflection alone: %s", r.String())
	}
	if m.KeepOriginal() {
		t.Errorf("KeepOriginal after SetKeepOriginal(false)")
	}
}

// A mirrored pocket is cut too, and a hole's circle is reflected.
func TestMirrorPocket(t *testing.T) {
	d := api.New()
	block(d, -20, 0, 20, 10, 5)
	d.SketchIn(d.Workplane("XY"), "")
	hole := d.Circle(api.V(10, 5, 0), 2)
	cut := d.Extrude(5, "")
	cut.SetCombine(api.Difference)
	m := d.Mirror(d.Workplane("YZ"), "")
	r := m.Check()
	want := 40*10*5 - 2*math.Pi*4*5
	if !r.Sound() || math.Abs(r.Volume-want) > want*1e-3 {
		t.Fatalf("mirrored pocket: %s, want volume %g", r.String(), want)
	}
	// The copies of the hole's circles sit at x = -10.
	found := 0
	for _, e := range m.Entities() {
		if e.Kind() == "circle" {
			if c := e.Center().Pos(); math.Abs(c.X+10) < 1e-9 && math.Abs(c.Y-5) < 1e-9 {
				found++
			}
			if math.Abs(e.Radius()-2) > 1e-9 {
				t.Errorf("mirrored circle of radius %g", e.Radius())
			}
		}
	}
	if found != 2 {
		t.Errorf("%d mirrored circles at (-10, 5), want 2", found)
	}
	_ = hole
}

// The copies follow the plane when it moves.
func TestMirrorFollowsPlane(t *testing.T) {
	d := api.New()
	d.Free3D()
	wp := d.NewWorkplane(api.V(30, 0, 0), api.V(0, 1, 0), api.V(0, 0, 1))
	d.SetWorkplane(d.Workplane("XY"))
	block(d, 10, 0, 20, 10, 5)
	m := d.Mirror(wp, "")
	if lo, hi := m.Bounds(); math.Abs(lo.X-10) > 1e-6 || math.Abs(hi.X-50) > 1e-6 {
		t.Fatalf("bounds %v %v, want x from 10 to 50", lo, hi)
	}
	wp.Points()[0].MoveTo(api.V(25, 0, 0))
	d.Regen()
	if lo, hi := m.Bounds(); math.Abs(lo.X-10) > 1e-6 || math.Abs(hi.X-40) > 1e-6 {
		t.Fatalf("after the move: bounds %v %v, want x from 10 to 40", lo, hi)
	}
	if r := m.Check(); !r.Sound() || math.Abs(r.Volume-1000) > 1e-6 {
		t.Errorf("after the move: %s", r.String())
	}
}

// A planar face of the solid can be the plane.
func TestMirrorInFace(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 5)
	var top api.Entity
	for _, f := range ex.Faces() {
		if p := f.Points(); len(p) > 0 && math.Abs(p[0].Pos().Z-5) < 1e-9 {
			top = f
		}
	}
	if !top.Valid() {
		t.Fatal("no top face")
	}
	m := d.Mirror(top, "")
	r := m.Check()
	if !r.Sound() || math.Abs(r.Volume-1000) > 1e-6 || math.Abs(r.Max.Z-10) > 1e-9 {
		t.Fatalf("mirror in the top face: %s", r.String())
	}
}

func TestMirrorMisuse(t *testing.T) {
	d := api.New()
	line := d.Line(api.V(0, 0, 0), api.V(1, 0, 0))
	ex := block(d, 0, 0, 10, 10, 5)
	for name, fn := range map[string]func(){
		"a line as the plane": func() { d.Mirror(line, "") },
		"keep on an extrude":  func() { ex.SetKeepOriginal(true) },
		"plane of extrude":    func() { ex.Plane() },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
}

// A D-shaped profile, a line and an arc, reflects into a sound solid
// of twice the volume; the file comes back the same, and the mirror is
// one undo step.
func TestMirrorArcRoundTripUndo(t *testing.T) {
	d := api.New()
	d.Line(api.V(10, -5, 0), api.V(10, 5, 0))
	d.Arc(api.V(10, 0, 0), api.V(10, 5, 0), api.V(10, -5, 0))
	ex := d.Extrude(4, "")
	half := ex.Check().Volume
	step(d)
	before := saved(t, d)
	m := d.Mirror(d.Workplane("YZ"), "twin")
	r := m.Check()
	if !r.Sound() || math.Abs(r.Volume-2*half) > 1e-9*half {
		t.Fatalf("mirrored D: %s, want volume %g", r.String(), 2*half)
	}
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	if bm := back.Group("twin"); bm.Kind() != "mirror" || math.Abs(bm.Check().Volume-2*half) > 1e-9*half {
		t.Fatalf("the reopened mirror is %s with volume %g", bm.Kind(), bm.Check().Volume)
	}
}

// A mirror of a solid forced to a mesh mirrors the mesh.
func TestMirrorMesh(t *testing.T) {
	d := api.New()
	ex := block(d, 10, 0, 20, 10, 5)
	forceToMesh(d, ex)
	m := d.Mirror(d.Workplane("YZ"), "")
	r := m.Check()
	if !r.Sound() || math.Abs(r.Volume-1000) > 1e-6 {
		t.Fatalf("mirrored mesh: %s", r.String())
	}
}

// A step and repeat of a mirror, and a mirror of a mirror.
func TestMirrorChained(t *testing.T) {
	d := api.New()
	block(d, 10, 0, 20, 10, 5)
	d.Mirror(d.Workplane("YZ"), "")
	m2 := d.Mirror(d.Workplane("ZX"), "")
	r := m2.Check()
	if !r.Sound() || math.Abs(r.Volume-2000) > 1e-6 {
		t.Fatalf("mirror of a mirror: %s", r.String())
	}
	tr := d.Translate(2, api.V(0, 0, 10), "")
	if r := tr.Check(); !r.Sound() || math.Abs(r.Volume-4000) > 1e-6 {
		t.Fatalf("translate of a mirror: %s", r.String())
	}
}

// TestMirrorSweep mirrors a block, with a hole through it or not, in
// the YZ plane, at gaps from the plane down to touching it, with the
// original kept or not, and requires a sound solid of twice the
// block's volume, or once without the original. The rate allowed, none,
// was fixed before the sweep was run (roadmap 12.8); a mirror in a
// plane of the solid's own, whose faces coincide, is 13.4's.
func TestMirrorSweep(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("gap", 0, 0.5, 5),
			sweep.Floats("w", 1, 10),
			sweep.Floats("h", 1, 10),
			sweep.Floats("depth", 0.5, 5, 20),
			sweep.Bools("hole"),
			sweep.Bools("keep"),
		},
		MaxFailRate: 0,
		Sample:      16,
	}
	grid.Run(func(p sweep.Point) error {
		gap, w, h, depth := p.Float("gap"), p.Float("w"), p.Float("h"), p.Float("depth")
		// The hole is in the block's profile: a mirror reflects its
		// source group, the extrusion, and not the solid so far (15.5).
		d := api.New()
		fineChord(d)
		d.Rect(api.V(gap, 0, 0), api.V(gap+w, h, 0))
		want := w * h * depth
		if p.Bool("hole") {
			r := math.Min(w, h) / 4
			d.Circle(api.V(gap+w/2, h/2, 0), r)
			want -= math.Pi * r * r * depth
		}
		d.Extrude(depth, "")
		m := d.Mirror(d.Workplane("YZ"), "")
		if p.Bool("keep") {
			want *= 2
		} else {
			m.SetKeepOriginal(false)
		}
		r := m.Check()
		switch {
		case m.Problem() != "":
			return fmt.Errorf("problem: %s", m.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case math.Abs(r.Volume-want) > 5e-3*want:
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}

// TestMirrorSymmetricSweep mirrors parts symmetric about the plane, with
// the original kept: a circle on it, a block with a hole on it or a
// pair either side, and a slot, over their sizes. Each reflection is
// the part itself, which a union would put through as coincident
// faces, curved ones too; the mirror keeps the part as it is. The rate
// allowed, none, was set before (roadmap 13.4). An L whose bounding
// box is symmetric but which is not, reflected, joins its reflection.
func TestMirrorSymmetricSweep(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("shape", "circle", "hole", "holes", "slot"),
			sweep.Floats("size", 1, 4, 15),
			sweep.Floats("depth", 0.5, 5),
			sweep.Strings("plane", "YZ", "ZX"),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		s, depth := p.Float("size"), p.Float("depth")
		d := api.New()
		fineChord(d)
		// Symmetric about both x = 0 and y = 0.
		var want float64
		switch p.String("shape") {
		case "circle":
			d.Circle(api.V(0, 0, 0), s)
			want = math.Pi * s * s
		case "hole":
			d.Rect(api.V(-2*s, -s, 0), api.V(2*s, s, 0))
			d.Circle(api.V(0, 0, 0), s/2)
			want = 8*s*s - math.Pi*s*s/4
		case "holes":
			d.Rect(api.V(-2*s, -s, 0), api.V(2*s, s, 0))
			d.Circle(api.V(-s, 0, 0), s/2)
			d.Circle(api.V(s, 0, 0), s/2)
			want = 8*s*s - math.Pi*s*s/2
		case "slot":
			d.Rect(api.V(-2*s, -s, 0), api.V(2*s, s, 0))
			d.Rect(api.V(-s, -s/2, 0), api.V(s, s/2, 0))
			want = 8*s*s - 2*s*s
		}
		d.Extrude(depth, "")
		want *= depth
		m := d.Mirror(d.Workplane(p.String("plane")), "")
		r := m.Check()
		switch {
		case m.Problem() != "":
			return fmt.Errorf("problem: %s", m.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case math.Abs(r.Volume-want) > 5e-3*want:
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)

	// An L in a square, symmetric only in its box: the reflection is
	// another solid, and the two join, a U: the bottom strip, 10 by 3,
	// and a side strip 3 by 7 either side.
	d := api.New()
	d.Polyline(true, api.V(-5, -5, 0), api.V(5, -5, 0), api.V(5, -2, 0), api.V(-2, -2, 0), api.V(-2, 5, 0), api.V(-5, 5, 0))
	d.Extrude(3, "")
	m := d.Mirror(d.Workplane("YZ"), "")
	if r := m.Check(); !r.Sound() || math.Abs(r.Volume-3*(30+21+21)) > 1e-6 {
		t.Errorf("the L and its reflection: %s, want volume %g", r.String(), 3.0*(30+21+21))
	}
}

// A mirror of the whole solid so far: a block and a hole cut in it by
// a later group, mirrored from the cut, reflects both, where a mirror
// of the cut alone would not (roadmap 15.5); kept, the two halves; not
// kept, the reflection alone. One undo step each way, and a round trip.
func TestMirrorSolidSoFar(t *testing.T) {
	d := api.New()
	block(d, 5, 0, 15, 10, 5)
	d.SketchIn(d.Workplane("XY"), "")
	d.Circle(api.V(10, 5, 0), 2)
	cut := d.Extrude(5, "")
	cut.SetCombine(api.Difference)
	half := 500 - math.Pi*4*5
	m := d.Mirror(d.Workplane("YZ"), "")
	step(d)
	before := saved(t, d)
	m.SetMirrorSolid(true)
	if !m.MirrorSolid() {
		t.Fatal("not set")
	}
	r := m.Check()
	if m.Problem() != "" || !r.Sound() || math.Abs(r.Volume-2*half) > 1e-2*half || math.Abs(r.Min.X+15) > 1e-9 {
		t.Fatalf("the whole solid mirrored: %s, want %g", r.String(), 2*half)
	}
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	if br := back.Group(m.Name()).Check(); math.Abs(br.Volume-r.Volume) > 1e-9*r.Volume {
		t.Errorf("reopened: %s", br.String())
	}
	m.SetKeepOriginal(false)
	r = m.Check()
	if !r.Sound() || math.Abs(r.Volume-half) > 1e-2*half || r.Max.X > 1e-9 {
		t.Errorf("the reflection alone: %s, want %g", r.String(), half)
	}
}
