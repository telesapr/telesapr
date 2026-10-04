package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A loft of two squares set square to both (roadmap 28.3): one smooth
// surface leaving the bottom and reaching the top along their normal;
// the top moved aside, its horizontal sections are the square moved,
// so the volume is the area times the height; undo, redo, round trip,
// expressions; free again.
func TestLoftTangency(t *testing.T) {
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	bottom := d.SketchIn(d.Workplane("XY"), "bottom")
	d.Rect(api.V(-3, -3, 0), api.V(3, 3, 0))
	top := d.SketchAt(above(d, 12), api.V(1, 0, 0), api.V(0, 1, 0), "top")
	d.Rect(api.V(2, -3, 12), api.V(8, 3, 12))
	l := d.Loft(bottom, top, "")
	ruled := l.Check().Volume
	step(d)
	before := saved(t, d)
	l.SetTangency(1, 1)
	if s, e := l.Tangency(); s != 1 || e != 1 {
		t.Fatalf("tangency %g, %g", s, e)
	}
	r := l.Check()
	if want := 36.0 * 12; l.Problem() != "" || !r.Sound() || math.Abs(r.Volume-want) > 0.002*want ||
		math.Abs(ruled-want) > 1e-6*want {
		t.Fatalf("square ends: %q, %s, want %g (ruled %g)", l.Problem(), r.String(), want, ruled)
	}
	undoRedo(t, d, before)
	if err := l.SetExpr(sketch.GroupValC, "1/2"); err != nil {
		t.Fatal(err)
	}
	// A file holds curve points at the chord tolerance, which it doesn't
	// save.
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	d.Regen()
	back := roundTrip(t, d)
	if s, e := back.Group(l.Name()).Tangency(); s != 1 || e != 0.5 || back.Group(l.Name()).Expr(sketch.GroupValC) != "1/2" {
		t.Errorf("after a round trip: %g, %g", s, e)
	}
	// In inches, a tangency is no length.
	d.SetUnits(core.UnitInches)
	back = roundTrip(t, d)
	if s, _ := back.Group(l.Name()).Tangency(); s != 1 {
		t.Errorf("in another unit the start's tangency became %g", s)
	}
	l.SetTangency(0, 0)
	if v := l.Check().Volume; math.Abs(v-ruled) > 1e-9*ruled {
		t.Errorf("free again: %g, want %g", v, ruled)
	}
	func() {
		defer func() {
			err, ok := recover().(error)
			if !ok || !strings.Contains(err.Error(), "loft") {
				t.Errorf("tangency of a sketch: %v", err)
			}
		}()
		bottom.SetTangency(1, 1)
	}()
}
