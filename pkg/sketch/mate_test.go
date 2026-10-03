package sketch_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// matePart saves a 20 x 10 x 4 plate with a hole of radius 2 at (5,
// 5), and links it twice into a new document; the second link is
// active, so that the mate moves it.
func matePart(t *testing.T) (d *api.Doc, a, b api.Group) {
	t.Helper()
	p := api.New()
	p.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	p.Circle(api.V(5, 5, 0), 2)
	p.Extrude(4, "")
	path := filepath.Join(t.TempDir(), "plate.tlspr")
	if err := p.Save(path); err != nil {
		t.Fatal(err)
	}
	d = api.New()
	var err error
	if a, err = d.Link(path, "a"); err != nil {
		t.Fatal(err)
	}
	if b, err = d.Link(path, "b"); err != nil {
		t.Fatal(err)
	}
	return d, a, b
}

// first is g's first entity of kind whose test holds.
func first(t *testing.T, g api.Group, kind string, test func(api.Entity) bool) api.Entity {
	t.Helper()
	for _, e := range g.Entities() {
		if e.Kind() == kind && test(e) {
			return e
		}
	}
	t.Fatalf("%s has no such %s", g, kind)
	return api.Entity{}
}

func near3(a, b api.Vec) bool { return a.Minus(b).Magnitude() < 1e-6 }

// ends are g's faces at its lowest and highest z: the faces of an
// extrusion's ends keep the sketch's normal, so the top is the one
// higher up.
func ends(g api.Group) (lo, hi api.Entity) {
	for _, e := range g.Entities() {
		if e.Kind() != "face" || math.Abs(e.FaceNormal().Z) < 0.99 {
			continue
		}
		if lo.H.V == 0 || e.FacePoint().Z < lo.FacePoint().Z {
			lo = e
		}
		if hi.H.V == 0 || e.FacePoint().Z > hi.FacePoint().Z {
			hi = e
		}
	}
	return lo, hi
}

// The mates (roadmap 16.9): concentric puts the holes on one axis, by
// parallel axes and two distances across it of zero; flush puts the second plate's bottom
// in the plane of the first's top; a hinge puts one edge on another,
// free to turn. Each leaves the group solved.
func TestMates(t *testing.T) {
	all := func(api.Entity) bool { return true }

	d, a, b := matePart(t)
	ca, cb := first(t, a, "circle", all), first(t, b, "circle", all)
	cs := d.Concentric(ca, cb)
	if len(cs) != 3 || !b.Solved() || !near3(ca.Center().Pos(), cb.Center().Pos()) {
		t.Errorf("concentric: %d constraints, solved %v, centres %v %v", len(cs), b.Solved(), ca.Center().Pos(), cb.Center().Pos())
	}
	if n, m := ca.Normal().Dir(), cb.Normal().Dir(); n.Cross(m).Magnitude() > 1e-6 {
		t.Errorf("concentric: axes %v and %v", n, m)
	}
	// Flush as well, as a pin in a hole: the slide along the axis was
	// left free, so the two together solve.
	_, topA := ends(a)
	botB, _ := ends(b)
	if cs := d.Flush(topA, botB); len(cs) != 1 || !b.Solved() {
		t.Errorf("flush after concentric: %d constraints, solved %v", len(cs), b.Solved())
	}

	d, a, b = matePart(t)
	_, topA = ends(a)
	botB, _ = ends(b)
	cs = d.Flush(topA, botB)
	if len(cs) != 2 || !b.Solved() {
		t.Errorf("flush: %d constraints, solved %v", len(cs), b.Solved())
	}
	if pa, pb, n := topA.FacePoint(), botB.FacePoint(), topA.FaceNormal(); math.Abs(pb.Minus(pa).Dot(n)) > 1e-6 || n.Cross(botB.FaceNormal()).Magnitude() > 1e-6 {
		t.Errorf("flush: the faces are %g apart", pb.Minus(pa).Dot(n))
	}

	d, a, b = matePart(t)
	la, lb := first(t, a, "line", all), first(t, b, "line", all)
	cs = d.Hinge(la, lb)
	if len(cs) != 2 || !b.Solved() || !near3(la.Start().Pos(), lb.Start().Pos()) {
		t.Errorf("hinge: %d constraints, solved %v", len(cs), b.Solved())
	}
	if dir, off := la.End().Pos().Minus(la.Start().Pos()), lb.End().Pos().Minus(la.Start().Pos()); dir.Cross(off).Magnitude() > 1e-6 {
		t.Errorf("hinge: b's end is off a's line")
	}

	for name, fn := range map[string]func(){
		"concentric of lines": func() { d.Concentric(la, lb) },
		"flush of lines":      func() { d.Flush(la, lb) },
		"hinge of circles":    func() { d.Hinge(first(t, a, "circle", all), first(t, b, "circle", all)) },
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
