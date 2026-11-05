package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// partwayPath draws in ZX a path whose first piece is a line up the z
// axis from z0 to z0+15, then, by kind: a quarter turn of radius 8 and
// a line along x ("arc"); a line along x at a corner ("corner"); a
// cubic leaving tangent ("cubic"); or a cubic leaving at a corner, which
// is swept in runs ("cubic-corner").
func partwayPath(d *api.Doc, kind string, z0 float64) []api.Entity {
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	top := api.V(0, 0, z0+15)
	up := d.Sketch.Line(api.V(0, 0, z0), top)
	switch kind {
	case "corner":
		return []api.Entity{up, d.Sketch.Line(top, top.Plus(api.V(12, 0, 0)))}
	case "cubic":
		return []api.Entity{up, d.Sketch.Cubic(top, top.Plus(api.V(0, 0, 6)), top.Plus(api.V(6, 0, 12)), top.Plus(api.V(14, 0, 12)))}
	case "cubic-corner":
		return []api.Entity{up, d.Sketch.Cubic(top, top.Plus(api.V(6, 0, 2)), top.Plus(api.V(10, 0, 8)), top.Plus(api.V(16, 0, 8)))}
	}
	return []api.Entity{up,
		d.Sketch.Arc(api.V(8, 0, z0+15), top, api.V(8, 0, z0+23)),
		d.Sketch.Line(api.V(8, 0, z0+23), api.V(20, 0, z0+23))}
}

// A profile drawn across an open path partway along it is swept along
// the whole path, the solid passing through the profile where it is
// drawn: the same solid as the path moved up by 5 swept from a profile
// at its start, moved down by 5. It was moved as if drawn at the path's
// start, the solid offset along the path by 5.
func TestSweepProfilePartway(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, path string
		up         bool
	}{
		{"rigid", "arc", false},
		{"mitred", "corner", false},
		{"held", "arc", true},
		{"cubic", "cubic", false},
		{"cubic corner", "cubic-corner", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			build := func(z0 float64) (*api.Doc, api.Group) {
				d := api.New()
				path := partwayPath(d, c.path, z0)
				d.Features.SketchIn(d.Workplane("XY"), "profile")
				d.Sketch.Rect(api.V(-2, -1.5, 0), api.V(2, 1.5, 0))
				step(d)
				s := d.Features.Sweep(path, "")
				if c.up {
					s.AsSweep().SetSweepUp(api.V(0, 1, 0))
				}
				if p := s.Problem(); p != "" {
					t.Fatalf("z0 %g: %s", z0, p)
				}
				return d, s
			}
			d, s := build(-5)
			_, ref := build(0)
			r, want := s.Analyze().Check(), ref.Analyze().Check()
			if !r.Sound() {
				t.Fatalf("partway: %s", r.String())
			}
			shift := api.V(0, 0, 5)
			com := want.CenterOfMass.Minus(shift)
			// The two solids are the same sweep, moved, but meshed apart:
			// the constrained Delaunay mesher's points inside a curved face
			// fall by rounding, so each mesh keeps only within the chord
			// tolerance of the surface. The swept sides' area is under the
			// profile's perimeter (14) times the reference's diagonal.
			wlo, whi := ref.Analyze().Bounds()
			var chord float64
			d.With(func(sk *sketch.Sketch) { chord = sk.Tol().ChordTolMm() })
			slack := chord * 14 * whi.Minus(wlo).Magnitude()
			if math.Abs(r.Volume-want.Volume) > slack || r.CenterOfMass.Minus(com).Magnitude() > slack/want.Volume*whi.Minus(wlo).Magnitude() {
				t.Errorf("partway: volume %g centre %v, want %g %v", r.Volume, r.CenterOfMass, want.Volume, com)
			}
			lo, hi := s.Analyze().Bounds()
			if lo.Minus(wlo.Minus(shift)).Magnitude() > 1e-6 || hi.Minus(whi.Minus(shift)).Magnitude() > 1e-6 {
				t.Errorf("partway: bounds %v %v, want %v %v", lo, hi, wlo.Minus(shift), whi.Minus(shift))
			}
			// The cap at the start is a copy of the profile at the path's
			// start, its corner at (-2, -1.5, -5).
			found := false
			for _, e := range s.Entities() {
				if e.Kind() == "point" && e.Pos().Minus(api.V(-2, -1.5, -5)).Magnitude() < 1e-9 {
					found = true
				}
			}
			if !found {
				t.Errorf("no copy of the profile at the path's start")
			}
			switch {
			case c.path == "cubic-corner":
				// The oracle refuses corners beside a cubic.
			case (testing.Short() || fixtures.RaceDetector) && (c.up || c.path == "cubic"):
				// The oracle's own frames along a fitted path take a minute,
				// ten under the race detector.
			default:
				if rc := s.Analyze().RayCheck(); len(rc.Disagree) > 0 {
					t.Errorf("the ray-cast oracle disagrees on %d of %d rays", len(rc.Disagree), rc.Rays)
				}
			}
			roundTrip(t, d)
		})
	}
}

