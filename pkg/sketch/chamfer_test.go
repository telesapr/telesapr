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

// lineAlong is the line of group g from a to b, either way round.
func lineAlong(t *testing.T, g api.Group, a, b api.Vec) api.Entity {
	t.Helper()
	near := func(p, q api.Vec) bool { return p.Minus(q).Magnitude() < 1e-6 }
	for _, e := range g.Entities() {
		if e.Kind() != "line" {
			continue
		}
		s, f := e.Start().Pos(), e.End().Pos()
		if (near(s, a) && near(f, b)) || (near(s, b) && near(f, a)) {
			return e
		}
	}
	t.Fatalf("%v has no line from %v to %v", g, a, b)
	return api.Entity{}
}

func TestChamferOneEdge(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 5)
	top := lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))
	step(d)
	before := saved(t, d)
	c := d.Features.Chamfer([]api.Entity{top}, 1, "")
	if p := c.Problem(); p != "" {
		t.Fatal(p)
	}
	r := c.Analyze().Check()
	if !r.Sound() || math.Abs(r.Volume-495) > 1e-6 || trimmedFaces(c) != 7 {
		t.Fatalf("chamfered block: %s", r.String())
	}
	undoRedo(t, d, before)
	// The distance can change, also as an expression in the unit.
	c.AsChamfer().SetDistance(2)
	if v := c.Analyze().Check().Volume; math.Abs(v-480) > 1e-6 {
		t.Fatalf("at distance 2 the volume is %g, want 480", v)
	}
	if err := c.SetExpr(sketch.GroupPitch, "1+0.5"); err != nil {
		t.Fatal(err)
	}
	d.Edit.Regen()
	if v := c.Analyze().Check().Volume; math.Abs(v-500+0.5*1.5*1.5*10) > 1e-6 || c.AsChamfer().Distance() != 1.5 {
		t.Fatalf("at distance %g the volume is %g", c.AsChamfer().Distance(), v)
	}
	if err := c.SetExpr(sketch.GroupPitch, "0"); err == nil {
		t.Fatal("a distance of 0 was taken")
	}
	back := roundTrip(t, d)
	bc := back.Group(c.Name())
	if bc.Kind() != "chamfer" || len(bc.Edges()) != 1 || bc.Expr(sketch.GroupPitch) != "1+0.5" {
		t.Fatalf("reopened: %s, %d edges, %q", bc.Kind(), len(bc.Edges()), bc.Expr(sketch.GroupPitch))
	}
}

// A corner where three chamfered edges meet.
func TestChamferCorner(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 10, 10, 10)
	edges := []api.Entity{
		lineAlong(t, ex, api.V(0, 0, 10), api.V(10, 0, 10)),
		lineAlong(t, ex, api.V(10, 0, 10), api.V(10, 10, 10)),
		lineAlong(t, ex, api.V(10, 0, 0), api.V(10, 0, 10)),
	}
	c := d.Features.Chamfer(edges, 1, "")
	r := c.Analyze().Check()
	// Three prisms of 0.5*10 each, less the overlaps at the corner.
	if !r.Sound() || r.Volume > 1000-15+1 || r.Volume < 1000-15 {
		t.Fatalf("chamfered corner: %s", r.String())
	}
}

func TestChamferRefused(t *testing.T) {
	d := api.New()
	// An L: the inner edge is concave.
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(20, 0, 0), api.V(20, 5, 0), api.V(5, 5, 0), api.V(5, 20, 0), api.V(0, 20, 0))
	ex := d.Features.Extrude(5, "")
	inner := lineAlong(t, ex, api.V(5, 5, 0), api.V(5, 5, 5))
	sketchLine := d.Groups()[1].Entities()[0]
	for name, c := range map[string]struct {
		fn   func()
		want string
	}{
		"no edges":      {func() { d.Features.Chamfer(nil, 1, "") }, "no edges"},
		"zero distance": {func() { d.Features.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(20, 0, 5))}, 0, "") }, "more than zero"},
		"a point":       {func() { d.Features.Chamfer([]api.Entity{inner.Start()}, 1, "") }, "neither a line segment nor a circle"},
		"twice": {func() {
			e := lineAlong(t, ex, api.V(0, 0, 5), api.V(20, 0, 5))
			d.Features.Chamfer([]api.Entity{e, e}, 1, "")
		}, "twice"},
		"distance of extrude": {func() { ex.AsChamfer().Distance() }, "chamfer"},
	} {
		func() {
			defer func() {
				err, ok := recover().(error)
				if !ok || !strings.Contains(err.Error(), c.want) {
					t.Errorf("%s: %v, want an error saying %q", name, err, c.want)
				}
			}()
			c.fn()
		}()
	}
	_ = sketchLine
}

