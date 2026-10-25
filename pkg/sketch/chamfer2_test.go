package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
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
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 5)
	edge := lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))
	c := d.Features.Chamfer([]api.Entity{edge}, 1, "")
	before := saved(t, d)
	step(d)
	c.AsChamfer().SetSecondDistance(2)
	if p := c.Problem(); p != "" {
		t.Fatal(p)
	}
	undoRedo(t, d, before)
	r := c.Analyze().Check()
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
	c.AsChamfer().SwapDistances()
	if c.AsChamfer().Distance() != 2 || c.AsChamfer().SecondDistance() != 1 {
		t.Fatalf("swapped: %g and %g", c.AsChamfer().Distance(), c.AsChamfer().SecondDistance())
	}
	if f, tp := setbacks(t, c, 5); math.Abs(f-wantTop) > 1e-9 || math.Abs(tp-wantFront) > 1e-9 {
		t.Fatalf("swapped: set back %g on the front and %g on the top", f, tp)
	}
	c.AsChamfer().SwapDistances()

	// In the file, in the document's unit, with an expression.
	if err := c.SetExpr(sketch.GroupValC, "1+1"); err != nil {
		t.Fatal(err)
	}
	d.Edit.Regen()
	back := roundTrip(t, d)
	bc := back.Group(c.Name())
	if bc.AsChamfer().SecondDistance() != 2 || bc.Expr(sketch.GroupValC) != "1+1" {
		t.Fatalf("reopened: %g, %q", bc.AsChamfer().SecondDistance(), bc.Expr(sketch.GroupValC))
	}

	// Back to one distance.
	c.AsChamfer().SetSecondDistance(0)
	if v := c.Analyze().Check().Volume; math.Abs(v-495) > 1e-6 {
		t.Fatalf("one distance again: %g", v)
	}

	// In inches: a quarter and a half of one, which read back exactly.
	d = api.New()
	ex = block(d, 0, 0, 30, 30, 30)
	c = d.Features.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 30), api.V(30, 0, 30))}, 6.35, "")
	c.AsChamfer().SetSecondDistance(12.7)
	d.View.SetUnits(core.UnitInches)
	back = roundTrip(t, d)
	if v := back.Group(c.Name()).AsChamfer().SecondDistance(); v != 12.7 {
		t.Fatalf("reopened in inches: %g mm", v)
	}
}

// Where chamfers of two distances meet at a corner, the local edit
// can't cut them; the wedges do.
func TestChamferTwoDistancesCorner(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 10)
	edges := []api.Entity{
		lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10)),
		lineAlong(t, ex, api.V(10, 0, 10), api.V(10, 10, 10)),
		lineAlong(t, ex, api.V(10, 0, 0), api.V(10, 0, 10)),
	}
	c := d.Features.Chamfer(edges, 1, "")
	c.AsChamfer().SetSecondDistance(2)
	r := c.Analyze().Check()
	// Three prisms of 1*10 each, less what they share at the corner.
	if c.Problem() != "" || !r.Sound() || r.Volume > 1000-30+4 || r.Volume < 1000-30 {
		t.Fatalf("chamfered corner: %q %s", c.Problem(), r.String())
	}
}

// A circular edge chamfered by two distances is cut by a cone of
// another angle: a triangle of legs a across and b along the axis
// turned about it, whose centroid is a third of a in from the rim.
func TestChamferTwoDistancesCircle(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Edit.SetChordTolerance(0.002)
	d.Sketch.Circle(api.V(0, 0, 0), 5)
	ex := d.Features.Extrude(10, "")
	var rim api.Entity
	for _, e := range ex.Entities() {
		if e.Kind() == "circle" && math.Abs(e.Center().Pos().Z-10) < 1e-9 {
			rim = e
		}
	}
	before := ex.Analyze().Check().Volume
	c := d.Features.Chamfer([]api.Entity{rim}, 1, "")
	c.AsChamfer().SetSecondDistance(2)
	r := c.Analyze().Check()
	took := before - r.Volume
	cut := func(a, b float64) float64 { return 2 * math.Pi * (5 - a/3) * a * b / 2 }
	if c.Problem() != "" || !r.Sound() ||
		math.Abs(took-cut(1, 2)) > 0.005*cut(1, 2) && math.Abs(took-cut(2, 1)) > 0.005*cut(2, 1) {
		t.Fatalf("%q, %s: took %g, want %g or %g", c.Problem(), r.String(), took, cut(1, 2), cut(2, 1))
	}
	c.AsChamfer().SwapDistances()
	took2 := before - c.Analyze().Check().Volume
	if math.Abs(took2-took) < 0.1 {
		t.Fatalf("swapping the distances took %g, as before", took2)
	}
}

func TestChamferTwoDistancesRefused(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 5)
	edge := lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))
	c := d.Features.Chamfer([]api.Entity{edge}, 1, "")
	for name, tc := range map[string]struct {
		fn   func()
		want string
	}{
		"negative":       {func() { c.AsChamfer().SetSecondDistance(-1) }, "more than zero"},
		"swap one":       {func() { c.AsChamfer().SwapDistances() }, "has one distance"},
		"not a chamfer":  {func() { ex.AsChamfer().SetSecondDistance(1) }, "chamfer"},
		"read extrusion": {func() { ex.AsChamfer().SecondDistance() }, "chamfer"},
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
	c.AsChamfer().SetSecondDistance(20)
	if p := c.Problem(); p == "" {
		t.Error("a second distance wider than the face made no problem")
	}
}

// The second distance, the swap and the same are the sketch's, which
// refuses a group that is not a chamfer and a swap of one distance.
func TestChamferDistancesOnSketch(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 5)
	sk := d.Host().Sketch()
	if err := sk.SetChamferSecondDistance(ex.H, 2); err == nil || !strings.Contains(err.Error(), "not a chamfer") {
		t.Errorf("an extrusion's second distance: %v", err)
	}
	if err := sk.SwapChamferDistances(ex.H); err == nil {
		t.Error("swapped an extrusion's distances")
	}
	c := d.Features.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))}, 1, "")
	if err := sk.SwapChamferDistances(c.H); err == nil || !strings.Contains(err.Error(), "one distance") {
		t.Errorf("swap of one distance: %v", err)
	}
	if err := sk.SetChamferSecondDistance(c.H, -1); err == nil {
		t.Error("a negative second distance")
	}
	if err := sk.SetChamferSecondDistance(c.H, 3); err != nil {
		t.Fatal(err)
	}
	if err := sk.SwapChamferDistances(c.H); err != nil {
		t.Fatal(err)
	}
	if g := sk.Group(c.H); g.ValB != 3 || g.ValC != 1 {
		t.Errorf("after the swap %g, %g", g.ValB, g.ValC)
	}
	if err := sk.SameChamferDistances(c.H); err != nil {
		t.Fatal(err)
	}
	if g := sk.Group(c.H); g.ValC != 0 || g.ExprC != "" {
		t.Errorf("after same %g %q", g.ValC, g.ExprC)
	}
}
