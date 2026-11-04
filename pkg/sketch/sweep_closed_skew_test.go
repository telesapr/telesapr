package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Closed paths out of a plane with sharp corners, each the profile's
// start at the origin on a first side up z: a skew quadrilateral and a
// skew pentagon, whose frames come back turned round them.
var (
	skewFour = []api.Vec{api.V(0, 0, -5), api.V(0, 0, 5), api.V(12, 3, 5), api.V(9, 10, -4)}
	skewFive = []api.Vec{api.V(0, 0, -5), api.V(0, 0, 5), api.V(10, 2, 6), api.V(12, 9, -1), api.V(3, 7, -6)}
)

// skewPath draws the closed polyline through pts in 3d.
func skewPath(d *api.Doc, pts []api.Vec) []api.Entity {
	d.Features.Sketch3D("path")
	var path []api.Entity
	for i := range pts {
		path = append(path, d.Sketch.Line(pts[i], pts[(i+1)%len(pts)]))
	}
	return path
}

// skewLength is the length of the closed polyline through pts, its
// corners rounded by r: each corner turning by θ takes 2r·tan(θ/2) of
// the lines and adds an arc of rθ.
func skewLength(pts []api.Vec, r float64) float64 {
	n := len(pts)
	l := 0.0
	for i := range pts {
		a, b, c := pts[(i+n-1)%n], pts[i], pts[(i+1)%n]
		l += c.Minus(b).Magnitude()
		d1, d2 := b.Minus(a).WithMagnitude(1), c.Minus(b).WithMagnitude(1)
		theta := math.Acos(math.Max(-1, math.Min(1, d1.Dot(d2))))
		l += r * (theta - 2*math.Tan(theta/2))
	}
	return l
}

// Sweeps round closed paths that leave a plane and turn sharply,
// refused before: the frames come back turned, so the sweep is fitted,
// the turn taken out along the path and held still beside each corner
// as a twist is, and each corner mitred, or rounded by a corner radius.
// Each sound, with no caps, of the profile's area times the path's
// length, the same after a save and load.
func TestSweepClosedSkewCorners(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("fitted sweeps round skew polygons")
	}
	for _, c := range []struct {
		name   string
		pts    []api.Vec
		circle bool
		radius float64
		twist  float64
		chord  float64
		within float64
	}{
		{name: "four, square", pts: skewFour, chord: 0.01, within: 1e-3},
		{name: "five, square", pts: skewFive, chord: 0.01, within: 1e-3},
		{name: "five, circle", pts: skewFive, circle: true, chord: 0.01, within: 5e-3},
		{name: "four, square, rounded", pts: skewFour, radius: 2, chord: 0.01, within: 2e-3},
		{name: "five, circle, rounded", pts: skewFive, circle: true, radius: 2, chord: 0.01, within: 5e-3},
		{name: "four, square, twisted", pts: skewFour, twist: 360, chord: 0.01, within: 2e-3},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			fineChord(d, c.chord)
			path := skewPath(d, c.pts)
			d.Features.SketchIn(d.Workplane("XY"), "profile")
			area := 4.0
			if c.circle {
				d.Sketch.Circle(api.V(0, 0, 0), 1)
				area = math.Pi
			} else {
				squareAt(d, api.V(0, 0, 0), 2)
			}
			step(d)
			s := d.Features.Sweep(path, "")
			if c.radius > 0 {
				s.AsSweep().SetCornerRadius(c.radius)
			}
			if c.twist != 0 {
				s.AsSweep().SetTwist(c.twist)
			}
			if p := s.Problem(); p != "" {
				t.Fatal(p)
			}
			var fitted bool
			d.With(func(sk *sketch.Sketch) {
				sp, err := sk.Group(s.H).FittedSweepPath(sk)
				fitted = err == nil && sp != nil
			})
			if !fitted {
				t.Error("the sweep is not fitted: the path's frames came back as they left")
			}
			want := area * skewLength(c.pts, c.radius)
			r := s.Analyze().Check()
			if !r.Sound() || math.Abs(r.Volume-want) > c.within*want {
				t.Fatalf("%s, want volume %g", r.String(), want)
			}
			for _, e := range s.Entities() {
				if e.Kind() == "face" {
					t.Errorf("a closed sweep has a cap face")
				}
			}
			d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
			roundTrip(t, d)
		})
	}
}

