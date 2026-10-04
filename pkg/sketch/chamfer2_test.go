package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// setbacks are how far a chamfer of a block's edge along x at y = 0,
// z = h cuts back on the front face (y = 0) and on the top (z = h):
// read from the solid's vertices at x = 0.
func setbacks(t *testing.T, g api.Group, h float64) (front, top float64) {
	t.Helper()
	front, top = -1, -1
	for _, v := range g.Solid().Vertices() {
		p := v.Pos()
		if math.Abs(p.X) > 1e-9 {
			continue
		}
		switch {
		case math.Abs(p.Y) < 1e-9 && p.Z < h-1e-9 && p.Z > 1e-9:
			front = h - p.Z
		case math.Abs(p.Z-h) < 1e-9 && p.Y > 1e-9 && p.Y < 5:
			top = p.Y
		}
	}
	if front < 0 || top < 0 {
		t.Fatalf("no chamfer at the block's end: front %g, top %g", front, top)
	}
	return front, top
}

// A chamfer of two distances (roadmap 28.1): the first on the edge's
// face of the lower identity, the second on the other; swapping them
// turns it round.
func TestChamferTwoDistances(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 5)
	edge := lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))
	c := d.Chamfer([]api.Entity{edge}, 1, "")
	before := saved(t, d)
	step(d)
	c.SetSecondDistance(2)
	if p := c.Problem(); p != "" {
		t.Fatal(p)
	}
	undoRedo(t, d, before)
	r := c.Check()
	if !r.Sound() || math.Abs(r.Volume-(500-0.5*1*2*10)) > 1e-6 {
		t.Fatalf("chamfer of 1 and 2: %s", r.String())
	}
	frontID := c.Solid().FaceAt(api.V(5, 0, 1)).ID()
	topID := c.Solid().FaceAt(api.V(5, 8, 5)).ID()
	front, top := setbacks(t, c, 5)
	wantFront, wantTop := 1.0, 2.0
	if topID < frontID {
		wantFront, wantTop = 2, 1
	}
	if math.Abs(front-wantFront) > 1e-9 || math.Abs(top-wantTop) > 1e-9 {
		t.Fatalf("set back %g on the front and %g on the top, want %g and %g", front, top, wantFront, wantTop)
	}
	c.SwapDistances()
	if c.Distance() != 2 || c.SecondDistance() != 1 {
		t.Fatalf("swapped: %g and %g", c.Distance(), c.SecondDistance())
	}
	if f, tp := setbacks(t, c, 5); math.Abs(f-wantTop) > 1e-9 || math.Abs(tp-wantFront) > 1e-9 {
		t.Fatalf("swapped: set back %g on the front and %g on the top", f, tp)
	}
	c.SwapDistances()

	// In the file, in the document's unit, with an expression.
	if err := c.SetExpr(sketch.GroupValC, "1+1"); err != nil {
		t.Fatal(err)
	}
	d.Regen()
	back := roundTrip(t, d)
	bc := back.Group(c.Name())
	if bc.SecondDistance() != 2 || bc.Expr(sketch.GroupValC) != "1+1" {
		t.Fatalf("reopened: %g, %q", bc.SecondDistance(), bc.Expr(sketch.GroupValC))
	}

	// Back to one distance.
	c.SetSecondDistance(0)
	if v := c.Check().Volume; math.Abs(v-495) > 1e-6 {
		t.Fatalf("one distance again: %g", v)
	}

	// In inches: a quarter and a half of one, which read back exactly.
	d = api.New()
	ex = block(d, 0, 0, 30, 30, 30)
	c = d.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 30), api.V(30, 0, 30))}, 6.35, "")
	c.SetSecondDistance(12.7)
	d.SetUnits(core.UnitInches)
	back = roundTrip(t, d)
	if v := back.Group(c.Name()).SecondDistance(); v != 12.7 {
		t.Fatalf("reopened in inches: %g mm", v)
	}
}

// Where chamfers of two distances meet at a corner, the local edit
// can't cut them; the wedges do.
func TestChamferTwoDistancesCorner(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 10)
	edges := []api.Entity{
		lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10)),
		lineAlong(t, ex, api.V(10, 0, 10), api.V(10, 10, 10)),
		lineAlong(t, ex, api.V(10, 0, 0), api.V(10, 0, 10)),
	}
	c := d.Chamfer(edges, 1, "")
	c.SetSecondDistance(2)
	r := c.Check()
	// Three prisms of 1*10 each, less what they share at the corner.
	if c.Problem() != "" || !r.Sound() || r.Volume > 1000-30+4 || r.Volume < 1000-30 {
		t.Fatalf("chamfered corner: %q %s", c.Problem(), r.String())
	}
}

