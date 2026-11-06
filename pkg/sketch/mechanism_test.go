package sketch_test

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// mechanismLinks saves a 60 x 10 x 4 bar with a hole at each end,
// drawn from its origin along x, and links it n times into a new
// document.
func mechanismLinks(t *testing.T, n int) (*api.Doc, []api.Group) {
	t.Helper()
	b := api.New()
	b.Sketch.Rect(api.V(0, 0, 0), api.V(60, 10, 0))
	b.Sketch.Circle(api.V(5, 5, 0), 2)
	b.Sketch.Circle(api.V(55, 5, 0), 2)
	b.Features.Extrude(4, "")
	path := filepath.Join(t.TempDir(), "bar.tlspr")
	if err := b.File.Save(path); err != nil {
		t.Fatal(err)
	}
	d := api.New()
	links := make([]api.Group, n)
	for i := range n {
		g, err := d.Assembly.Link(path, string(rune('a'+i)))
		if err != nil {
			t.Fatal(err)
		}
		links[i] = g
	}
	return d, links
}

func names(gs []api.Group) []string {
	var s []string
	for _, g := range gs {
		s = append(s, g.Name())
	}
	return s
}

// A mechanism is the flagged links in a row: an unflagged link, a
// group of another kind or a link suppressed whole ends it, and a part
// that doesn't move is a mechanism of one.
func TestMechanism(t *testing.T) {
	t.Parallel()
	d, l := mechanismLinks(t, 4)
	for _, g := range l[1:] {
		g.AsLink().SetMoves(true)
	}
	if !l[1].AsLink().Moves() || l[0].AsLink().Moves() {
		t.Fatalf("moves: a %v, b %v", l[0].AsLink().Moves(), l[1].AsLink().Moves())
	}
	for _, tc := range []struct {
		g    api.Group
		want []string
	}{{l[0], []string{"a"}}, {l[1], []string{"b", "c", "d"}}, {l[3], []string{"b", "c", "d"}}} {
		if got := names(tc.g.AsLink().Mechanism()); !slices.Equal(got, tc.want) {
			t.Errorf("mechanism of %s: %v, want %v", tc.g.Name(), got, tc.want)
		}
	}
	l[2].AsLink().SetMoves(false)
	if got := names(l[1].AsLink().Mechanism()); !slices.Equal(got, []string{"b"}) {
		t.Errorf("c unflagged: mechanism of b %v", got)
	}
	l[2].AsLink().SetMoves(true)
	l[2].SetSuppress(api.SuppressWhole)
	if got := names(l[3].AsLink().Mechanism()); !slices.Equal(got, []string{"d"}) {
		t.Errorf("c suppressed: mechanism of d %v", got)
	}
	l[2].SetSuppress(api.Unsuppressed)

	// A 3D sketch after d, and a flagged link after it: two mechanisms.
	d.Features.Sketch3D("datum")
	e, err := d.Assembly.Link(l[0].AsLink().File(), "e")
	if err != nil {
		t.Fatal(err)
	}
	step(d)
	e.AsLink().SetMoves(true)
	if got, other := names(e.AsLink().Mechanism()), names(l[1].AsLink().Mechanism()); !slices.Equal(got, []string{"e"}) || len(other) != 3 {
		t.Errorf("after a sketch: mechanism of e %v, of b %v", got, other)
	}

	// One step of undo takes the flag back, and redo brings it again.
	if !d.Edit.Undo() || e.AsLink().Moves() {
		t.Error("undo left e moving")
	}
	if !d.Edit.Redo() || !e.AsLink().Moves() {
		t.Error("redo didn't bring the flag back")
	}
}

// Only a link moves in a mechanism.
func TestSetMovesRefusedOnSketch(t *testing.T) {
	t.Parallel()
	d := api.New()
	g := d.Features.Sketch3D("s")
	defer func() {
		if _, ok := recover().(error); !ok {
			t.Error("SetMoves on a sketch: no error")
		}
	}()
	g.AsLink().SetMoves(true)
}

