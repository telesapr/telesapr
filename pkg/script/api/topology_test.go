package api

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"
)

var slotModel = filepath.Join("..", "..", "..", "testdata", "topo", "slot.spcs")

func openSlot(t *testing.T) *Doc {
	t.Helper()
	d, err := Open(slotModel)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSolidTopology(t *testing.T) {
	d := openSlot(t)
	slot := d.Group("slot")
	s := slot.Solid()
	if len(s.Problems()) != 0 || s.Euler() != 2 {
		t.Errorf("problems %v, Euler %d", s.Problems(), s.Euler())
	}
	// The top and the bottom, the front in two, the back, the ends, and
	// the notch's three walls.
	if n := len(s.Faces()); n != 10 {
		t.Errorf("%d faces", n)
	}
	if n := len(s.Vertices()); n != 16 {
		t.Errorf("%d vertices", n)
	}
	// The top's right piece meets the front's along x, from 25 to 40.
	right := s.EdgeAt(V(32, 0, 10))
	a, b := right.Ends()
	if math.Min(a.X, b.X) != 25 || math.Max(a.X, b.X) != 40 || !right.Straight() || right.Length() != 15 {
		t.Errorf("ends %v %v", a, b)
	}
	fs := right.Faces()
	between := s.EdgesBetween(fs[0], fs[1])
	if len(between) != 2 || between[1].Name() != right.Name() {
		t.Errorf("between: %d, the right one %q, want %q", len(between), between[1].Name(), right.Name())
	}
	if again := s.Edge(right.Name()); again.Name() != right.Name() {
		t.Error("the name reads back")
	}
	if !strings.Contains(right.Describe(), "the edge between face") || !strings.Contains(right.Describe(), "number 2") {
		t.Errorf("describe %q", right.Describe())
	}
	// Eight edges run along x: the front's four, cut in two by the
	// notch, the back's two, and the notch's back wall's two.
	if n := len(s.EdgesAlong(V(1, 0, 0))); n != 8 {
		t.Errorf("%d edges along x", n)
	}
	// The notch's rim on the top, an edge no entity draws.
	rim := s.EdgeAt(V(25, 20, 10))
	rf := rim.Faces()
	if _, err := func() (e Entity, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = r.(error)
			}
		}()
		return rf[1].Entity(), nil
	}(); err != nil && rf[0].ID() == 0 {
		t.Errorf("rim faces %v", err)
	}
	if len(rf[0].Edges()) < 4 || len(rf[0].Neighbours()) < 4 || rf[0].Name() == "" {
		t.Errorf("a face with %d edges, %d neighbours", len(rf[0].Edges()), len(rf[0].Neighbours()))
	}
	v := s.Vertices()[0]
	if len(v.Edges()) != 3 {
		t.Errorf("a corner with %d edges", len(v.Edges()))
	}
}

func TestChamferSolidEdge(t *testing.T) {
	d := openSlot(t)
	s := d.Group("slot").Solid()
	right := s.EdgeAt(V(32, 0, 10))
	ch := d.ChamferSolid([]SolidEdge{right}, 1, "")
	if p := ch.Problem(); p != "" {
		t.Fatal(p)
	}
	r := ch.Check()
	want := 10000.0 - 0.5*15
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-6 {
		t.Errorf("sound %v, volume %g, want %g", r.Sound(), r.Volume, want)
	}
	names := ch.SolidEdges()
	if len(names) != 1 || names[0].Name() != right.Name() {
		t.Errorf("the chamfer's edges %v", names)
	}
	// The edges of the chamfer's own solid have names too: chamfer the
	// notch's upright edge, which no entity draws either.
	again := d.ChamferSolid([]SolidEdge{ch.Solid().EdgeAt(V(25, 0, 5))}, 0.5, "")
	if p := again.Problem(); p != "" {
		t.Errorf("chamfer of the notch's edge: %s", p)
	}
	if r := again.Check(); !r.Sound() {
		t.Errorf("the second chamfer: %s", r.Summary())
	}
}

// TestNamingSweep moves the slot across the block: the chamfer of the
// top's right piece of the front edge, by name, must stay on that
// piece, or say it can't, never move to another edge.
func TestNamingSweep(t *testing.T) {
	d := openSlot(t)
	right := d.Group("slot").Solid().EdgeAt(V(32, 0, 10))
	ch := d.ChamferSolid([]SolidEdge{right}, 1, "")
	reported := 0
	for x := -12.2; x <= 43; x += 2.5 {
		if err := d.SetParameter("slot", fmt.Sprint(x)); err != nil {
			t.Fatal(err)
		}
		p := ch.Problem()
		// The front is in two while the notch lies inside it; then the
		// second piece runs from the notch's right side to x = 40.
		if x <= 0 || x+10 >= 40 {
			if p == "" {
				t.Errorf("notch at %g: the chamfer found a second edge", x)
			}
			reported++
			continue
		}
		if p != "" {
			t.Errorf("notch at %g: %s", x, p)
			continue
		}
		lo, hi := x+10, 40.0
		r := ch.Check()
		want := 10000 - 0.5*(hi-lo)
		if !r.Sound() || math.Abs(r.Volume-want) > 1e-6 {
			t.Errorf("notch at %g: volume %g, want %g (the chamfer moved?)", x, r.Volume, want)
		}
		e := ch.SolidEdges()[0]
		a, b := e.Ends()
		if math.Abs(math.Min(a.X, b.X)-lo) > 1e-9 || math.Max(a.X, b.X) != hi {
			t.Errorf("notch at %g: the edge runs %g to %g", x, math.Min(a.X, b.X), math.Max(a.X, b.X))
		}
	}
	if reported == 0 {
		t.Error("no position took the edge away")
	}
}

