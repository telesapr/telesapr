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

// A sweep along an L with its corner rounded (F4): the straight runs
// cut back by r tan(θ/2) and joined by an arc of radius r, so the
// volume is the area times the rounded path's length, the profile's
// centroid riding on the path; sound and exact, a square profile and a
// round one. The radius is a length in the file, and an undo step.
func TestSweepRoundedCorner(t *testing.T) {
	t.Parallel()
	for _, round := range []bool{false, true} {
		d := api.New()
		if round {
			// Not saved: the round trip is the square's.
			d.Edit.SetChordTolerance(0.002)
		}
		path := cornerPath(d, 10, 12, 90)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		// The square's sides round the bend are curved, and their mesh
		// is chords of them.
		area, tol := 8.0, 5e-4
		if round {
			d.Sketch.Circle(api.V(0, 0, 0), 1.5)
			area, tol = math.Pi*2.25, 0.005
		} else {
			d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		}
		s := d.Features.Sweep(path, "")
		step(d)
		before := saved(t, d)
		s.AsSweep().SetCornerRadius(3)
		if p := s.Problem(); p != "" || s.AsSweep().CornerRadius() != 3 {
			t.Fatalf("round %v: %q, radius %g", round, p, s.AsSweep().CornerRadius())
		}
		r := s.Analyze().Check()
		want := area * (10 + 12 - 2*3 + 3*math.Pi/2)
		if !r.Sound() || math.Abs(r.Volume-want) > tol*want {
			t.Errorf("round %v: %s, want %g", round, r.String(), want)
		}
		if !round {
			undoRedo(t, d, before)
			if e := roundTrip(t, d); e.Groups()[len(e.Groups())-1].AsSweep().CornerRadius() != 3 {
				t.Error("the radius didn't come back")
			}
			if !strings.Contains(string(saved(t, d)), "corner-radius = 3") {
				t.Error("the file doesn't hold the radius")
			}
		}
		s.AsSweep().SetCornerRadius(0)
		if r := s.Analyze().Check(); s.Problem() != "" || math.Abs(r.Volume-area*22) > tol*area*22 {
			t.Errorf("round %v: mitred again: %q %s", round, s.Problem(), r.String())
		}
	}
}

// A line meeting an arc at a right angle: joined where the profile's
// paths cross, sound, its volume near the area times the path's length
// (the kernel's tests hold it to the slices'), or rounded. The arc runs
// three quarters of a
// turn from the line's top, over (0, 20), to (5, 15), turning the other
// way from the corner, so the rounded path is an S: the line cut back,
// the radius's arc tangent to the line and outside the arc's circle,
// and the arc cut back where it touches. Worked out here from that
// circle, the volume is the area times the path's length, within the
// mesh's chords of the curved faces.
func TestSweepRoundedBesideArc(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.002)
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	path := []api.Entity{
		d.Sketch.Line(api.V(0, 0, 0), api.V(0, 0, 10)),
		d.Sketch.Arc(api.V(0, 0, 15), api.V(0, 0, 10), api.V(5, 0, 15)),
	}
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-0.5, -1, 0), api.V(0.5, 1, 0))
	s := d.Features.Sweep(path, "")
	if p := s.Problem(); p != "" {
		t.Fatalf("unrounded: %q", p)
	}
	joined := 2 * (10 + 5*3*math.Pi/2)
	if v := s.Analyze().Check(); !v.Sound() || math.Abs(v.Volume-joined) > 0.01*joined {
		t.Errorf("unrounded: %s, want about %g", v.String(), joined)
	}
	const r = 2.0
	s.AsSweep().SetCornerRadius(r)
	if p := s.Problem(); p != "" {
		t.Fatal(p)
	}
	// The fillet's centre is r from the line and 5 + r from the arc's
	// centre (0, 15): at height cz.
	cz := 15 - math.Sqrt((5+r)*(5+r)-r*r)
	turn := math.Atan2(15-cz, r) // the fillet's angle
	cut := math.Atan2(r, 15-cz)  // the arc's, cut from its start
	length := cz + r*turn + 5*(3*math.Pi/2-cut)
	want := 2 * length
	if v := s.Analyze().Check(); !v.Sound() || math.Abs(v.Volume-want) > 1e-3*want {
		t.Errorf("%s, want %g", v.String(), want)
	}
}

