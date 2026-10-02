package sketch_test

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// twoProfiles makes a square of side a in the XY plane and, in a
// workplane h above, a square of side b, and returns their groups.
func twoProfiles(d *api.Doc, a, b, h float64) (api.Group, api.Group) {
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	bottom := d.SketchIn(d.Workplane("XY"), "bottom")
	d.Rect(api.V(-a/2, -a/2, 0), api.V(a/2, a/2, 0))
	top := d.SketchAt(above(d, h), api.V(1, 0, 0), api.V(0, 1, 0), "top")
	d.Rect(api.V(-b/2, -b/2, h), api.V(b/2, b/2, h))
	return bottom, top
}

// above is a point at height h on the z axis, in 3d, for a workplane's
// origin.
func above(d *api.Doc, h float64) api.Entity {
	d.Sketch3D("")
	return d.Point(api.V(0, 0, h))
}

func TestLoftFrustum(t *testing.T) {
	d := api.New()
	bottom, top := twoProfiles(d, 10, 4, 6)
	step(d)
	before := saved(t, d)
	l := d.Loft(bottom, top, "")
	if p := l.Problem(); p != "" {
		t.Fatal(p)
	}
	r := l.Check()
	want := 6.0 / 3 * (100 + 16 + 40)
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-9*want || l.Kind() != "loft" {
		t.Fatalf("frustum: %s, want %g", r.String(), want)
	}
	undoRedo(t, d, before)
	// Turned by one, the square twists a quarter turn, and its faces
	// cross; turned by four, it is as it was.
	l.SetTurn(1)
	if r := l.Check(); r.Sound() || l.Turn() != 1 {
		t.Fatalf("a quarter turn: %s", r.String())
	}
	l.SetTurn(4)
	if r := l.Check(); !r.Sound() || math.Abs(r.Volume-want) > 1e-6 {
		t.Fatalf("a whole turn: %s", r.String())
	}
	back := roundTrip(t, d)
	if bl := back.Group(l.Name()); bl.Kind() != "loft" || bl.Turn() != 4 {
		t.Fatalf("reopened: %s turn %d", bl.Kind(), bl.Turn())
	}
	if s := l.Solid(); len(s.Problems()) != 0 || len(s.Faces()) != 6 {
		t.Fatalf("topology: %v, %d faces", s.Problems(), len(s.Faces()))
	}
}

func TestLoftCircleToSquare(t *testing.T) {
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	bottom := d.SketchIn(d.Workplane("XY"), "circle")
	d.Circle(api.V(0, 0, 0), 5)
	top := d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "square")
	d.Rect(api.V(-3, -3, 10), api.V(3, 3, 10))
	l := d.Loft(bottom, top, "")
	r := l.Check()
	// Between the solids lofting the circle's inscribed square and its
	// circumscribed one to the square.
	inner := 10.0 / 3 * (50 + 36 + math.Sqrt(50*36))
	outer := 10.0 / 3 * (100 + 36 + 60)
	if l.Problem() != "" || !r.Sound() || r.Volume < inner || r.Volume > outer {
		t.Fatalf("circle to square: %s (%s), want between %g and %g", r.String(), l.Problem(), inner, outer)
	}
}

// A square to a square turned by an eighth: of the four ways of pairing
// their sides, two twist by an eighth of a turn, one way and the other,
// which are sound and mirror each other, so hold one volume; a third
// may be sound too.
func TestLoftTwist(t *testing.T) {
	d := api.New()
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0.001 })
	bottom := d.SketchIn(d.Workplane("XY"), "square")
	d.Rect(api.V(-5, -5, 0), api.V(5, 5, 0))
	top := d.SketchAt(above(d, 10), api.V(1, 0, 0), api.V(0, 1, 0), "diamond")
	d.Polyline(true, api.V(5, 0, 10), api.V(0, 5, 10), api.V(-5, 0, 10), api.V(0, -5, 10))
	l := d.Loft(bottom, top, "")
	var sound []float64
	for k := 0; k < 4; k++ {
		l.SetTurn(k)
		if r := l.Check(); r.Sound() {
			sound = append(sound, r.Volume)
		}
	}
	mirrored := false
	for i := range sound {
		for j := i + 1; j < len(sound); j++ {
			mirrored = mirrored || math.Abs(sound[i]-sound[j]) < 1e-6*sound[i]
		}
	}
	if !mirrored {
		t.Errorf("sound turns' volumes %v: no two mirror each other", sound)
	}
}
