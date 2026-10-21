package sketch_test

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// dragging is the Hooks of a sketch whose user drags params.
type dragging struct {
	sketch.NoHooks
	params []core.HParam
}

func (h dragging) DraggedParams() []core.HParam { return h.params }

// A held depth takes the extrusion's freedom along its normal, follows a
// parameter, survives a drag of its end, comes back on undo and redo and
// in the file, and frees again.
func TestExtrudeDepthHeld(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	if ex.Dof() != 1 || ex.AsExtrude().DepthHeld() {
		t.Fatalf("a free extrusion: dof %d, held %v", ex.Dof(), ex.AsExtrude().DepthHeld())
	}
	if got := ex.AsExtrude().Depth(); math.Abs(got-10) > 1e-9 {
		t.Fatalf("depth %g, want 10", got)
	}
	step(d)
	before := saved(t, d)
	if err := d.Params.Set("h", "15"); err != nil {
		t.Fatal(err)
	}
	if err := ex.SetExpr(sketch.GroupExtent, "h*2"); err != nil {
		t.Fatal(err)
	}
	if ex.Dof() != 0 || !ex.AsExtrude().DepthHeld() {
		t.Fatalf("a held extrusion: dof %d", ex.Dof())
	}
	if v := ex.Analyze().Check().Volume; math.Abs(v-20*10*30) > 1e-6 {
		t.Fatalf("volume %g, want %g", v, 20.0*10*30)
	}
	undoRedo(t, d, before)
	back := roundTrip(t, d)
	if g := back.Group(ex.Name()); g.Expr(sketch.GroupExtent) != "h*2" || math.Abs(g.AsExtrude().Depth()-30) > 1e-9 {
		t.Fatalf("after a round trip: %q, depth %g", g.Expr(sketch.GroupExtent), g.AsExtrude().Depth())
	}
	if f := saved(t, d); !strings.Contains(string(f), "depth = h*2") {
		t.Fatalf("the file has no depth:\n%s", f)
	}

	// A drag of the extrusion's end doesn't move it.
	d.With(func(sk *sketch.Sketch) {
		hg := ex.H
		for i := range 3 {
			sk.MustParam(hg.Param(i)).Val *= 3
		}
		sk.Hooks = dragging{params: []core.HParam{hg.Param(0), hg.Param(1), hg.Param(2)}}
		sk.MarkGroupDirty(hg, false)
		sk.GenerateAll()
		sk.Hooks = nil
	})
	if got := ex.AsExtrude().Depth(); math.Abs(got-30) > 1e-9 {
		t.Fatalf("dragged to %g, want 30", got)
	}

	if err := d.Params.Set("h", "-4"); err != nil {
		t.Fatal(err)
	}
	if got := ex.AsExtrude().Depth(); math.Abs(got+8) > 1e-9 {
		t.Fatalf("depth %g, want -8", got)
	}
	ex.AsExtrude().FreeDepth()
	if ex.Dof() != 1 || ex.AsExtrude().DepthHeld() || ex.Expr(sketch.GroupExtent) != "" {
		t.Fatalf("freed: dof %d, expr %q", ex.Dof(), ex.Expr(sketch.GroupExtent))
	}
	if got := ex.AsExtrude().Depth(); math.Abs(got+8) > 1e-9 {
		t.Fatalf("freed at %g, want -8", got)
	}
	ex.AsExtrude().SetDepth(12)
	if got := ex.AsExtrude().Depth(); math.Abs(got-12) > 1e-9 || ex.Dof() != 0 {
		t.Fatalf("depth %g dof %d, want 12 and 0", got, ex.Dof())
	}
}

// A two-sided extrusion's depth is from end to end, and an inch
// document's depth expression is in inches.
func TestExtrudeDepthTwoSidedInches(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	d.With(func(sk *sketch.Sketch) {
		sk.Group(ex.H).Subtype = sketch.GroupTwoSided
		sk.MarkGroupDirty(ex.H, false)
	})
	d.View.SetUnits(core.UnitInches)
	if err := ex.SetExpr(sketch.GroupExtent, "1/2"); err != nil {
		t.Fatal(err)
	}
	r := ex.Analyze().Check()
	if math.Abs(r.Volume-200*12.7) > 1e-6 || math.Abs(r.Max.Z-6.35) > 1e-9 || math.Abs(r.Min.Z+6.35) > 1e-9 {
		t.Fatalf("two-sided half inch: %s, %v to %v", r.String(), r.Min, r.Max)
	}
	roundTrip(t, d)
}

// A held angle takes a revolve's freedom and follows a parameter; the
// angles a revolve can't have are refused, and so is a depth on a group
// without one.
func TestRevolveAngleHeld(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(5, 0, 0), api.V(10, 10, 0))
	axis := d.Sketch.Line(api.V(0, 0, 0), api.V(0, 10, 0))
	axis.SetConstruction(true)
	rv := d.Features.Revolve(axis, 90, "")
	if rv.Dof() != 1 {
		t.Fatalf("a free revolve: dof %d", rv.Dof())
	}
	if got := rv.AsRevolve().Angle(); math.Abs(got-90) > 1e-9 {
		t.Fatalf("angle %g, want 90", got)
	}
	if err := d.Params.Set("a", "180"); err != nil {
		t.Fatal(err)
	}
	if err := rv.SetExpr(sketch.GroupExtent, "a"); err != nil {
		t.Fatal(err)
	}
	want := math.Pi * (100 - 25) * 10 / 2
	if v := rv.Analyze().Check().Volume; rv.Dof() != 0 || math.Abs(v-want) > 1e-3*want {
		t.Fatalf("half a turn: dof %d volume %g, want %g", rv.Dof(), v, want)
	}
	for _, text := range []string{"0", "a*3", "a-a"} {
		if err := rv.SetExpr(sketch.GroupExtent, text); err == nil {
			t.Errorf("angle %q accepted", text)
		}
	}
	if !rv.AsRevolve().AngleHeld() || rv.Expr(sketch.GroupExtent) != "a" {
		t.Fatalf("a refused angle changed the held one: %q", rv.Expr(sketch.GroupExtent))
	}
	rv.AsRevolve().FreeAngle()
	if rv.Dof() != 1 || rv.AsRevolve().AngleHeld() {
		t.Fatal("not freed")
	}
	rv.AsRevolve().SetAngle(45)
	if got := rv.AsRevolve().Angle(); math.Abs(got-45) > 1e-9 {
		t.Fatalf("angle %g, want 45", got)
	}
	roundTrip(t, d)

	var err error
	d.With(func(sk *sketch.Sketch) { err = sk.SetGroupText(sk.Group(rv.H).OpA, sketch.GroupExtent, "3") })
	if err == nil || !strings.Contains(err.Error(), "only an extrusion") {
		t.Errorf("a depth on a sketch: %v", err)
	}
}