// barFile saves a bar length x 10 x 4 drawn from its origin along x,
// with a hole at each end, into dir.
func barFile(t *testing.T, dir string, length float64) string {
	t.Helper()
	b := api.New()
	b.Sketch.Rect(api.V(0, 0, 0), api.V(length, 10, 0))
	b.Sketch.Circle(api.V(5, 5, 0), 2)
	b.Sketch.Circle(api.V(length-5, 5, 0), 2)
	b.Features.Extrude(4, "")
	path := filepath.Join(dir, fmt.Sprintf("bar%g.tlspr", length))
	if err := b.File.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// uprights are the upright edges of a linked bar, by their distance
// along it from its origin's: the bar is found where its link put it,
// before anything turned it.
func uprights(t *testing.T, g api.Group) map[float64]api.Entity {
	t.Helper()
	x0, y0 := math.Inf(1), math.Inf(1)
	for _, e := range g.Entities() {
		if e.Kind() == "line" {
			x0, y0 = min(x0, e.Start().Pos().X), min(y0, e.Start().Pos().Y)
		}
	}
	out := map[float64]api.Entity{}
	for _, e := range g.Entities() {
		if e.Kind() != "line" {
			continue
		}
		s, f := e.Start().Pos(), e.End().Pos()
		if math.Abs(s.X-f.X) > 1e-9 || math.Abs(s.Y-f.Y) > 1e-9 || math.Abs(s.Y-y0) > 1e-9 || s.Z < f.Z {
			continue
		}
		out[math.Round(s.X-x0)] = e
	}
	return out
}

// fourBar is a ground bar of 60, a link held where it is, and a crank
// of 20, a coupler of 60 and a rocker of 50 hinged in a loop on it,
// their upright edges the pins: the crank on the ground's first, the
// coupler on the crank's end, the rocker on the coupler's end, and the
// rocker's end on the ground's second, closing the loop. A loop of
// hinges about parallel axes would be redundant in 3d, so the last pin
// puts the rocker's end on the ground's upright line, two equations.
// With moving, the three are flagged before they are mated.
func fourBar(t *testing.T, moving bool) (d *api.Doc, ground, crank, coupler, rocker api.Group, closeAt func() float64) {
	t.Helper()
	return makeFourBar(t, moving, false)
}

// fourBarDriven is fourBar, moving, with its crank's hinge driven by
// the parameter angle.
func fourBarDriven(t *testing.T) (d *api.Doc, ground, crank, coupler, rocker api.Group, closeAt func() float64) {
	t.Helper()
	return makeFourBar(t, true, true)
}

// makeFourBar is fourBar, its crank's hinge driven when driven.
func makeFourBar(t *testing.T, moving, driven bool) (d *api.Doc, ground, crank, coupler, rocker api.Group, closeAt func() float64) {
	t.Helper()
	dir := t.TempDir()
	d = api.New()
	link := func(length float64, name string) api.Group {
		g, lerr := d.Assembly.Link(barFile(t, dir, length), name)
		if lerr != nil {
			t.Fatal(lerr)
		}
		return g
	}
	ground, crank, coupler, rocker = link(60, "ground"), link(20, "crank"), link(60, "coupler"), link(50, "rocker")
	if moving {
		for _, g := range []api.Group{crank, coupler, rocker} {
			g.AsLink().SetMoves(true)
		}
	}
	ug, ucr, uco, ur := uprights(t, ground), uprights(t, crank), uprights(t, coupler), uprights(t, rocker)
	// The bars turned roughly as they will be: in a straight line the
	// chain is at full stretch, where its end can't move along it to
	// first order, and the loop would not close from there.
	turn := func(g api.Group, deg float64) {
		c, s := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
		g.AsLink().SetOrientation(api.V(c, s, 0), api.V(-s, c, 0))
	}
	turn(crank, 80)
	turn(coupler, 10)
	turn(rocker, -100)
	if driven {
		d.Sketch.Anywhere3D()
		d.Assembly.HingeDriven(ug[0], ucr[0])
	} else {
		d.Assembly.Hinge(ug[0], ucr[0])
	}
	d.Assembly.Hinge(ucr[20], uco[0])
	d.Assembly.Hinge(uco[60], ur[0])
	end := ur[50].Start()
	d.Constrain.On(end, ug[60])
	closeAt = func() float64 {
		a, b := ug[60].Start().Pos(), ug[60].End().Pos()
		p := end.Pos()
		return b.Minus(a).Cross(p.Minus(a)).Magnitude() / b.Minus(a).Magnitude()
	}
	return d, ground, crank, coupler, rocker, closeAt
}

// The three bars of a four-bar linkage flagged to move solve as one
// system: each is solved, the mechanism keeps the one freedom a
// four-bar has, and the loop is closed.
func TestFourBarSolves(t *testing.T) {
	t.Parallel()
	_, _, crank, coupler, rocker, closeAt := fourBar(t, true)
	for _, g := range []api.Group{crank, coupler, rocker} {
		if !g.Solved() || g.Dof() != 1 {
			t.Errorf("%s: solved %v, %d DOF; want solved, 1", g.Name(), g.Solved(), g.Dof())
		}
	}
	if off := closeAt(); off > 1e-6 {
		t.Errorf("the loop is open by %g", off)
	}
}

// Without the flag each bar's group is solved alone, the earlier ones
// held, as before mechanisms: the mates are in the rocker's group,
// which moves only the rocker, so the crank and the coupler stay where
// they were turned and the loop can't be closed.
func TestUnflaggedFourBarAsBefore(t *testing.T) {
	t.Parallel()
	_, _, crank, coupler, rocker, closeAt := fourBar(t, false)
	for _, tc := range []struct {
		g   api.Group
		deg float64
	}{{crank, 80}, {coupler, 10}} {
		u, _ := tc.g.AsLink().Orientation()
		if want := api.V(math.Cos(tc.deg*math.Pi/180), math.Sin(tc.deg*math.Pi/180), 0); !near3(u, want) {
			t.Errorf("%s turned to %v, want %v as it was put", tc.g.Name(), u, want)
		}
	}
	if rocker.Solved() || closeAt() < 1 {
		t.Errorf("unflagged: rocker solved %v, the loop open by %g", rocker.Solved(), closeAt())
	}
}

// A change in any part of a mechanism solves all of them again: the
// rocker moved away, or the crank turned, the loop closes again with
// every part solved, the others moved with it.
func TestMechanismDirtyFromAnyMember(t *testing.T) {
	t.Parallel()
	_, _, crank, coupler, rocker, closeAt := fourBar(t, true)
	parts := []api.Group{crank, coupler, rocker}
	check := func(what string) {
		t.Helper()
		for _, g := range parts {
			if !g.Solved() {
				t.Errorf("%s: %s not solved", what, g.Name())
			}
		}
		if off := closeAt(); off > 1e-6 {
			t.Errorf("%s: the loop is open by %g", what, off)
		}
	}
	rocker.AsLink().Move(api.V(5, 5, 0))
	check("rocker moved")
	before, _ := coupler.AsLink().Orientation()
	c, s := math.Cos(110*math.Pi/180), math.Sin(110*math.Pi/180)
	crank.AsLink().SetOrientation(api.V(c, s, 0), api.V(-s, c, 0))
	check("crank turned")
	if after, _ := coupler.AsLink().Orientation(); near3(before, after) {
		t.Errorf("the crank turned and the coupler stayed at %v", after)
	}
}

// A crank and a coupler whose far end slides along the ground's edge: a
// crank-slider. The slide puts a point on a line, two equations, one of
// which the pins already make (the point is at the edge's height), so
// the coupler allows redundancy, which then holds for the mechanism.
func TestCrankSlider(t *testing.T) {
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
	ug, ucr, uco := uprights(t, ground), uprights(t, crank), uprights(t, coupler)
	for _, g := range []api.Group{crank, coupler} {
		g.AsLink().SetMoves(true)
	}
	coupler.SetAllowRedundant(true)
	turn := func(g api.Group, deg float64) {
		c, s := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
		g.AsLink().SetOrientation(api.V(c, s, 0), api.V(-s, c, 0))
	}
	turn(crank, 60)
	turn(coupler, -20)
	d.Assembly.Hinge(ug[0], ucr[0])
	d.Assembly.Hinge(ucr[20], uco[0])
	// The ground's top edge along it, from its first upright's top.
	var edge api.Entity
	for _, e := range ground.Entities() {
		if e.Kind() == "line" && near3(e.Start().Pos(), ug[0].Start().Pos()) && near3(e.End().Pos(), ug[60].Start().Pos()) ||
			e.Kind() == "line" && near3(e.End().Pos(), ug[0].Start().Pos()) && near3(e.Start().Pos(), ug[60].Start().Pos()) {
			edge = e
		}
	}
	if edge.H.V == 0 {
		t.Fatal("no top edge along the ground")
	}
	slide := uco[60].Start()
	d.Constrain.On(slide, edge)
	for _, g := range []api.Group{crank, coupler} {
		if !g.Solved() || g.Dof() != 1 {
			t.Errorf("%s: solved %v, %d DOF; want solved, 1", g.Name(), g.Solved(), g.Dof())
		}
	}
	a, b, p := edge.Start().Pos(), edge.End().Pos(), slide.Pos()
	if off := b.Minus(a).Cross(p.Minus(a)).Magnitude() / b.Minus(a).Magnitude(); off > 1e-6 {
		t.Errorf("the slide is %g off the edge", off)
	}
}

// hingedBar is a ground bar of 60, held, and a bar of 60 flagged to
// move, hinged at its origin's upright edge to the ground's far one.
// It returns the bar's far top corner and the hinge's pivot.
func hingedBar(t *testing.T) (d *api.Doc, ground, bar api.Group, far api.Entity, pivot api.Vec) {
	t.Helper()
	d, ground, bar = twoBars(t)
	ug, ub := uprights(t, ground), uprights(t, bar)
	d.Assembly.Hinge(ug[60], ub[0])
	if !bar.Solved() || bar.Dof() != 1 {
		t.Fatalf("hinged: solved %v, %d DOF", bar.Solved(), bar.Dof())
	}
	return d, ground, bar, ub[60].Start(), ug[60].Start().Pos()
}

// twoBars is a ground bar of 60, held, and a bar of 60 linked after it
// and flagged to move, not yet mated.
func twoBars(t *testing.T) (d *api.Doc, ground, bar api.Group) {
	t.Helper()
	dir := t.TempDir()
	d = api.New()
	ground, err := d.Assembly.Link(barFile(t, dir, 60), "ground")
	if err != nil {
		t.Fatal(err)
	}
	if bar, err = d.Assembly.Link(barFile(t, dir, 60), "bar"); err != nil {
		t.Fatal(err)
	}
	bar.AsLink().SetMoves(true)
	return d, ground, bar
}

// turned is p turned by deg about the upright through pivot.
func turned(p, pivot api.Vec, deg float64) api.Vec {
	c, s := math.Cos(deg*math.Pi/180), math.Sin(deg*math.Pi/180)
	v := p.Minus(pivot)
	return pivot.Plus(api.V(c*v.X-s*v.Y, s*v.X+c*v.Y, v.Z))
}

// angleAbout is the turn of p about the upright through pivot, in
// degrees.
func angleAbout(p, pivot api.Vec) float64 {
	v := p.Minus(pivot)
	return math.Atan2(v.Y, v.X) * 180 / math.Pi
}

// flat is v with no z.
func flat(v api.Vec) api.Vec { return api.V(v.X, v.Y, 0) }

// A bar hinged at its own origin, pulled by its far end, turns about
// its hinge: put there and solved, it would come back, as its hinge
// can bring it back only by moving it, never by turning it. Ten pulls
// of a millimetre along the arc, and one of 90°, each end on the arc.
func TestPullHingedBar(t *testing.T) {
	t.Parallel()
	_, _, bar, far, pivot := hingedBar(t)
	r := flat(far.Pos().Minus(pivot)).Magnitude()
	angle := angleAbout(far.Pos(), pivot)
	for i := range 10 {
		at := far.Pos()
		tangent := api.V(-(at.Y - pivot.Y), at.X-pivot.X, 0)
		far.MoveTo(at.Plus(tangent.ScaledBy(1 / tangent.Magnitude())))
		now := angleAbout(far.Pos(), pivot)
		if now <= angle+0.5 {
			t.Fatalf("pull %d: the bar turned from %.3f° to %.3f°", i, angle, now)
		}
		angle = now
		if off := flat(far.Pos().Minus(pivot)).Magnitude() - r; math.Abs(off) > 0.05 {
			t.Errorf("pull %d: the end is %g off its arc", i, off)
		}
	}
	to := turned(far.Pos(), pivot, 90)
	far.MoveTo(to)
	if off := far.Pos().Minus(to).Magnitude(); off > 0.05 || !bar.Solved() {
		t.Errorf("a pull of 90°: %g from the target, solved %v", off, bar.Solved())
	}
}

// Pulling the coupler of a four-bar moves the crank and the rocker with
// it, the loop kept closed.
func TestPullFourBar(t *testing.T) {
	t.Parallel()
	_, _, crank, coupler, rocker, closeAt := fourBar(t, true)
	// A point of the coupler away from its pins: its first one.
	var pt api.Entity
	for _, e := range coupler.Entities() {
		if e.Kind() == "point" {
			pt = e
			break
		}
	}
	cu, _ := crank.AsLink().Orientation()
	ru, _ := rocker.AsLink().Orientation()
	at := pt.Pos()
	to := at.Plus(api.V(0, 3, 0))
	pt.MoveTo(to)
	if off := closeAt(); off > 1e-6 {
		t.Errorf("the loop is open by %g", off)
	}
	for _, g := range []api.Group{crank, coupler, rocker} {
		if !g.Solved() {
			t.Errorf("%s not solved", g.Name())
		}
	}
	// One freedom: the point goes along its path, which here runs
	// steeply across the pull, as near the target as the path comes.
	before, after := to.Minus(at).Magnitude(), to.Minus(pt.Pos()).Magnitude()
	if moved := pt.Pos().Minus(at).Magnitude(); moved < 0.1 || after > before-1e-3 {
		t.Errorf("the coupler's point moved %g, from %g to %g of its target", moved, before, after)
	}
	cu2, _ := crank.AsLink().Orientation()
	ru2, _ := rocker.AsLink().Orientation()
	if near3(cu, cu2) || near3(ru, ru2) {
		t.Errorf("crank %v → %v, rocker %v → %v: both should turn", cu, cu2, ru, ru2)
	}
}

// A mechanism with no freedom left doesn't move when pulled; a part
// that doesn't move in one is put where it is pulled, as before.
func TestPullFixedStays(t *testing.T) {
	t.Parallel()
	d, ground, bar, far, pivot := hingedBar(t)
	// The bar's far end held on the ground's top edge, run on: no turn
	// left. The edge's line is at the end's height already, which the
	// hinge makes, so one of its two equations is redundant.
	ug := uprights(t, ground)
	var edge api.Entity
	for _, e := range ground.Entities() {
		if e.Kind() == "line" && (near3(e.Start().Pos(), ug[0].Start().Pos()) && near3(e.End().Pos(), ug[60].Start().Pos()) ||
			near3(e.End().Pos(), ug[0].Start().Pos()) && near3(e.Start().Pos(), ug[60].Start().Pos())) {
			edge = e
		}
	}
	bar.SetAllowRedundant(true)
	d.Constrain.On(far, edge)
	if !bar.Solved() || bar.Dof() != 0 {
		t.Fatalf("held: solved %v, %d DOF", bar.Solved(), bar.Dof())
	}
	at := far.Pos()
	far.MoveTo(turned(at, pivot, 30))
	if moved := far.Pos().Minus(at).Magnitude(); moved > 1e-6 || !bar.Solved() {
		t.Errorf("held, it moved %g, solved %v", moved, bar.Solved())
	}

	p := ug[0].Start()
	to := p.Pos().Plus(api.V(0, 0, 7))
	p.MoveTo(to)
	if !near3(p.Pos(), to) {
		t.Errorf("the ground's point went to %v, not %v", p.Pos(), to)
	}
}

// A pull stops at a bound: the bar's end kept at least 100 from the
// ground's first upright turns it no more than 2·acos(100/120), 67.1°,
// from straight, however far it is pulled.
func TestPullStopsAtBound(t *testing.T) {
	t.Parallel()
	d, ground, bar, far, pivot := hingedBar(t)
	start := far.Pos()
	o := uprights(t, ground)[0].Start()
	d.Constrain.Distance(far, o, 100).SetLimit(api.Minimum)
	far.MoveTo(turned(start, pivot, 30))
	if got := angleAbout(far.Pos(), pivot) - angleAbout(start, pivot); math.Abs(got-30) > 0.5 {
		t.Errorf("a pull of 30°, inside the bound, turned it %.2f°", got)
	}
	far.MoveTo(turned(start, pivot, 90))
	got := angleAbout(far.Pos(), pivot) - angleAbout(start, pivot)
	limit := 2 * math.Acos(100.0/120) * 180 / math.Pi
	if dist := far.Pos().Minus(o.Pos()).Magnitude(); got > limit+0.01 || got < limit-1 || dist < 100-1e-6 || !bar.Solved() {
		t.Errorf("a pull of 90°: turned %.3f° (bound at %.3f°), %g from the upright, solved %v", got, limit, dist, bar.Solved())
	}
}

// axisAngle adds a turn of point pb about line, through pa, held at
// its angle now and by more, in the active group, and returns it.
func axisAngle(d *api.Doc, pa, line, pb api.Entity, by float64) core.HConstraint {
	var h core.HConstraint
	d.With(func(sk *sketch.Sketch) {
		c := core.Constraint{Group: sk.ActiveGroup(), Workplane: core.FreeIn3D, Type: core.ConstraintAxisAngle,
			PtA: pa.H, EntityA: line.H, PtB: pb.H}
		c.ValA = c.AxisAngleNum(sk) + by
		h = sk.AddConstraintRemember(&c, false)
	})
	return h
}

// A turn about a line: the hinged bar's far corner held 30° round from
// where it is about the ground's pin, a line held where it is; then
// about the bar's own pin, which turns with the bar, a point of the
// ground held 30° the other way round in the bar's frame. Each turns
// the bar 30° and leaves it no freedom. The uprights run from the top
// down, so a turn about them is clockwise seen from above: -30° about
// z.
func TestAxisAngleAboutLine(t *testing.T) {
	t.Parallel()
	for _, own := range []bool{false, true} {
		d, ground, bar, far, pivot := hingedBar(t)
		ug := uprights(t, ground)
		start := far.Pos()
		if own {
			// The bar's pin, its upright on the ground's: found by its
			// start, which the hinge put on the ground's.
			var pin api.Entity
			for _, e := range bar.Entities() {
				if e.Kind() == "line" && near3(e.Start().Pos(), ug[60].Start().Pos()) && !near3(e.End().Pos(), e.Start().Pos()) &&
					math.Abs(e.End().Pos().Z-ug[60].End().Pos().Z) < 1e-9 {
					pin = e
				}
			}
			axisAngle(d, pin.Start(), pin, ug[0].Start(), -30)
		} else {
			axisAngle(d, ug[60].Start(), ug[60], far, 30)
		}
		got := angleAbout(far.Pos(), pivot) - angleAbout(start, pivot)
		if math.Abs(got+30) > 1e-6 || !bar.Solved() || bar.Dof() != 0 {
			t.Errorf("own pin %v: turned %.6f° about z, solved %v, %d DOF; want -30, solved, 0", own, got, bar.Solved(), bar.Dof())
		}
	}
}

// A turn about a line takes a bound: a maximum of 45° from where the
// bar is stops a pull of 90° there, and a minimum of -20° one the
// other way. The turns are about the ground's upright, from its top
// down: clockwise seen from above, so a pull is by minus its angle
// about z.
func TestAxisAngleBound(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		limit    core.Limit
		by, pull float64
		wantTurn float64
	}{{api.Maximum, 45, 90, 45}, {api.Minimum, -20, -60, -20}} {
		d, ground, bar, far, pivot := hingedBar(t)
		start := far.Pos()
		h := axisAngle(d, uprights(t, ground)[60].Start(), uprights(t, ground)[60], far, tc.by)
		d.With(func(sk *sketch.Sketch) {
			if err := sk.SetConstraintLimit(h, tc.limit); err != nil {
				t.Fatal(err)
			}
		})
		far.MoveTo(turned(start, pivot, -tc.pull))
		got := angleAbout(start, pivot) - angleAbout(far.Pos(), pivot)
		if math.Abs(got-tc.wantTurn) > 0.01 || !bar.Solved() {
			t.Errorf("%v %g°, pulled %g°: turned %.4f°, solved %v", tc.limit, tc.by, tc.pull, got, bar.Solved())
		}
	}
}

// A part that moves in a mechanism but has no mate yet slides where a
// point of it is pulled, as any linked part's did before mechanisms,
// rather than turning about its origin, which a far point's pull would
// take for the least change were a turn weighed as a move.
func TestPullFreePartSlides(t *testing.T) {
	t.Parallel()
	_, _, bar := twoBars(t)
	far := uprights(t, bar)[60].Start()
	u0, w0 := bar.AsLink().Orientation()
	at := far.Pos()
	to := at.Plus(api.V(0, -40, 0))
	far.MoveTo(to)
	u1, w1 := bar.AsLink().Orientation()
	if !near3(far.Pos(), to) || u0.Minus(u1).Magnitude() > 1e-3 || w0.Minus(w1).Magnitude() > 1e-3 {
		t.Errorf("pulled 40 aside: the corner at %v, want %v; turned from %v, %v to %v, %v", far.Pos(), to, u0, w0, u1, w1)
	}
}