func TestSolidMisuse(t *testing.T) {
	d := openSlot(t)
	s := d.Group("slot").Solid()
	sketch := d.Group("slot-sketch")
	for name, fn := range map[string]func(){
		"bad name":        func() { s.Edge("edge zz") },
		"name of nothing": func() { s.Edge("edge 1 2 0") },
		"edge off":        func() { s.EdgeAt(V(100, 0, 0)) },
		"no direction":    func() { s.EdgesAlong(V(0, 0, 0)) },
		"chamfer bad name": func() {
			d.ChamferSolid([]SolidEdge{{s: s, name: s.Edges()[0].name}, {s: s, name: s.Edges()[0].name}}, 1, "")
		},
		"chamfer curved":          func() { d.ChamferSolid(nil, 1, "") },
		"solid edges of a sketch": func() { sketch.SolidEdges() },
		"solid of a deleted group": func() {
			g := d.Sketch3D("gone")
			g.Delete()
			g.Solid().Faces()
		},
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
	// A solid made only of a mesh has no names.
	m := New()
	if _, err := m.Link(filepath.Join("..", "..", "..", "testdata", "step", "shapes", "box.step"), ""); err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if _, ok := recover().(error); !ok {
				t.Error("a mesh's faces")
			}
		}()
		m.Group("box").Solid().Faces()
	}()
}

// TestSolidSheets extrudes a circle: its side is one face of four
// patches, its rims one edge each of four pieces, and it has no
// vertex.
func TestSolidSheets(t *testing.T) {
	d := New()
	d.SetChordTolerance(0.01)
	d.Circle(V(0, 0, 0), 10)
	s := d.Extrude(5, "body").Solid()
	if n := len(s.Faces()); n != 3 {
		t.Errorf("%d faces, want 3", n)
	}
	side := s.FaceAt(V(10, 0, 2.5))
	if side.Patches() != 4 || len(side.Edges()) != 2 || len(side.Neighbours()) != 2 {
		t.Errorf("the side: %d patches, %d edges, %d neighbours", side.Patches(), len(side.Edges()), len(side.Neighbours()))
	}
	if n := len(s.Edges()); n != 2 {
		t.Errorf("%d edges, want 2", n)
	}
	if n := len(s.Vertices()); n != 0 {
		t.Errorf("%d vertices, want none", n)
	}
	rim := s.EdgeAt(V(0, 10, 5))
	if rim.Pieces() != 4 || rim.Straight() || math.Abs(rim.Length()-20*math.Pi) > 0.05 {
		t.Errorf("the rim: %d pieces, straight %v, length %g", rim.Pieces(), rim.Straight(), rim.Length())
	}
	// Every piece of the rim is the same edge.
	for _, p := range []Vec{V(10, 0, 5), V(-10, 0, 5), V(0, -10, 5), V(7.0710678, 7.0710678, 5)} {
		if e := s.EdgeAt(p); e.Name() != rim.Name() {
			t.Errorf("the rim at %v is %s, not %s", p, e.Name(), rim.Name())
		}
	}
	if a, b := rim.Ends(); !a.Equals(b) {
		t.Errorf("the rim's ends %v %v", a, b)
	}
	top := s.FaceAt(V(0, 0, 5))
	if bt := s.EdgesBetween(top, side); len(bt) != 1 || bt[0].Name() != rim.Name() {
		t.Errorf("between the top and the side: %v", bt)
	}
	fs := rim.Faces()
	if fs[0].Patches()+fs[1].Patches() != 5 {
		t.Errorf("the rim's faces have %d and %d patches", fs[0].Patches(), fs[1].Patches())
	}
	// Filleting the rim by its name rounds it all round.
	f := d.FilletSolid([]SolidEdge{rim}, 1, "")
	if p := f.Problem(); p != "" {
		t.Fatal(p)
	}
	// Pappus: the spandrel of radius 1 turned about the axis, its
	// centroid (10-3pi)/(12-3pi) in from the rim.
	want := math.Pi*100*5 - 2*math.Pi*(10-(10-3*math.Pi)/(12-3*math.Pi))*(1-math.Pi/4)
	if r := f.Check(); !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want {
		t.Errorf("the fillet: %s, want volume %g", r.Summary(), want)
	}
}