// The inner edge of an L, which goes in, is chamfered by the fillet's
// edit, which fills it: the solid gains a prism of half the distance
// squared along the edge (roadmap 15.1).
func TestChamferConcave(t *testing.T) {
	d := api.New()
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(20, 0, 0), api.V(20, 5, 0), api.V(5, 5, 0), api.V(5, 20, 0), api.V(0, 20, 0))
	ex := d.Features.Extrude(5, "")
	before := ex.Analyze().Check().Volume
	inner := lineAlong(t, ex, api.V(5, 5, 0), api.V(5, 5, 5))
	c := d.Features.Chamfer([]api.Entity{inner}, 1, "")
	r := c.Analyze().Check()
	if c.Problem() != "" || !r.Sound() || math.Abs(r.Volume-before-0.5*5) > 1e-6 {
		t.Errorf("the concave chamfer: %q, %s, gained %g, want 2.5", c.Problem(), r.String(), r.Volume-before)
	}
}

// An edge that stops being one, as when the solid under it is made
// smaller, gives a problem on the group, and no solid of its own.
func TestChamferProblem(t *testing.T) {
	d := api.New()
	sides := d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	width := d.Constrain.Length(sides[1], 10)
	ex := d.Features.Extrude(5, "")
	c := d.Features.Chamfer([]api.Entity{lineAlong(t, ex, api.V(0, 0, 5), api.V(10, 0, 5))}, 1, "")
	width.Set(0.5)
	if p := c.Problem(); !strings.Contains(p, "wider than a face") {
		t.Fatalf("problem %q", p)
	}
	if v := c.Analyze().Check().Volume; math.Abs(v-0.5*10*5) > 1e-6 {
		t.Fatalf("with a problem the chamfer cut %g away", 25-v)
	}
	// A chamfer of nothing.
	d.Activate(ex)
	ex.SetCombine(api.Difference)
	d.Activate(c)
	if p := c.Problem(); !strings.Contains(p, "no solid") {
		t.Fatalf("problem %q", p)
	}
}

// trimmedFaces counts the surfaces of a group's solid that a trim
// bounds: a Boolean keeps the surfaces of the solid taken away that it
// cut off whole, with no trims.
func trimmedFaces(g api.Group) int {
	n := 0
	for _, s := range g.Analyze().Shell().Surface {
		if len(s.Trim) > 0 {
			n++
		}
	}
	return n
}

// TestChamferSweep chamfers the edge at a corner of angle angle of a
// triangular prism of length length, at distance d, and requires a
// sound solid of the closed-form volume. The rate allowed was fixed at
// none before the chamfer was written.
func TestChamferSweep(t *testing.T) {
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("angle", 20, 45, 60, 90, 120, 150, 165),
			sweep.Floats("d", 0.1, 1, 2.5, 4),
			sweep.Floats("length", 0.5, 5, 40),
		},
		MaxFailRate: 0,
		Sample:      20,
	}
	grid.Run(func(p sweep.Point) error {
		th := p.Float("angle") * math.Pi / 180
		dist, length := p.Float("d"), p.Float("length")
		d := api.New()
		d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(10, 0, 0), api.V(10*math.Cos(th), 10*math.Sin(th), 0))
		ex := d.Features.Extrude(length, "")
		edge := lineAlong(t, ex, api.V(0, 0, 0), api.V(0, 0, length))
		c := d.Features.Chamfer([]api.Entity{edge}, dist, "")
		r := c.Analyze().Check()
		want := (50*math.Sin(th) - 0.5*dist*dist*math.Sin(th)) * length
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

// A chamfer of one side of a rounded rectangle's top goes on round the
// rounded corners, as a fillet does, by cones there: all the way round
// the top (roadmap 15.1).
func TestChamferRoundedOutline(t *testing.T) {
	d := api.New()
	d.Edit.SetChordTolerance(0.005)
	sides := d.Sketch.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	for _, s := range sides {
		d.Sketch.TangentArc(s.Start(), 4, true)
	}
	ex := d.Features.Extrude(10, "")
	before := ex.Analyze().Check().Volume
	c := d.Features.Chamfer([]api.Entity{lineAlong(t, ex, api.V(4, 0, 10), api.V(26, 0, 10))}, 1, "")
	r := c.Analyze().Check()
	// Half the distance squared along the straight runs, and round each
	// corner the cone's ring: the triangle turned about the corner's
	// axis, its centroid a third of the distance in from the rim.
	straight := 0.5 * (2*22 + 2*12)
	corners := 0.5 * 2 * math.Pi * (4 - 1.0/3)
	want := straight + corners
	if c.Problem() != "" || !r.Sound() || math.Abs(before-r.Volume-want) > 0.01*want {
		t.Errorf("%q: %s, took %g, want %g", c.Problem(), r.String(), before-r.Volume, want)
	}
}