// A circular edge chamfered by two distances is cut by a cone of
// another angle: a triangle of legs a across and b along the axis
// turned about it, whose centroid is a third of a in from the rim.
func TestChamferTwoDistancesCircle(t *testing.T) {
	d := api.New()
	d.SetChordTolerance(0.002)
	d.Circle(api.V(0, 0, 0), 5)
	ex := d.Extrude(10, "")
	var rim api.Entity
	for _, e := range ex.Entities() {
		if e.Kind() == "circle" && math.Abs(e.Center().Pos().Z-10) < 1e-9 {
			rim = e
		}
	}
	before := ex.Check().Volume
	c := d.Chamfer([]api.Entity{rim}, 1, "")
	c.SetSecondDistance(2)
	r := c.Check()
	took := before - r.Volume
	cut := func(a, b float64) float64 { return 2 * math.Pi * (5 - a/3) * a * b / 2 }
	if c.Problem() != "" || !r.Sound() ||
		math.Abs(took-cut(1, 2)) > 0.005*cut(1, 2) && math.Abs(took-cut(2, 1)) > 0.005*cut(2, 1) {
		t.Fatalf("%q, %s: took %g, want %g or %g", c.Problem(), r.String(), took, cut(1, 2), cut(2, 1))
	}
	c.SwapDistances()
	took2 := before - c.Check().Volume
	if math.Abs(took2-took) < 0.1 {
		t.Fatalf("swapping the distances took %g, as before", took2)
	}
}

func TestChamferTwoDistancesRefused(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 5)
	edge := lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))
	c := d.Chamfer([]api.Entity{edge}, 1, "")
	for name, tc := range map[string]struct {
		fn   func()
		want string
	}{
		"negative":       {func() { c.SetSecondDistance(-1) }, "more than zero"},
		"swap one":       {func() { c.SwapDistances() }, "has one distance"},
		"not a chamfer":  {func() { ex.SetSecondDistance(1) }, "chamfer"},
		"read extrusion": {func() { ex.SecondDistance() }, "chamfer"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, tc.want)
				}
			}()
			tc.fn()
		}()
	}
	if err := c.SetExpr(sketch.GroupValC, "-1"); err == nil {
		t.Error("a second distance of -1 was taken")
	}
	// Wider than the face.
	c.SetSecondDistance(20)
	if p := c.Problem(); p == "" {
		t.Error("a second distance wider than the face made no problem")
	}
}

// TestChamferTwoDistancesSweep chamfers the edge at a corner of angle
// angle of a triangular prism by d1 and d2 and requires a sound solid
// of the closed-form volume, whichever face takes d1. The rate allowed
// was fixed at none before the feature was written.
func TestChamferTwoDistancesSweep(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("angle", 30, 60, 90, 120, 150),
			sweep.Floats("d1", 0.2, 1, 3),
			sweep.Floats("d2", 0.5, 2, 4),
		},
		MaxFailRate: 0,
		Sample:      20,
	}
	grid.Run(func(p sweep.Point) error {
		th := p.Float("angle") * math.Pi / 180
		d1, d2 := p.Float("d1"), p.Float("d2")
		d := api.New()
		d.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(10*math.Cos(th), 10*math.Sin(th), 0))
		ex := d.Extrude(5, "")
		edge := lineAlong(t, ex, api.V(0, 0, 0), api.V(0, 0, 5))
		c := d.Chamfer([]api.Entity{edge}, d1, "")
		c.SetSecondDistance(d2)
		r := c.Check()
		want := (50*math.Sin(th) - 0.5*d1*d2*math.Sin(th)) * 5
		switch {
		case c.Problem() != "":
			return fmt.Errorf("problem: %s", c.Problem())
		case !r.Sound():
			return fmt.Errorf("unsound: %s", r.String())
		case math.Abs(r.Volume-want) > 1e-6*want:
			return fmt.Errorf("volume %g, want %g", r.Volume, want)
		}
		return nil
	}).Require(t)
}
