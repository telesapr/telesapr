package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
)

// pipePath draws in ZX a line up from the origin by up, a quarter turn
// of radius r towards +x, and a line along x by along.
func pipePath(d *api.Doc, up, r, along float64) []api.Entity {
	d.SketchIn(d.Workplane("ZX"), "path")
	return []api.Entity{
		d.Line(api.V(0, 0, 0), api.V(0, 0, up)),
		d.Arc(api.V(r, 0, up), api.V(0, 0, up), api.V(r, 0, up+r)),
		d.Line(api.V(r, 0, up+r), api.V(r+along, 0, up+r)),
	}
}

func TestSweepPipe(t *testing.T) {
	d := api.New()
	path := pipePath(d, 10, 8, 12)
	d.SketchIn(d.Workplane("XY"), "profile")
	d.Circle(api.V(0, 0, 0), 2)
	step(d)
	before := saved(t, d)
	// The path is given in any order.
	s := d.Sweep([]api.Entity{path[2], path[0], path[1]}, "")
	if p := s.Problem(); p != "" {
		t.Fatal(p)
	}
	r := s.Check()
	want := math.Pi * 4 * (10 + 8*math.Pi/2 + 12)
	if !r.Sound() || math.Abs(r.Volume-want) > 0.01*want {
		t.Fatalf("pipe: %s, want volume %g", r.String(), want)
	}
	if lo, hi := s.Bounds(); math.Abs(hi.X-20) > 1e-6 || math.Abs(hi.Z-20) > 1e-6 || math.Abs(lo.Z) > 1e-6 {
		t.Errorf("pipe bounds %v %v", lo, hi)
	}
	// The end cap's copy of the circle is at the end of the path,
	// turned to face along x.
	found := false
	for _, e := range s.Entities() {
		if e.Kind() == "circle" {
			if c := e.Center().Pos(); c.Minus(api.V(20, 0, 18)).Magnitude() < 1e-9 {
				found = true
				if n := e.Normal().Dir(); math.Abs(math.Abs(n.X)-1) > 1e-9 {
					t.Errorf("the end circle's normal is %v", n)
				}
			}
		}
	}
	if !found {
		t.Errorf("no copy of the profile at the end of the path")
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
}

func TestSweepProblems(t *testing.T) {
	d := api.New()
	d.SketchIn(d.Workplane("ZX"), "path")
	up := d.Line(api.V(0, 0, 0), api.V(0, 0, 10))
	corner := d.Line(api.V(0, 0, 10), api.V(10, 0, 10))
	apart := d.Line(api.V(0, 0, 20), api.V(0, 0, 30))
	flat := d.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	tight := d.Arc(api.V(1, 0, 10), api.V(0, 0, 10), api.V(1, 0, 11))
	branch := d.Line(api.V(0, 0, 10), api.V(0, 0, 20))
	d.SketchIn(d.Workplane("XY"), "profile")
	circle := d.Circle(api.V(0, 0, 0), 2)
	for name, c := range map[string]struct {
		path []api.Entity
		want string
	}{
		"corner":    {[]api.Entity{up, corner}, "corner"},
		"apart":     {[]api.Entity{up, apart}, "not one chain"},
		"flat":      {[]api.Entity{flat}, "along the profile's plane"},
		"tight":     {[]api.Entity{up, tight}, "too tight"},
		"circle":    {[]api.Entity{circle}, "not a line segment or an arc"},
		"no path":   {nil, "no path"},
		"branching": {[]api.Entity{up, corner, branch}, "branches"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			d.Sweep(c.path, "")
		}()
	}
}

// TestSweepGrid sweeps a circle of radius rho along a line of length
// up (down, for a negative one), a turn of radius r by angle degrees
// and a straight tail, and requires a sound solid of the volume
// Pappus gives. The rate allowed was fixed at none before the sweep
// was written; see sweepGridMaxFailRate for what it is.
func TestSweepGrid(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("rho", 0.5, 2),
			sweep.Floats("r", 5, 10, 30),
			sweep.Floats("angle", 30, 90, 150),
			sweep.Floats("up", 10, -10),
			sweep.Floats("tilt", 0, 25),
		},
		MaxFailRate: sweepGridMaxFailRate,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		rho, r, up := p.Float("rho"), p.Float("r"), p.Float("up")
		th := p.Float("angle") * math.Pi / 180
		tail := 5.0
		d := api.New()
		fineChord(d)
		// The path, in a workplane through the z axis, turned by tilt
		// about it.
		tilt := p.Float("tilt") * math.Pi / 180
		dir := api.V(math.Cos(tilt), math.Sin(tilt), 0)
		d.Free3D()
		wp := d.NewWorkplane(api.V(0, 0, 0), api.V(0, 0, 1), dir)
		d.SketchIn(wp, "path")
		sgn := 1.0
		if up < 0 {
			sgn = -1
		}
		at := func(x, z float64) api.Vec { return dir.ScaledBy(x).Plus(api.V(0, 0, z)) }
		c := at(r, up)
		// The turn bends towards +dir, starting along z.
		end := c.Plus(at(-r*math.Cos(th), sgn*r*math.Sin(th)).Minus(api.V(0, 0, 0)))
		tangent := at(math.Sin(th), sgn*math.Cos(th))
		var arc api.Entity
		if up > 0 {
			arc = d.Arc(c, at(0, up), end)
		} else {
			arc = d.Arc(c, end, at(0, up))
		}
		path := []api.Entity{
			d.Line(at(0, 0), at(0, up)),
			arc,
			d.Line(end, end.Plus(tangent.ScaledBy(tail))),
		}
		d.SketchIn(d.Workplane("XY"), "profile")
		d.Circle(api.V(0, 0, 0), rho)
		s := d.Sweep(path, "")
		res := s.Check()
		want := math.Pi * rho * rho * (math.Abs(up) + r*th + tail)
		switch {
		case s.Problem() != "":
			return fmt.Errorf("problem: %s", s.Problem())
		case !res.Sound():
			return fmt.Errorf("unsound: %s", res.String())
		case math.Abs(res.Volume-want) > 5e-3*want:
			return fmt.Errorf("volume %g, want %g", res.Volume, want)
		}
		return nil
	}).Require(t)
}

// sweepGridMaxFailRate is the fraction of TestSweepGrid's grid allowed
// to fail: none, as the whole grid gave in the review of 2026-10-03.
// When the sweep was written, earlier that day, 7 of 72 failed, all
// with the thinnest tube (0.5 about bends of 5 and 10): the solid
// check found edges of the turn's triangulation, the grid
// triangulation of thin tori, crossing a neighbouring row by about
// 2e-4 mm.
const sweepGridMaxFailRate = 0