// Twisted, a profile drawn partway along the path is the section there:
// the same solid as one drawn at the start turned back by the share of
// the twist the frames turn by up to it, the path moved up by 5.
func TestSweepProfilePartwayTwisted(t *testing.T) {
	t.Parallel()
	const twist = 60.0
	square := func(d *api.Doc, turn float64) {
		var pts []api.Vec
		for _, p := range []api.Vec{api.V(-2, -1.5, 0), api.V(2, -1.5, 0), api.V(2, 1.5, 0), api.V(-2, 1.5, 0)} {
			pts = append(pts, p.RotatedAroundAxis(api.V(0, 0, 1), turn))
		}
		d.Sketch.Polyline(true, pts...)
	}
	build := func(z0, turn float64) api.Group {
		d := api.New()
		path := partwayPath(d, "arc", z0)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		square(d, turn)
		step(d)
		s := d.Features.Sweep(path, "")
		s.AsSweep().SetTwist(twist)
		if p := s.Problem(); p != "" {
			t.Fatalf("z0 %g: %s", z0, p)
		}
		return s
	}
	s := build(-5, 0)
	r := s.Analyze().Check()
	if !r.Sound() {
		t.Fatalf("partway: %s", r.String())
	}
	length := 15 + 4*math.Pi + 12
	ref := build(0, -twist*math.Pi/180*5/length).Analyze().Check()
	com := ref.CenterOfMass.Minus(api.V(0, 0, 5))
	if math.Abs(r.Volume-ref.Volume) > 1e-3*ref.Volume || r.CenterOfMass.Minus(com).Magnitude() > 2e-3 {
		t.Errorf("partway: volume %g centre %v, want %g %v", r.Volume, r.CenterOfMass, ref.Volume, com)
	}
}

// A profile drawn across the far end of an open path, as at either end
// of a U whose ends are both on its plane, is swept from that end: it
// was swept from the other, the solid offset by the gap between them.
func TestSweepProfileAtFarEnd(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	path := []api.Entity{
		d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 10)),
		d.Sketch.Arc(api.V(8, 0, 10), api.V(0, 0, 10), api.V(16, 0, 10)),
		d.Sketch.Line(api.V(16, 0, 10), api.V(16, 0, 0)),
	}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Circle(api.V(16, 0, 0), 2)
	step(d)
	s := d.Features.Sweep(path, "")
	if p := s.Problem(); p != "" {
		t.Fatal(p)
	}
	r := s.Analyze().Check()
	want := math.Pi * 4 * (20 + 8*math.Pi)
	if !r.Sound() || math.Abs(r.Volume-want) > 0.01*want {
		t.Fatalf("U: %s, want volume %g", r.String(), want)
	}
	if lo, hi := s.Analyze().Bounds(); math.Abs(lo.X+2) > 1e-6 || math.Abs(hi.X-18) > 1e-6 || math.Abs(lo.Z) > 1e-6 || math.Abs(hi.Z-20) > 1e-6 {
		t.Errorf("U bounds %v %v, want x -2 to 18, z 0 to 20", lo, hi)
	}
}

// A profile drawn across a sharp corner of an open path would be cut by
// the corner's mitre: refused, saying so.
func TestSweepProfileAtCorner(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	path := []api.Entity{
		d.Sketch.Line(api.V(0, 0, -10), api.V(0, 0, 0)),
		d.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 10)),
	}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Circle(api.V(0, 0, 0), 2)
	step(d)
	defer func() {
		err, ok := recover().(error)
		if !ok || !strings.Contains(err.Error(), "at a corner of the path") {
			t.Errorf("%v, want an error saying the profile is at a corner", err)
		}
	}()
	d.Features.Sweep(path, "")
}