// What a corner radius can't do is a problem on the group: too large
// for a short run, too small for the profile, which would reach the
// arc's axis.
func TestSweepRoundedRefused(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		along, r float64
		want     string
	}{
		"too large": {2, 5, "too large"},
		"too small": {12, 0.5, "too small"},
	} {
		d := api.New()
		path := cornerPath(d, 10, c.along, 90)
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
		s := d.Features.Sweep(path, "")
		s.AsSweep().SetCornerRadius(c.r)
		if p := s.Problem(); !strings.Contains(p, c.want) {
			t.Errorf("%s: %q, want %q", name, p, c.want)
		}
	}
	d := api.New()
	path := cornerPath(d, 10, 12, 90)
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(-1, -2, 0), api.V(1, 2, 0))
	s := d.Features.Sweep(path, "")
	func() {
		defer func() {
			if err, ok := recover().(error); !ok || !strings.Contains(err.Error(), "a length of 0 or more") {
				t.Errorf("a negative radius: %v", err)
			}
		}()
		s.AsSweep().SetCornerRadius(-1)
	}()
	if err := s.SetExpr(sketch.GroupPitch, "1+2"); err != nil || s.AsSweep().CornerRadius() != 3 {
		t.Errorf("an expression: %v, %g", err, s.AsSweep().CornerRadius())
	}
}

// TestSweepRoundedSweep sweeps square and round profiles round corners
// of several angles, mitred (r = 0) or rounded with two radii, and
// requires sound solids of the area times the rounded path's length.
// The rate allowed was fixed at none before the mitre was written, and
// again before the rounding was.
func TestSweepRoundedSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Strings("profile", "square", "circle"),
			sweep.Floats("angle", 20, 45, 90, 120, 150),
			sweep.Floats("r", 0, 2, 5),
			sweep.Floats("along", 6, 20),
		},
		MaxFailRate: 0,
		Sample:      16,
	}
	grid.Run(func(p sweep.Point) error {
		d := api.New()
		d.Edit.SetChordTolerance(0.002)
		along, r, th := p.Float("along"), p.Float("r"), p.Float("angle")*math.Pi/180
		path := cornerPath(d, 10, along, p.Float("angle"))
		d.Features.SketchIn(d.Workplane("XY"), "profile")
		area := 0.0
		if p.String("profile") == "square" {
			d.Sketch.Rect(api.V(-1, -1.5, 0), api.V(1, 1.5, 0))
			area = 6
		} else {
			d.Sketch.Circle(api.V(0, 0, 0), 1.5)
			area = math.Pi * 2.25
		}
		s := d.Features.Sweep(path, "")
		if r > 0 {
			s.AsSweep().SetCornerRadius(r)
		}
		cut := r * math.Tan(th/2)
		if cut >= along-1e-6 || cut >= 10-1e-6 {
			if !strings.Contains(s.Problem(), "too large") {
				return fmt.Errorf("a radius too large for the runs: %q", s.Problem())
			}
			return nil
		}
		rr := s.Analyze().Check()
		want := area * (10 + along - 2*cut + r*th)
		// The square's sides round the bend are curved, and their mesh
		// is chords of them; at a mitre they are flat.
		tol := 5e-4
		switch {
		case p.String("profile") == "circle":
			tol = 0.005
		case r == 0:
			tol = 1e-6
		}
		switch {
		case s.Problem() != "":
			return fmt.Errorf("problem: %s", s.Problem())
		case !rr.Sound():
			return fmt.Errorf("unsound: %s", rr.String())
		case math.Abs(rr.Volume-want) > tol*want:
			return fmt.Errorf("volume %g, want %g", rr.Volume, want)
		}
		return nil
	}).Require(t)
}
