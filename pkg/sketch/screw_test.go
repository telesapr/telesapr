package sketch_test

import (
	"bytes"
	"math"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// A screw mate (item 11 of the modelling upgrades): the second plate
// turns about the holes' axis by the parameter turn and moves along it
// by turn/360 of the lead, through whole turns and either way; the
// group keeps no freedom; undo, redo and a round trip keep it, and a
// motion study of turn screws the plate along.
func TestScrewMate(t *testing.T) {
	t.Parallel()
	all := func(api.Entity) bool { return true }
	d, a, b := matePart(t)
	d.Sketch.Anywhere3D()
	ca, cb := first(t, a, "circle", all), first(t, b, "circle", all)
	step(d)
	before := saved(t, d)
	mate := d.Assembly.Screw(ca, cb, 2)
	if mate.Turn != "turn" || mate.Lead != "lead" || len(mate.Constraints) != 5 || !b.Solved() || b.Dof() != 0 {
		t.Fatalf("screw: %+v, solved %v, dof %d", mate, b.Solved(), b.Dof())
	}
	// One step of undo; redo brings it back. The solve of the part's
	// placement starts again from where it ended, so the last bits may
	// move: the parameters and the mate's expressions are compared.
	if !d.Edit.Undo() || !bytes.Equal(saved(t, d), before) {
		t.Fatal("undo didn't take the whole mate back")
	}
	if !d.Edit.Redo() || len(d.Params.List()) != 2 || mate.Constraints[3].Expr() != "turn - 18.434948822922024" ||
		mate.Constraints[4].Expr() != "turn*lead/360" || !b.Solved() {
		t.Fatalf("redo: %v, %q, %q", d.Params.List(), mate.Constraints[3].Expr(), mate.Constraints[4].Expr())
	}

	// Where b's points start, about the axis through the hole's centre.
	axis := ca.Center().Pos()
	n := ca.Normal().Dir()
	start := map[uint32]api.Vec{}
	for _, e := range b.Entities() {
		if e.Kind() == "point" {
			start[e.H.V] = e.Pos()
		}
	}
	at := func(deg float64) {
		t.Helper()
		if err := d.Params.Set("turn", strconv.FormatFloat(deg, 'g', -1, 64)); err != nil {
			t.Fatal(err)
		}
		if !b.Solved() || b.Dof() != 0 {
			t.Fatalf("turn %g: solved %v, dof %d", deg, b.Solved(), b.Dof())
		}
		a := deg * math.Pi / 180
		for _, e := range b.Entities() {
			p0, ok := start[e.H.V]
			if !ok {
				continue
			}
			off := p0.Minus(axis)
			h := n.ScaledBy(off.Dot(n))
			r := off.Minus(h)
			want := axis.Plus(h).Plus(r.ScaledBy(math.Cos(a))).Plus(n.Cross(r).ScaledBy(math.Sin(a))).
				Plus(n.ScaledBy(2 * deg / 360))
			if got := e.Pos(); got.Minus(want).Magnitude() > 1e-6 {
				t.Fatalf("turn %g: %v at %v, want %v", deg, e, got, want)
			}
		}
	}
	for _, deg := range []float64{90, 180, 270, 450, 720, -135, 0} {
		at(deg)
	}
	if err := d.Params.Set("lead", "-1"); err != nil {
		t.Fatal(err)
	}
	if err := d.Params.Set("turn", "360"); err != nil {
		t.Fatal(err)
	}
	if got := cb.Center().Pos().Minus(axis).Dot(n); math.Abs(got+1) > 1e-6 {
		t.Fatalf("a left-hand turn moved %g, want -1", got)
	}
	frames, err := d.Assembly.MotionStudy("turn", 0, 1080, 6, "")
	if err != nil || len(frames) != 7 {
		t.Fatalf("motion study: %d frames, %v", len(frames), err)
	}
	for _, f := range frames {
		if !f.Solved {
			t.Fatalf("motion study: %+v didn't solve", f)
		}
	}
	// Opened again, it solves to the same place, but for the last bits
	// of the part's placement, which a new solve starts from.
	file := filepath.Join(t.TempDir(), "screw.tlspr")
	if err := d.File.Save(file); err != nil {
		t.Fatal(err)
	}
	back, err := api.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	if bb := back.Group(b.Name()); !bb.Solved() || bb.Dof() != 0 ||
		first(t, bb, "circle", all).Center().Pos().Minus(cb.Center().Pos()).Magnitude() > 1e-9 {
		t.Fatalf("after a round trip: solved %v dof %d", bb.Solved(), bb.Dof())
	}
	if err := back.Params.Set("turn", "45"); err != nil || !back.Group(b.Name()).Solved() {
		t.Fatalf("turned after a round trip: %v", err)
	}

	// A second screw names its own parameters.
	d2, a2, b2 := matePart(t)
	d2.Sketch.Anywhere3D()
	if err := d2.Params.Set("turn", "1"); err != nil {
		t.Fatal(err)
	}
	if m := d2.Assembly.Screw(first(t, a2, "circle", all), first(t, b2, "circle", all), 1); m.Turn != "turn2" || m.Lead != "lead" {
		t.Errorf("names %q %q", m.Turn, m.Lead)
	}
}

// Bad input stops with a message: lines, a lead of zero.
func TestScrewMateRefused(t *testing.T) {
	t.Parallel()
	all := func(api.Entity) bool { return true }
	d, a, b := matePart(t)
	d.Sketch.Anywhere3D()
	for name, fn := range map[string]func(){
		"lines":     func() { d.Assembly.Screw(first(t, a, "line", all), first(t, b, "line", all), 1) },
		"zero lead": func() { d.Assembly.Screw(first(t, a, "circle", all), first(t, b, "circle", all), 0) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: no error", name)
				}
			}()
			fn()
		}()
	}
}
