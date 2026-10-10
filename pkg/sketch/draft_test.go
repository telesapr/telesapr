package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A block's extrusion drafted (roadmap 15.4): a frustum of its
// rectangle, the far end smaller by the height times the angle's
// tangent all round; its faces picked as an extrusion's; the entities
// of the far end hidden; a draft the sides can't take a problem, with
// the extrusion made without it; undo, redo and a round trip.
func TestDraft(t *testing.T) {
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	step(d)
	before := saved(t, d)
	ex.AsExtrude().SetDraft(5)
	if ex.AsExtrude().Draft() != 5 {
		t.Fatalf("draft %g", ex.AsExtrude().Draft())
	}
	x := 10 * math.Tan(5*math.Pi/180)
	area := func(z float64) float64 { s := z / 10 * x; return (20 - 2*s) * (10 - 2*s) }
	want := 10.0 / 6 * (area(0) + 4*area(5) + area(10))
	r := ex.Analyze().Check()
	if ex.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 1e-6*want {
		t.Fatalf("drafted block: %q, %s, want %g", ex.Problem(), r.String(), want)
	}
	if math.Abs(r.Max.X-20) > 1e-9 || math.Abs(r.Min.X) > 1e-9 {
		t.Errorf("the foot is the sketch: bounds %v to %v", r.Min, r.Max)
	}
	// The top, smaller, and a side, leaning, picked by points on them.
	top := ex.Solid().FaceAt(api.V(10, 5, 10))
	side := ex.Solid().FaceAt(api.V(10, x/2, 5))
	if top.Entity().H == side.Entity().H {
		t.Error("the top and the side are one face")
	}
	sk := d.Host().Sketch()
	for _, e := range ex.Entities() {
		if e.Kind() == "line" && e.Start().Pos().Z == 10 && !sk.Entity(e.H).ForceHidden {
			t.Errorf("%s at the far end is shown", e.Kind())
		}
	}
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	if g := back.Group(ex.Name()); g.AsExtrude().Draft() != 5 || math.Abs(g.Analyze().Check().Volume-want) > 1e-6*want {
		t.Errorf("after a round trip: draft %g, volume %g", g.AsExtrude().Draft(), g.Analyze().Check().Volume)
	}
	// An expression.
	if err := ex.SetExpr(sketch.GroupValC, "2+1"); err != nil || ex.AsExtrude().Draft() != 3 {
		t.Errorf("draft by expression: %g, %v", ex.AsExtrude().Draft(), err)
	}
	// Leaning out.
	ex.AsExtrude().SetDraft(-5)
	if r := ex.Analyze().Check(); ex.Problem() != "" || !r.Sound() || r.Max.X < 20+x-1e-9 {
		t.Errorf("leaning out: %q, %s", ex.Problem(), r.String())
	}
	// The sides meet: a problem, and the block without the draft.
	ex.AsExtrude().SetDraft(40)
	if r := ex.Analyze().Check(); !strings.Contains(ex.Problem(), "meet") || math.Abs(r.Volume-2000) > 1e-6 {
		t.Errorf("too steep: %q, volume %g", ex.Problem(), r.Volume)
	}
	ex.AsExtrude().SetDraft(0)
	if r := ex.Analyze().Check(); ex.Problem() != "" || math.Abs(r.Volume-2000) > 1e-6 {
		t.Errorf("no draft: %q, volume %g", ex.Problem(), r.Volume)
	}
}

// A round boss with a hole drafted: the outside a cone narrowing, the
// hole a cone widening; and a fillet on the drafted solid's top edge.
func TestDraftRound(t *testing.T) {
	d := api.New()
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 10)
	d.Sketch.Circle(api.V(0, 0, 0), 4)
	ex := d.Features.Extrude(10, "")
	ex.AsExtrude().SetDraft(3)
	x := 10 * math.Tan(3*math.Pi/180)
	cone := func(r0, r1 float64) float64 { return math.Pi * 10 / 3 * (r0*r0 + r0*r1 + r1*r1) }
	want := cone(10, 10-x) - cone(4, 4+x)
	if r := ex.Analyze().Check(); ex.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 0.01*want {
		t.Fatalf("drafted ring: %q, %s, want %g", ex.Problem(), r.String(), want)
	}
	// A sketch on the top face still finds its plane.
	if f := ex.Solid().FaceAt(api.V(7, 0, 10)); f.Entity().H.V == 0 {
		t.Error("the top has no entity")
	}
}

func TestDraftMisuse(t *testing.T) {
	d := api.New()
	d.Sketch.Line(api.V(0, 0, 0), api.V(10, 0, 0))
	d.Sketch.Rect(api.V(20, 0, 0), api.V(30, 10, 0))
	ex := d.Features.Extrude(5, "")
	for name, c := range map[string]struct {
		fn   func()
		want string
	}{
		"ninety":       {func() { ex.AsExtrude().SetDraft(90) }, "between -90 and 90"},
		"not extruded": {func() { d.Group("sketch-in-plane").AsExtrude().SetDraft(1) }, "extrude"},
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
}
