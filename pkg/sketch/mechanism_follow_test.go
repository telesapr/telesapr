package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// heading is where part g's own x points, in degrees about z.
func heading(g api.Group) float64 {
	u, _ := g.AsLink().Orientation()
	return math.Atan2(u.Y, u.X) * 180 / math.Pi
}

// slideDriven is a slider-crank driven by its slide: a ground bar of
// 60, held, a crank of 20 hinged on its first upright, and a rod of
// rodLen hinged on the crank's end, whose far end runs on the ground's
// top edge, its distance from the crank's pin the parameter travel, at
// 40. For each place of the slide the crank can be on either side of
// the edge.
func slideDriven(t *testing.T, rodLen float64) (d *api.Doc, crank, rod api.Group) {
	t.Helper()
	dir := t.TempDir()
	d = api.New()
	link := func(length float64, name string) api.Group {
		g, err := d.Assembly.Link(barFile(t, dir, length), name)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	ground, crank, rod := link(60, "ground"), link(20, "crank"), link(rodLen, "rod")
	ug, uc, ur := uprights(t, ground), uprights(t, crank), uprights(t, rod)
	for _, g := range []api.Group{crank, rod} {
		g.AsLink().SetMoves(true)
	}
	rod.SetAllowRedundant(true)
	turn := func(g api.Group, deg float64) {
		c, s := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
		g.AsLink().SetOrientation(api.V(c, s, 0), api.V(-s, c, 0))
	}
	turn(crank, 60)
	turn(rod, -20)
	d.Sketch.Anywhere3D()
	d.Assembly.Hinge(ug[0], uc[0])
	d.Assembly.Hinge(uc[20], ur[0])
	edge := first(t, ground, "line", func(e api.Entity) bool {
		a, b := ug[0].Start().Pos(), ug[60].Start().Pos()
		return near3(e.Start().Pos(), a) && near3(e.End().Pos(), b) || near3(e.End().Pos(), a) && near3(e.Start().Pos(), b)
	})
	slide := ur[rodLen].Start()
	d.Constrain.On(slide, edge)
	if err := d.Params.Set("travel", "40"); err != nil {
		t.Fatal(err)
	}
	if err := d.Constrain.Distance(slide, ug[0].Start(), 40).SetExpr("travel"); err != nil {
		t.Fatal(err)
	}
	if !rod.Solved() || rod.Dof() != 0 {
		t.Fatalf("the slider-crank: solved %v, %d DOF", rod.Solved(), rod.Dof())
	}
	return d, crank, rod
}

// angleDiff is a - b in degrees, taken between -180 and 180.
func angleDiff(a, b float64) float64 {
	return math.Mod(a-b+540, 360) - 180
}

// A motion study keeps the mechanism's assembly: the slider-crank
// driven by its slide, at 49, then by a study from 11 to 49 in two
// steps, the first of them 38 back, and put back at 49 after it, has
// its crank at each step where steps of 1 put it, on the same side of
// the slide. Each change of the drive is followed in steps that agree;
// solved in one, as before (NoFollow), the step of 38 lands the crank
// on the other side of the slide, mirrored, which this measures too.
// And the engine's crank driven round a whole turn in four steps comes
// back to where it started, the piston at each quarter where steps of
// 5° put it.
func TestMotionStudyNoFlip(t *testing.T) {
	t.Parallel()
	fine := map[float64]float64{}
	d, crank, _ := slideDriven(t, 30)
	for v := 40.0; v <= 49; v++ {
		setParam(t, d, "travel", v)
		fine[v] = heading(crank)
	}
	for v := 48.0; v >= 11; v-- {
		setParam(t, d, "travel", v)
		fine[v] = heading(crank)
	}
	for _, follow := range []bool{true, false} {
		d, crank, rod := slideDriven(t, 30)
		d.With(func(sk *sketch.Sketch) { sk.NoFollow = !follow })
		setParam(t, d, "travel", 49)
		// The study's steps by hand, where each can be read, then the
		// study itself.
		var worst float64
		for _, v := range []float64{11, 30, 49} {
			setParam(t, d, "travel", v)
			worst = max(worst, math.Abs(angleDiff(heading(crank), fine[v])))
			if !rod.Solved() {
				t.Errorf("follow %v, travel %g: not solved", follow, v)
			}
		}
		frames, err := d.Assembly.MotionStudy("travel", 11, 49, 2, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range frames {
			if !f.Solved {
				t.Errorf("follow %v, the study at %g: not solved", follow, f.Value)
			}
		}
		worst = max(worst, math.Abs(angleDiff(heading(crank), fine[49])))
		if follow && worst > 0.01 {
			t.Errorf("followed, the crank is %.3f° from where small steps put it", worst)
		}
		if !follow && worst < 10 {
			t.Errorf("not followed, the crank is only %.3f° from where small steps put it: no flip to measure", worst)
		}
		var followed int
		d.With(func(sk *sketch.Sketch) { followed = sk.FollowedDrives })
		if !follow && followed != 0 || follow && followed == 0 {
			t.Errorf("follow %v, and %d drives followed", follow, followed)
		}
	}

	d, _, _, piston, _ := engine(t)
	pin := first(t, piston, "circle", func(api.Entity) bool { return true }).Center()
	fineY := map[float64]float64{}
	for deg := 0.0; deg <= 360; deg += 5 {
		setParam(t, d, "angle", deg)
		fineY[deg] = pin.Pos().Y
	}
	d, _, _, piston, _ = engine(t)
	pin = first(t, piston, "circle", func(api.Entity) bool { return true }).Center()
	frames, err := d.Assembly.MotionStudy("angle", 0, 360, 4, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range frames {
		setParam(t, d, "angle", f.Value)
		if !f.Solved || math.Abs(pin.Pos().Y-fineY[f.Value]) > 1e-3 {
			t.Errorf("the engine at %g°: solved %v, the piston's pin at %.4f, want %.4f", f.Value, f.Solved, pin.Pos().Y, fineY[f.Value])
		}
	}
}

// A long drag keeps the mechanism's assembly: the four-bar's crank
// pulled round a whole turn by its far hole in jumps of 60°, and
// turned round in jumps of 90°, has its rocker where jumps of 5° put
// it, and comes back where it started.
func TestDragNoFlip(t *testing.T) {
	t.Parallel()
	run := func(jump float64, byTurn bool) map[float64]float64 {
		_, ground, crank, _, rocker, closeAt := fourBar(t, true)
		pivot := uprights(t, ground)[0].Start().Pos()
		// The crank's hole away from the pin.
		end := first(t, crank, "circle", func(e api.Entity) bool {
			return flat(e.Center().Pos().Minus(pivot)).Magnitude() > 5
		}).Center()
		start := angleAbout(end.Pos(), pivot)
		out := map[float64]float64{}
		for deg := jump; deg <= 360; deg += jump {
			if byTurn {
				end.Turn(api.V(0, 0, 1), jump)
			} else {
				end.MoveTo(turned(end.Pos(), pivot, angleDiff(start+deg, angleAbout(end.Pos(), pivot))))
			}
			if got := angleDiff(angleAbout(end.Pos(), pivot), start+deg); math.Abs(got) > 0.05 {
				t.Errorf("jump %g, by turn %v: at %g° the crank is %.3f° off", jump, byTurn, deg, got)
			}
			if closeAt() > 1e-6 || !rocker.Solved() {
				t.Errorf("jump %g, by turn %v: at %g° the loop is open by %g, solved %v", jump, byTurn, deg, closeAt(), rocker.Solved())
			}
			out[deg] = heading(rocker)
		}
		return out
	}
	ref := run(5, false)
	for _, tc := range []struct {
		jump   float64
		byTurn bool
	}{{60, false}, {90, true}} {
		for deg, h := range run(tc.jump, tc.byTurn) {
			if d := angleDiff(h, ref[deg]); math.Abs(d) > 0.05 {
				t.Errorf("jump %g, by turn %v: at %g° the rocker is %.3f° from where small jumps put it", tc.jump, tc.byTurn, deg, d)
			}
		}
	}
}