// A sweep round a closed skew path is one undo step, and comes back
// identical on redo.
func TestSweepClosedSkewUndo(t *testing.T) {
	t.Parallel()
	d := api.New()
	path := skewPath(d, skewFour)
	squareProfile(d, 2)
	step(d)
	before := saved(t, d)
	d.Features.Sweep(path, "")
	undoRedo(t, d, before)
}

// A closed path out of a plane with a sharp corner beside an arc: in the
// arc's plane the corner is joined as on an open path; out of it, it is
// refused by name, the corner radius offered.
func TestSweepClosedSkewArc(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		tail []api.Vec
		want string
	}{
		"corner in the arc's plane":     {tail: []api.Vec{api.V(14, 0, 0), api.V(8, 8, -6)}},
		"corner out of the arc's plane": {tail: []api.Vec{api.V(12, 6, -3)}, want: "out of the plane of the arc"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := api.New()
			fineChord(d, 0.01)
			d.Features.Sketch3D("path")
			// Up z, then half a circle over the top in y = 0, its ends'
			// tangents along z, then lines back out of that plane.
			a, b, e := api.V(0, 0, -5), api.V(0, 0, 5), api.V(10, 0, 5)
			path := []api.Entity{d.Sketch.Line(a, b),
				d.Sketch.ArcIn3D(api.V(5, 0, 5), b, e, api.V(1, 0, 0), api.V(0, 0, -1))}
			prev, length := e, 10+5*math.Pi
			for _, p := range append(c.tail, a) {
				path = append(path, d.Sketch.Line(prev, p))
				length += p.Minus(prev).Magnitude()
				prev = p
			}
			squareProfile(d, 2)
			step(d)
			var msg string
			var s api.Group
			func() {
				defer func() {
					if err, ok := recover().(error); ok {
						msg = err.Error()
					}
				}()
				s = d.Features.Sweep(path, "")
				msg = s.Problem()
			}()
			if c.want != "" {
				if !strings.Contains(msg, c.want) {
					t.Errorf("%q, want a problem saying %q", msg, c.want)
				}
				return
			}
			if msg != "" {
				t.Fatal(msg)
			}
			want := 4 * length
			if r := s.Analyze().Check(); !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want {
				t.Fatalf("%s, want volume %g", r.String(), want)
			}
		})
	}
}

// TestSweepClosedSkewSweep sweeps a square or a circle round the skew
// quadrilateral and pentagon, sharp or rounded, twisted or not, and
// requires a sound solid of the profile's area times the path's length.
// The rate allowed was fixed at none before the feature was written.
func TestSweepClosedSkewSweep(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("fitted sweeps round skew polygons")
	}
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("path", "four", "five"),
			sweep.Strings("profile", "square", "circle"),
			sweep.Floats("size", 1, 2),
			sweep.Floats("radius", 0, 2),
			sweep.Floats("twist", 0, 360),
		},
		MaxFailRate: 0,
		Sample:      8,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		d.Edit.SetChordTolerance(0.01)
		pts := skewFour
		if p.String("path") == "five" {
			pts = skewFive
		}
		path := skewPath(d, pts)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		size := p.Float("size")
		// A twisted or round surface is fitted to the chord tolerance
		// and its volume follows it: within 5e-3 at 0.01%, closing on
		// the exact one as the tolerance is made finer.
		area, within := size*size, 2e-3
		if p.String("profile") == "square" {
			squareAt(d, api.V(0, 0, 0), size)
		} else {
			d.Sketch.Circle(api.V(0, 0, 0), size/2)
			area, within = math.Pi*size*size/4, 5e-3
		}
		if p.Float("twist") != 0 {
			within = 5e-3
		}
		s := d.Features.Sweep(path, "")
		if r := p.Float("radius"); r > 0 {
			s.AsSweep().SetCornerRadius(r)
		}
		if tw := p.Float("twist"); tw != 0 {
			s.AsSweep().SetTwist(tw)
		}
		r := s.Analyze().Check()
		want := area * skewLength(pts, p.Float("radius"))
		switch {
		case s.Problem() != "":
			return fmt.Errorf("problem: %s", s.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case math.Abs(r.Volume-want) > within*want:
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}

// BenchmarkSweepClosedSkew regenerates a square swept round the skew
// quadrilateral, fitted and mitred.
func BenchmarkSweepClosedSkew(b *testing.B) {
	benchRegen(b, func(d *api.Doc) {
		path := skewPath(d, skewFour)
		squareProfile(d, 2)
		d.Features.Sweep(path, "")
	})
}
