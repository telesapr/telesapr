package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A chamfer's edge with a distance of its own: at a block's top corner,
// the edge along x cut at the chamfer's 1, the one along y at its own
// 2. The two cuts meet in a mitre, which takes what both prisms take,
// 1*1/2*10 and 2*2/2*10, less what they share at the corner:
// ∫₀¹ (1-w)(2-w) dw = 5/6.
func TestChamferEdgeDistance(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 10)
	c := d.Features.Chamfer([]api.Entity{
		lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10)),
		lineAlong(t, ex, api.V(10, 0, 10), api.V(10, 10, 10)),
	}, 1, "")
	before := saved(t, d)
	step(d)
	c.AsChamfer().SetEdgeDistance(1, 2)
	undoRedo(t, d, before)
	want := 1000 - (5 + 20 - 5.0/6)
	if r := c.Analyze().Check(); c.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 1e-6 {
		t.Fatalf("%q %s, want volume %g", c.Problem(), r.String(), want)
	}
	if got := c.AsChamfer().EdgeDistance(1); got != 2 {
		t.Errorf("edge 1's distance %g", got)
	}
	if got := c.AsChamfer().EdgeDistance(0); got != 1 {
		t.Errorf("edge 0's distance %g, not the chamfer's", got)
	}

	// The same solid the other way round: the chamfer's distance 2 and
	// the first edge's own 1.
	other := api.New()
	ox := block(other, 0, 0, 10, 10, 10)
	oc := other.Features.Chamfer([]api.Entity{
		lineAlong(t, ox, api.V(0, 0, 10), api.V(10, 0, 10)),
		lineAlong(t, ox, api.V(10, 0, 10), api.V(10, 10, 10)),
	}, 2, "")
	oc.AsChamfer().SetEdgeDistance(0, 1)
	if r := oc.Analyze().Check(); oc.Problem() != "" || math.Abs(r.Volume-want) > 1e-6 {
		t.Fatalf("the other way round: %q %s", oc.Problem(), r.String())
	}

	// An edge's own distance stays with it in the file, and is taken
	// off with 0.
	back := roundTrip(t, d)
	if got := back.Group(c.Name()).AsChamfer().EdgeDistance(1); got != 2 {
		t.Errorf("reopened: edge 1's distance %g", got)
	}
	c.AsChamfer().SetEdgeDistance(1, 0)
	if r := c.Analyze().Check(); math.Abs(r.Volume-(1000-10+1.0/3)) > 1e-6 {
		t.Errorf("after the own distance is taken off: %s", r.String())
	}
}

// An edge's own second distance: the edge along y cut 1 on one face and
// 3 on the other, the edge along x at the chamfer's 1 on both.
func TestChamferEdgeSecondDistance(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 10)
	c := d.Features.Chamfer([]api.Entity{
		lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10)),
		lineAlong(t, ex, api.V(10, 0, 10), api.V(10, 10, 10)),
	}, 1, "")
	before := saved(t, d)
	step(d)
	c.AsChamfer().SetEdgeSecondDistance(1, 3)
	undoRedo(t, d, before)
	if got := c.AsChamfer().EdgeSecondDistance(1); got != 3 {
		t.Errorf("edge 1's second distance %g", got)
	}
	if got := c.AsChamfer().EdgeSecondDistance(0); got != 0 {
		t.Errorf("edge 0's second distance %g, not the chamfer's 0", got)
	}
	// Either way round, the prisms take 5 and 15; what they share is
	// ∫₀¹ (1-w)(3-3w) dw = 1 with the 3 down the side, or
	// ∫₀¹ (1-w)(1-w/3) dw = 7/18 with it across the top.
	r := c.Analyze().Check()
	down, across := 1000-(20-1.0), 1000-(20-7.0/18)
	if c.Problem() != "" || !r.Sound() || math.Abs(r.Volume-down) > 1e-6 && math.Abs(r.Volume-across) > 1e-6 {
		t.Fatalf("%q %s, want volume %g or %g", c.Problem(), r.String(), down, across)
	}

	// Swapping the chamfer's distances swaps the edge's own as well.
	c.AsChamfer().SwapDistances()
	r2 := c.Analyze().Check()
	if c.Problem() != "" || !r2.Sound() || math.Abs(r2.Volume-r.Volume) < 1e-3 {
		t.Fatalf("swapped: %q %s, before %g", c.Problem(), r2.String(), r.Volume)
	}
	if a, b := c.AsChamfer().EdgeDistance(1), c.AsChamfer().EdgeSecondDistance(1); a != 3 || b != 1 {
		t.Errorf("swapped: edge 1 at %g and %g", a, b)
	}
	if a := c.AsChamfer().EdgeDistance(0); a != 1 {
		t.Errorf("swapped: edge 0 at %g", a)
	}
	roundTrip(t, d)
}

// An edge's own distance follows the edge when the chamfer's edges
// change, and goes when its edge does.
func TestChamferEdgeDistanceFollowsEdges(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 10)
	a := lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10))
	b := lineAlong(t, ex, api.V(10, 0, 10), api.V(10, 10, 10))
	c := d.Features.Chamfer([]api.Entity{a, b}, 1, "")
	c.AsChamfer().SetEdgeDistance(1, 2)
	c.AsChamfer().SetEdgeSecondDistance(1, 1.5)
	c.AsChamfer().SetEdges([]api.Entity{b}, nil)
	if got := c.AsChamfer().EdgeDistance(0); got != 2 {
		t.Errorf("the kept edge's distance %g", got)
	}
	if got := c.AsChamfer().EdgeSecondDistance(0); got != 1.5 {
		t.Errorf("the kept edge's second distance %g", got)
	}
	c.AsChamfer().SetEdges([]api.Entity{a}, nil)
	if got := c.AsChamfer().EdgeDistance(0); got != 1 {
		t.Errorf("a new edge's distance %g, not the chamfer's", got)
	}
}

// Expressions, and what is refused.
func TestChamferEdgeDistanceRefused(t *testing.T) {
	t.Parallel()
	d := api.New()
	if err := d.Params.Set("w", "1.5"); err != nil {
		t.Fatal(err)
	}
	ex := block(d, 0, 0, 10, 10, 5)
	c := d.Features.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))}, 1, "")
	if err := c.AsChamfer().SetEdgeDistanceExpr(0, "w*2"); err != nil {
		t.Fatal(err)
	}
	if got := c.AsChamfer().EdgeDistance(0); got != 3 {
		t.Errorf("from w*2: %g", got)
	}
	if err := c.AsChamfer().SetEdgeSecondDistanceExpr(0, "w-2"); err == nil || !strings.Contains(err.Error(), "more than zero") {
		t.Errorf("w-2: %v", err)
	}
	if got := c.AsChamfer().EdgeSecondDistance(0); got != 0 {
		t.Errorf("a refused second distance stayed: %g", got)
	}
	for name, tc := range map[string]struct {
		fn   func()
		want string
	}{
		"negative":      {func() { c.AsChamfer().SetEdgeDistance(0, -1) }, "more than zero"},
		"no such edge":  {func() { c.AsChamfer().SetEdgeSecondDistance(3, 1) }, "no edge 4"},
		"read past":     {func() { c.AsChamfer().EdgeDistance(1) }, "no edge 1"},
		"not a chamfer": {func() { ex.AsChamfer().SetEdgeDistance(0, 1) }, "chamfer"},
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
	// A distance wider than a face is the group's problem, not a panic.
	c.AsChamfer().SetEdgeDistance(0, 50)
	if c.Problem() == "" {
		t.Error("a distance of 50 on a block 10 wide made no problem")
	}
}
