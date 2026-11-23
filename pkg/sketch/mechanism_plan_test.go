package sketch_test

import (
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// noPlan has d's mechanisms solved without their plans, from where the
// parts are, as before the plans, or with them again.
func noPlan(d *api.Doc, off bool) {
	d.With(func(sk *sketch.Sketch) { sk.NoMechanismPlan = off })
}

// planCounts are d's counts of plans compiled, of solves the plan
// placed the parts for, and of those it stood for.
func planCounts(d *api.Doc) (compiles, placed, solved int) {
	d.With(func(sk *sketch.Sketch) { compiles, placed, solved = sk.PlanCompiles, sk.PlanPlaced, sk.PlanSolved })
	return compiles, placed, solved
}

// placements are the move and turn of every part of d that moves in a
// mechanism, its seven params, in group order.
func placements(d *api.Doc) [][7]float64 {
	var out [][7]float64
	d.With(func(sk *sketch.Sketch) {
		for _, hg := range sk.GroupOrder {
			if g := sk.Group(hg); g.Type != sketch.GroupLinked || !g.LinkMoves {
				continue
			}
			var p [7]float64
			for i := range 7 {
				p[i] = sk.MustParam(hg.Param(i)).Val
			}
			out = append(out, p)
		}
	})
	return out
}

// solveAt sets d's parameter param to v and regenerates, from the
// params start, with the mechanism's plan or without, and returns where
// the parts are then.
func solveAt(t *testing.T, d *api.Doc, start []core.Param, param string, v float64, off bool) [][7]float64 {
	t.Helper()
	noPlan(d, off)
	d.With(func(sk *sketch.Sketch) {
		for _, p := range start {
			if q := sk.Param(p.H); q != nil {
				q.Val = p.Val
			}
		}
		for _, g := range sk.Groups {
			if g.LinkMoves {
				sk.MarkGroupDirty(g.H, false)
			}
		}
	})
	setParam(t, d, param, v)
	d.Edit.Regen()
	return placements(d)
}

// planFixture is a mechanism the plan is checked on, driven by param
// through values; free when the drive leaves it a freedom, so that the
// solve still runs after the plan.
type planFixture struct {
	name   string
	free   bool
	make   func(t *testing.T) *api.Doc
	param  string
	values []float64
}

// planFixtures are the mechanisms of the tests and the examples, each
// driven: a four-bar by its crank, a driven hinge of lines and one of
// pins, a driven concentric (its plate free to turn), a screw, the
// engine and the guide's lid.
func planFixtures() []planFixture {
	return []planFixture{
		{"four-bar", false, func(t *testing.T) *api.Doc { d, _, _, _, _, _ := fourBarDriven(t); return d },
			"angle", []float64{15, 40, 90, 200, 330}},
		{"driven hinge", false, func(t *testing.T) *api.Doc {
			d, ground, bar := twoBars(t)
			d.Sketch.Anywhere3D()
			d.Assembly.HingeDriven(uprights(t, ground)[60], uprights(t, bar)[0])
			return d
		}, "angle", []float64{30, 90, -45, 400}},
		{"driven pins", false, func(t *testing.T) *api.Doc {
			all := func(api.Entity) bool { return true }
			d, a, b := matePart(t)
			b.AsLink().SetMoves(true)
			d.Sketch.Anywhere3D()
			d.Assembly.HingeDriven(first(t, a, "circle", all), first(t, b, "circle", all))
			_, topA := ends(a)
			botB, _ := ends(b)
			d.Assembly.Flush(topA, botB)
			return d
		}, "angle", []float64{20, 135, -90}},
		{"driven concentric", true, func(t *testing.T) *api.Doc {
			all := func(api.Entity) bool { return true }
			d, a, b := matePart(t)
			b.AsLink().SetMoves(true)
			d.Sketch.Anywhere3D()
			d.Assembly.ConcentricDriven(first(t, a, "circle", all), first(t, b, "circle", all))
			return d
		}, "travel", []float64{2.5, 7.5, -3}},
		{"screw", false, func(t *testing.T) *api.Doc {
			all := func(api.Entity) bool { return true }
			d, a, b := matePart(t)
			b.AsLink().SetMoves(true)
			d.Sketch.Anywhere3D()
			d.Assembly.Screw(first(t, a, "circle", all), first(t, b, "circle", all), 2)
			return d
		}, "turn", []float64{45, 180, 720}},
		{"engine", false, func(t *testing.T) *api.Doc { d, _, _, _, _ := engine(t); return d },
			"angle", []float64{10, 45, 120, 250, 359}},
		{"lid", false, func(t *testing.T) *api.Doc {
			d, err := api.Open(filepath.Join("..", "..", "testdata", "ui", "mechanism", "boxlid.tlspr"))
			if err != nil {
				t.Fatal(err)
			}
			return d
		}, "angle", []float64{20, 60, 105}},
	}
}

// The plan places every driven mechanism where the solve alone puts it,
// from the same start, to 1e-6, and stands for the solve: the parts of
// a four-bar, of driven hinges and slides, a screw, the engine and the
// lid, each at several values of its drive.
func TestPlanAgreesWithSolveBlock(t *testing.T) {
	t.Parallel()
	for _, fx := range planFixtures() {
		t.Run(fx.name, func(t *testing.T) {
			t.Parallel()
			d := fx.make(t)
			_, _, solved0 := planCounts(d)
			for _, v := range fx.values {
				var start []core.Param
				d.With(func(sk *sketch.Sketch) { start = slices.Clone(sk.Params) })
				alone := solveAt(t, d, start, fx.param, v, true)
				for _, g := range d.Groups() {
					if !g.Solved() {
						t.Fatalf("%s = %g: %s not solved by the solve alone", fx.param, v, g.Name())
					}
				}
				planned := solveAt(t, d, start, fx.param, v, false)
				for i := range alone {
					for k := range 7 {
						if off := math.Abs(alone[i][k] - planned[i][k]); off > 1e-6 {
							t.Errorf("%s = %g: part %d param %d %.9g by the solve, %.9g by the plan", fx.param, v, i, k, alone[i][k], planned[i][k])
						}
					}
				}
				for _, g := range d.Groups() {
					if !g.Solved() {
						t.Errorf("%s = %g: %s not solved by the plan", fx.param, v, g.Name())
					}
				}
			}
			if _, _, solved := planCounts(d); !fx.free && solved <= solved0 {
				t.Errorf("the plan stood for no solve (%d before, %d after)", solved0, solved)
			}
		})
	}
}

// fourBarAt is fourBar with its bars put as start says, before they
// are mated: "stretch", in a line from the ground's first pin, each
// bar's origin at the one before's far end; "link", each where its link
// put it, on the ground; "origin", each at the origin. With off, the
// mechanism is solved without its plan.
func fourBarAt(t *testing.T, start string, driven, off bool) (d *api.Doc, parts []api.Group, closeAt func() float64) {
	t.Helper()
	dir := t.TempDir()
	d = api.New()
	noPlan(d, off)
	link := func(length float64, name string) api.Group {
		g, err := d.Assembly.Link(barFile(t, dir, length), name)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	ground, crank, coupler, rocker := link(60, "ground"), link(20, "crank"), link(60, "coupler"), link(50, "rocker")
	parts = []api.Group{crank, coupler, rocker}
	for _, g := range parts {
		g.AsLink().SetMoves(true)
	}
	ug, ucr, uco, ur := uprights(t, ground), uprights(t, crank), uprights(t, coupler), uprights(t, rocker)
	switch start {
	case "stretch":
		coupler.AsLink().Move(api.V(20, 0, 0))
		rocker.AsLink().Move(api.V(80, 0, 0))
	case "origin":
		for _, g := range parts {
			g.AsLink().Move(g.AsLink().Offset().ScaledBy(-1))
		}
	}
	if driven {
		d.Sketch.Anywhere3D()
		d.Assembly.HingeDriven(ug[0], ucr[0])
	} else {
		d.Assembly.Hinge(ug[0], ucr[0])
	}
	d.Assembly.Hinge(ucr[20], uco[0])
	d.Assembly.Hinge(uco[60], ur[0])
	end, pin := ur[50].Start(), ug[60]
	d.Constrain.On(end, pin)
	closeAt = func() float64 {
		a, b := pin.Start().Pos(), pin.End().Pos()
		return b.Minus(a).Cross(end.Pos().Minus(a)).Magnitude() / b.Minus(a).Magnitude()
	}
	return d, parts, closeAt
}

// assembled reports whether the four-bar's parts are solved with dof
// freedom and its loop closed.
func assembled(parts []api.Group, closeAt func() float64, dof int) bool {
	for _, g := range parts {
		if !g.Solved() || g.Dof() != dof {
			return false
		}
	}
	return closeAt() < 1e-6
}

// A four-bar drawn at full stretch, its bars in a line, assembles: the
// plan places each bar from the ground out and closes the loop by a
// search over the coupler's turn. From there the solve alone can't move
// the chain's end toward its pin, and leaves it unsolved.
func TestFourBarFromFullStretch(t *testing.T) {
	t.Parallel()
	for _, driven := range []bool{false, true} {
		dof := 1
		if driven {
			dof = 0
		}
		_, parts, closeAt := fourBarAt(t, "stretch", driven, false)
		if !assembled(parts, closeAt, dof) {
			t.Errorf("driven %v: not assembled, the loop open by %g", driven, closeAt())
		}
		if _, parts, closeAt := fourBarAt(t, "stretch", driven, true); assembled(parts, closeAt, dof) {
			t.Errorf("driven %v: the solve alone assembled it too; the test no longer shows the plan", driven)
		}
	}
}

// A four-bar whose bars are all where their links put them, on top of
// one another on the ground, assembles, and so does one whose bars are
// all at the origin, unturned. The solve alone leaves the first
// unsolved.
func TestFourBarFromOrigin(t *testing.T) {
	t.Parallel()
	for _, start := range []string{"link", "origin"} {
		for _, driven := range []bool{false, true} {
			dof := 1
			if driven {
				dof = 0
			}
			_, parts, closeAt := fourBarAt(t, start, driven, false)
			if !assembled(parts, closeAt, dof) {
				t.Errorf("%s, driven %v: not assembled, the loop open by %g", start, driven, closeAt())
			}
		}
	}
	if _, parts, closeAt := fourBarAt(t, "link", true, true); assembled(parts, closeAt, 0) {
		t.Error("the solve alone assembled it from where the links put it; the test no longer shows the plan")
	}
}

// A constraint the plan doesn't know leaves the mechanism to the solve,
// whole: a driven crank and a coupler whose far end is held at a
// distance from a point of the ground, a dimension, not a mate. The
// mechanism solves at each angle as before, and the plan places nothing.
func TestPlanFallsBack(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	d := api.New()
	link := func(length float64, name string) api.Group {
		g, err := d.Assembly.Link(barFile(t, dir, length), name)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	ground, crank, coupler := link(60, "ground"), link(20, "crank"), link(60, "coupler")
	for _, g := range []api.Group{crank, coupler} {
		g.AsLink().SetMoves(true)
	}
	ug, ucr, uco := uprights(t, ground), uprights(t, crank), uprights(t, coupler)
	turn := func(g api.Group, deg float64) {
		c, s := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
		g.AsLink().SetOrientation(api.V(c, s, 0), api.V(-s, c, 0))
	}
	turn(crank, 80)
	turn(coupler, 10)
	d.Sketch.Anywhere3D()
	d.Assembly.HingeDriven(ug[0], ucr[0])
	d.Assembly.Hinge(ucr[20], uco[0])
	end, o := uco[60].Start(), ug[60].Start()
	d.Constrain.Distance(end, o, end.Pos().Minus(o.Pos()).Magnitude())
	_, placed0, solved0 := planCounts(d)
	for _, v := range []float64{5, 10, 15} {
		setParam(t, d, "angle", v)
		d.Edit.Regen()
		if !crank.Solved() || !coupler.Solved() || coupler.Dof() != 0 {
			t.Errorf("angle %g: crank solved %v, coupler solved %v with %d DOF", v, crank.Solved(), coupler.Solved(), coupler.Dof())
		}
	}
	if _, placed, solved := planCounts(d); placed != placed0 || solved != solved0 {
		t.Errorf("the plan placed %d and stood for %d solves of a mechanism with a dimension in it", placed-placed0, solved-solved0)
	}
}

// A plan is compiled once for a mechanism and kept while its
// constraints are as they were, through a motion study; a constraint
// added compiles it again, and the plan goes on standing for the solve.
func TestPlanCacheInvalidated(t *testing.T) {
	t.Parallel()
	d, ground, crank, _, _, _ := fourBarDriven(t)
	d.Edit.Regen()
	compiles, _, _ := planCounts(d)
	if _, err := d.Assembly.MotionStudy("angle", 0, 90, 6, ""); err != nil {
		t.Fatal(err)
	}
	again, _, solved := planCounts(d)
	if again != compiles {
		t.Errorf("a motion study compiled the plan %d times again", again-compiles)
	}
	// A reference dimension, which holds nothing, but is a change.
	o, p := nearestPoint(ground, ground.AsLink().Offset()), nearestPoint(crank, crank.AsLink().Offset())
	d.With(func(sk *sketch.Sketch) {
		c := core.Constraint{Group: sk.ActiveGroup(), Workplane: core.FreeIn3D, Type: core.ConstraintPtPtDistance,
			PtA: o.H, PtB: p.H, Reference: true}
		sk.AddConstraintRemember(&c, false)
	})
	setParam(t, d, "angle", 30)
	d.Edit.Regen()
	after, _, solvedAfter := planCounts(d)
	if after != again+1 || solvedAfter <= solved {
		t.Errorf("a constraint added: compiled %d times, stood for %d solves; want once, and more", after-again, solvedAfter-solved)
	}
}

// The engine round a whole turn in steps of 45°, coarse: the piston
// stays above the crank's bearing at every step, on the branch it was
// assembled on, rather than flipping to the one below.
func TestEngineFullTurnFlipFree(t *testing.T) {
	t.Parallel()
	d, _, _, piston, bearing := engine(t)
	// The piston's corner, 10 below its pin, which runs from 38 to 62
	// above the bearing.
	corner := nearestPoint(piston, piston.AsLink().Offset())
	for deg := 0.0; deg <= 360; deg += 45 {
		setParam(t, d, "angle", deg)
		d.Edit.Regen()
		if !piston.Solved() {
			t.Errorf("angle %g: not solved", deg)
		}
		if y := corner.Pos().Y - bearing.Y; y < 20 {
			t.Errorf("angle %g: the piston's corner %g above the bearing, want 28 to 52", deg, y)
		}
	}
}
