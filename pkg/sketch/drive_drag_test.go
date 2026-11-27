package sketch_test

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// paramValue is parameter name's value.
func paramValue(t testing.TB, d *api.Doc, name string) float64 {
	t.Helper()
	p, ok := d.Params.Get(name)
	if !ok {
		t.Fatalf("no parameter %q", name)
	}
	return p.Value
}

// A drag of the driven lid of testdata/ui/mechanism/boxlid.tlspr, the
// lid whose only freedom is its drive: pulled up, the lid opens and the
// pull writes its opening into angle; pulled on past where the range of
// 0 to 110 ends, it stops at 110. Each pull is one step of undo, which
// takes the angle back with the lid.
func TestDragDrivenLid(t *testing.T) {
	t.Parallel()
	// Opened from a copy in the temporary directory, which is where
	// saved writes: a sketch with links saves only on the volume of the
	// files it links, and on Windows the checkout and the temporary
	// directory are on different drives.
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS(filepath.Join("..", "..", "testdata", "ui", "mechanism"))); err != nil {
		t.Fatal(err)
	}
	d, err := api.Open(filepath.Join(dir, "boxlid.tlspr"))
	if err != nil {
		t.Fatal(err)
	}
	gs := d.Groups()
	lid := gs[len(gs)-1]
	// The lid's front top corner, and the hinge it turns about: the
	// back top edge of the box, the lid's back bottom edge.
	var front api.Entity
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, e := range lid.Entities() {
		if e.Kind() != "point" {
			continue
		}
		p := e.Pos()
		lo, hi = math.Min(lo, p.Y), math.Max(hi, p.Y)
		if front.H.V == 0 || p.Z > front.Pos().Z+1e-6 || math.Abs(p.Z-front.Pos().Z) < 1e-6 && p.Y < front.Pos().Y-1e-6 {
			front = e
		}
	}
	at := front.Pos()
	hingeY, hingeZ := hi, at.Z-3
	opened := func(deg float64) api.Vec {
		s, c := math.Sincos(deg * math.Pi / 180)
		dy, dz := at.Y-hingeY, at.Z-hingeZ
		return api.V(at.X, hingeY+c*dy+s*dz, hingeZ-s*dy+c*dz)
	}
	step(d)
	before := saved(t, d)
	front.MoveTo(opened(60))
	if a := paramValue(t, d, "angle"); math.Abs(a-60) > 0.5 || !lid.Solved() || front.Pos().Minus(opened(60)).Magnitude() > 0.5 {
		t.Errorf("pulled to 60°: angle %g, solved %v, the corner at %v, want %v", a, lid.Solved(), front.Pos(), opened(60))
	}
	step(d)
	front.MoveTo(opened(150))
	if a := paramValue(t, d, "angle"); a != 110 || !lid.Solved() {
		t.Errorf("pulled to 150°: angle %g, solved %v, want 110 at the range's end", a, lid.Solved())
	}
	if front.Pos().Minus(opened(110)).Magnitude() > 1e-3 {
		t.Errorf("at 110: the corner at %v, want %v", front.Pos(), opened(110))
	}
	if !d.Edit.Undo() {
		t.Fatal("no undo")
	}
	if a := paramValue(t, d, "angle"); math.Abs(a-60) > 0.5 {
		t.Errorf("undo of the second pull: angle %g, want the first pull's", a)
	}
	if !d.Edit.Undo() || !bytes.Equal(saved(t, d), before) {
		t.Errorf("undo of the first pull: angle %g, the file not as before", paramValue(t, d, "angle"))
	}
	d.Edit.Redo()
	if !d.Edit.Redo() || paramValue(t, d, "angle") != 110 {
		t.Fatalf("redo: angle %g", paramValue(t, d, "angle"))
	}
	// Pulled on past the bound, it stays there.
	step(d)
	front.MoveTo(opened(170))
	if a := paramValue(t, d, "angle"); a != 110 || !lid.Solved() {
		t.Errorf("pulled again past the bound: angle %g, solved %v", a, lid.Solved())
	}
}

// A drag of the engine's piston, whose crank is driven: pulled up its
// cylinder, the piston slides, the crank turns, and the crank's angle
// takes the turn; the piston keeps to its slide.
func TestDragDrivenCrankEngine(t *testing.T) {
	t.Parallel()
	d, crank, _, piston, bearing := engine(t)
	// At 0 the crank stands up the cylinder, at the top of its stroke,
	// where the piston can't follow a pull: turned 40° on, it can.
	setParam(t, d, "angle", 40)
	pin := nearestPoint(piston, bearing.Plus(api.V(0, 1000, 0)))
	at := pin.Pos()
	axisX := at.X
	step(d)
	pin.MoveTo(at.Plus(api.V(0, -5, 0)))
	a := paramValue(t, d, "angle")
	if !piston.Solved() || math.Abs(a) < 1 || math.Abs(pin.Pos().X-axisX) > 1e-6 ||
		math.Abs(pin.Pos().Y-(at.Y-5)) > 0.1 {
		t.Errorf("piston pulled down 5: angle %g, solved %v, the pin from %v to %v", a, piston.Solved(), at, pin.Pos())
	}
	// The crank's turn is what the angle now says: set it again, and
	// nothing moves.
	p := pin.Pos()
	setParam(t, d, "angle", a)
	if pin.Pos().Minus(p).Magnitude() > 1e-3 || !crank.Solved() {
		t.Errorf("angle %g set again: the pin from %v to %v", a, p, pin.Pos())
	}
}
