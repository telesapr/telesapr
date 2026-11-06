package sketch_test

import (
	"bytes"
	"math"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// setParam sets parameter name to v.
func setParam(t *testing.T, d *api.Doc, name string, v float64) {
	t.Helper()
	if err := d.Params.Set(name, strconv.FormatFloat(v, 'g', -1, 64)); err != nil {
		t.Fatal(err)
	}
}

// A driven hinge: the bar turns about the ground's pin by the parameter
// angle, from where it was, either way and past a whole turn, and keeps
// no freedom, so a pull moves nothing; the mate is one step of undo,
// and redo brings it back. The pin runs from the top down, so a
// positive angle is clockwise seen from above.
func TestDrivenHinge(t *testing.T) {
	t.Parallel()
	d, ground, bar := twoBars(t)
	d.Sketch.Anywhere3D()
	ug, ub := uprights(t, ground), uprights(t, bar)
	far, pivot := ub[60].Start(), ug[60].Start().Pos()
	step(d)
	before := saved(t, d)
	mate := d.Assembly.HingeDriven(ug[60], ub[0])
	if mate.Param != "angle" || len(mate.Constraints) != 3 || !bar.Solved() || bar.Dof() != 0 {
		t.Fatalf("driven hinge: %v, solved %v, %d DOF", mate, bar.Solved(), bar.Dof())
	}
	if !d.Edit.Undo() || !bytes.Equal(saved(t, d), before) {
		t.Fatal("undo didn't take the whole mate back")
	}
	if !d.Edit.Redo() || !bar.Solved() || bar.Dof() != 0 || len(d.Params.List()) != 1 {
		t.Fatalf("redo: solved %v, %d DOF, %v", bar.Solved(), bar.Dof(), d.Params.List())
	}
	start := far.Pos()
	for _, deg := range []float64{30, 90, -45, 400} {
		setParam(t, d, "angle", deg)
		if want := turned(start, pivot, -deg); !near3(far.Pos(), want) || !bar.Solved() {
			t.Errorf("angle %g: the corner at %v, want %v; solved %v", deg, far.Pos(), want, bar.Solved())
		}
	}
	at := far.Pos()
	far.MoveTo(turned(at, pivot, 20))
	if !near3(far.Pos(), at) {
		t.Errorf("a pull moved a driven hinge to %v", far.Pos())
	}
}

// A driven concentric mate slides the second plate along the holes'
// axis by the parameter travel, from where it was.
func TestDrivenConcentric(t *testing.T) {
	t.Parallel()
	all := func(api.Entity) bool { return true }
	d, a, b := matePart(t)
	d.Sketch.Anywhere3D()
	ca, cb := first(t, a, "circle", all), first(t, b, "circle", all)
	mate := d.Assembly.ConcentricDriven(ca, cb)
	if mate.Param != "travel" || !b.Solved() {
		t.Fatalf("driven concentric: %v, solved %v", mate, b.Solved())
	}
	n := ca.Normal().Dir()
	at := cb.Center().Pos().Minus(ca.Center().Pos()).Dot(n)
	setParam(t, d, "travel", 7.5)
	if got := cb.Center().Pos().Minus(ca.Center().Pos()).Dot(n); math.Abs(got-at-7.5) > 1e-6 || !b.Solved() {
		t.Errorf("travel 7.5: along the axis %g, from %g; solved %v", got, at, b.Solved())
	}
}

// A range on a driven hinge: a motion study of angle from 0 to 180 with
// the range 0 to 90 solves to 90 and fails past it.
func TestDrivenHingeLimited(t *testing.T) {
	t.Parallel()
	d, ground, bar := twoBars(t)
	d.Sketch.Anywhere3D()
	ug, ub := uprights(t, ground), uprights(t, bar)
	mate := d.Assembly.HingeDriven(ug[60], ub[0])
	if bounds := mate.SetRange(0, 90); len(bounds) != 2 {
		t.Fatalf("%d bounds", len(bounds))
	}
	frames, err := d.Assembly.MotionStudy("angle", 0, 180, 6, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range frames {
		if want := f.Value <= 90+1e-9; f.Solved != want {
			t.Errorf("angle %g: solved %v, want %v", f.Value, f.Solved, want)
		}
	}
}

// A four-bar driven by its crank: a motion study of the crank's angle
// round a whole turn in twelve steps solves at every step; set to each
// angle in turn, the crank is at it, not half a turn off it (the turn's
// half-plane holds there too), the loop is closed and no freedom left.
// The crank's pin runs from the top down, so its own x turns by minus
// the angle about z.
func TestDrivenCrankFourBar(t *testing.T) {
	t.Parallel()
	d, _, crank, _, rocker, closeAt := fourBarDriven(t)
	frames, err := d.Assembly.MotionStudy("angle", 0, 360, 12, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range frames {
		if !f.Solved {
			t.Errorf("crank at %g°: not solved", f.Value)
		}
	}
	heading := func() float64 {
		u, _ := crank.AsLink().Orientation()
		return math.Atan2(u.Y, u.X) * 180 / math.Pi
	}
	setParam(t, d, "angle", 0)
	start := heading()
	for deg := 30.0; deg <= 360; deg += 30 {
		setParam(t, d, "angle", deg)
		turned := math.Mod(start-heading()+720, 360)
		if diff := math.Abs(math.Mod(turned-deg+540, 360) - 180); diff > 1e-6 {
			t.Errorf("crank at %g°: turned %.4f°", deg, turned)
		}
		if off := closeAt(); off > 1e-6 || !rocker.Solved() || rocker.Dof() != 0 {
			t.Errorf("crank at %g°: the loop open by %g, solved %v, %d DOF", deg, off, rocker.Solved(), rocker.Dof())
		}
	}
}

// A driven hinge of a pin in its hole: two plates' holes on one axis,
// the second flush on the first, turned about the holes' axis by angle,
// the hole's centre kept where it is.
func TestDrivenHingeOfPins(t *testing.T) {
	t.Parallel()
	all := func(api.Entity) bool { return true }
	d, a, b := matePart(t)
	b.AsLink().SetMoves(true)
	d.Sketch.Anywhere3D()
	ca, cb := first(t, a, "circle", all), first(t, b, "circle", all)
	mate := d.Assembly.HingeDriven(ca, cb)
	_, topA := ends(a)
	botB, _ := ends(b)
	d.Assembly.Flush(topA, botB)
	if mate.Param != "angle" || !b.Solved() || b.Dof() != 0 {
		t.Fatalf("driven pin: %v, solved %v, %d DOF", mate, b.Solved(), b.Dof())
	}
	u0, _ := b.AsLink().Orientation()
	setParam(t, d, "angle", 90)
	u1, _ := b.AsLink().Orientation()
	if !b.Solved() || !near3(ca.Center().Pos().Minus(api.V(0, 0, ca.Center().Pos().Z)), cb.Center().Pos().Minus(api.V(0, 0, cb.Center().Pos().Z))) ||
		math.Abs(u0.Dot(u1)) > 1e-6 {
		t.Errorf("angle 90: solved %v, centres %v %v, the plate's x from %v to %v", b.Solved(), ca.Center().Pos(), cb.Center().Pos(), u0, u1)
	}
}

// testdata/ui/mechanism/boxlid.tlspr, the guide's box and lid hinged as
// examples/scripts/lid hinges them, opens solved, closed and with no
// freedom; its angle opens the lid, and past the range of 0 to 110 it
// fails to solve.
func TestBoxLidFixture(t *testing.T) {
	t.Parallel()
	d, err := api.Open(filepath.Join("..", "..", "testdata", "ui", "mechanism", "boxlid.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	gs := d.Groups()
	lid := gs[len(gs)-1]
	if !lid.AsLink().Moves() || !lid.Solved() || lid.Dof() != 0 {
		t.Fatalf("opened: moves %v, solved %v, %d DOF", lid.AsLink().Moves(), lid.Solved(), lid.Dof())
	}
	for _, tc := range []struct {
		angle  float64
		solved bool
	}{{90, true}, {105, true}, {120, false}, {0, true}} {
		setParam(t, d, "angle", tc.angle)
		if lid.Solved() != tc.solved {
			t.Errorf("angle %g: solved %v, want %v", tc.angle, lid.Solved(), tc.solved)
		}
	}
}
