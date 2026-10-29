package api

import (
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// lensWallPoint is a point on the lens's wall over its first cubic
// (lens), a quarter of the way along the cubic, at height z.
func lensWallPoint(z float64) Vec {
	return V(4.53125, 4.5, z)
}

// A datum on the lens's wall picked off its middle keeps the place it
// was picked at: the plane touches the wall there, and when the lens is
// made taller the plane follows the place, a quarter of the way up the
// wall, through the file, the model written as a script, and undo. A
// datum on a flat face keeps no place.
func TestDatumsKeepWherePicked(t *testing.T) {
	t.Parallel()
	d := New()
	g := lens(t, d)
	s := g.Solid()
	p := lensWallPoint(1.25)
	d.Datums.NameFaceAt("wall", s.FaceAt(V(10, 6, 2.5)), p)
	if at, n := frameOf(d.Datums.Get("wall")); at.Minus(p).Magnitude() > 1e-3 || n.Y <= 0 || math.Abs(n.Z) > 1e-6 {
		t.Errorf("the wall's plane at %v, normal %v, not at %v", at, n, p)
	}
	rim := rimEdges(g, 5)[0]
	a, b := rim.Ends()
	near := a.Plus(b.Minus(a).ScaledBy(0.2))
	d.Activate(g)
	d.Datums.NameEdgeAt("rim", rim, near)
	rimAt, _ := frameOf(d.Datums.Get("rim"))
	if math.Abs(rimAt.Z-5) > 1e-9 || rimAt.Minus(a).Magnitude() > rimAt.Minus(b).Magnitude() {
		t.Errorf("the rim's line at %v, not nearer %v than %v", rimAt, a, b)
	}
	d.Activate(g)
	d.Datums.NameFaceAt("top", s.FaceAt(V(10, 0, 5)), V(3, 1, 5))
	if top := d.Group("top"); len(d.h.Sketch().Group(top.H).DatumAt) != 0 {
		t.Error("a flat face keeps where it was picked")
	}

	// Taller: the place is a quarter of the wall's height. The wall's
	// datum, named first, comes last, after the others named with the
	// lens active; with it active every datum follows.
	d.Activate(d.Group("wall"))
	step := d.Host().(*SketchHost).BeginEval
	step()
	if err := g.SetExpr(sketch.GroupExtent, "10"); err != nil {
		t.Fatal(err)
	}
	if at, _ := frameOf(d.Datums.Get("wall")); at.Minus(lensWallPoint(2.5)).Magnitude() > 1e-3 {
		t.Errorf("taller, the wall's plane at %v", at)
	}
	if at, _ := frameOf(d.Datums.Get("rim")); math.Abs(at.Z-10) > 1e-9 || math.Abs(at.X-rimAt.X) > 1e-9 {
		t.Errorf("taller, the rim's line at %v, was %v", at, rimAt)
	}
	if !d.Edit.Undo() {
		t.Fatal("no undo")
	}
	if at, _ := frameOf(d.Datums.Get("wall")); at.Minus(p).Magnitude() > 1e-3 {
		t.Errorf("undone, the wall's plane at %v", at)
	}

	path := filepath.Join(t.TempDir(), "lens.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	back := New()
	if err := back.File.Load(path); err != nil {
		t.Fatal(err)
	}
	want, _ := frameOf(d.Datums.Get("wall"))
	if at, _ := frameOf(back.Datums.Get("wall")); at.Minus(want).Magnitude() > 1e-9 {
		t.Errorf("loaded, the wall's plane at %v, saved at %v", at, want)
	}

	src, _ := d.Script()
	for _, call := range []string{`Datums.NameFaceAt("wall"`, `Datums.NameEdgeAt("rim"`, `Datums.NameFace("top"`} {
		if !strings.Contains(src, call) {
			t.Errorf("the script has no %s:\n%s", call, src)
		}
	}
}

// A place is taken on a face or an edge of the solid that is there.
func TestDatumsKeepWherePickedMisuse(t *testing.T) {
	t.Parallel()
	d := New()
	g := lens(t, d)
	failsWith(t, "can't name a datum", func() { d.Datums.NameFaceAt("bad name", g.Solid().FaceAt(V(10, 6, 2.5)), V(0, 0, 0)) })
	failsWith(t, "can't name a datum", func() { d.Datums.NameEdgeAt("bad name", rimEdges(g, 5)[0], V(0, 0, 0)) })
}

// A datum on the lens's rim picked near one end stays near that end
// when the profile's cubics are bent the other way: the loop then winds
// the other way and the rim's edge runs from its other end, which a
// place counted from the edge's start would follow to the far end. It
// stays there through the file and undo.
func TestDatumsKeepWherePickedTurnedRound(t *testing.T) {
	t.Parallel()
	d := New()
	d.Edit.SetChordTolerance(0.01)
	d.Features.SketchIn(d.Workplane("XY"), "")
	a := d.Sketch.Cubic(V(0, 0, 0), V(5, 8, 0), V(15, 8, 0), V(20, 0, 0))
	b := d.Sketch.Cubic(V(20, 0, 0), V(15, -8, 0), V(5, -8, 0), V(0, 0, 0))
	pa, pb := a.Points(), b.Points()
	d.Constrain.Coincident(pa[3], pb[0])
	d.Constrain.Coincident(pb[3], pa[0])
	g := d.Features.Extrude(5, "")
	var rim SolidEdge
	for _, e := range rimEdges(g, 5) {
		if e.Points()[1].Y > 0 {
			rim = e
		}
	}
	start, _ := rim.Ends()
	d.Datums.NameEdgeAt("rim", rim, V(1.7, 2.2, 5))
	was, _ := frameOf(d.Datums.Get("rim"))
	if was.X > 2 {
		t.Fatalf("the rim's datum at %v", was)
	}

	d.Activate(d.Group("rim"))
	d.Host().(*SketchHost).BeginEval()
	pa[1].MoveTo(V(5, -8, 0))
	pa[2].MoveTo(V(15, -8, 0))
	pb[1].MoveTo(V(15, 8, 0))
	pb[2].MoveTo(V(5, 8, 0))
	var turned SolidEdge
	for _, e := range rimEdges(g, 5) {
		if e.Name() == rim.Name() {
			turned = e
		}
	}
	if now, _ := turned.Ends(); now.Minus(start).Magnitude() < 1 {
		t.Fatalf("the edge still starts at %v: the test turns nothing round", now)
	}
	mirrored := V(was.X, -was.Y, was.Z)
	at, _ := frameOf(d.Datums.Get("rim"))
	if at.Minus(mirrored).Magnitude() > 1e-6 {
		t.Errorf("bent the other way, the rim's datum at %v, not %v", at, mirrored)
	}

	path := filepath.Join(t.TempDir(), "lens.tlspr")
	if err := d.File.Save(path); err != nil {
		t.Fatal(err)
	}
	back := New()
	if err := back.File.Load(path); err != nil {
		t.Fatal(err)
	}
	if got, _ := frameOf(back.Datums.Get("rim")); got.Minus(mirrored).Magnitude() > 1e-6 {
		t.Errorf("loaded, the rim's datum at %v, saved at %v", got, mirrored)
	}
	if !d.Edit.Undo() {
		t.Fatal("no undo")
	}
	if got, _ := frameOf(d.Datums.Get("rim")); got.Minus(was).Magnitude() > 1e-6 {
		t.Errorf("undone, the rim's datum at %v, was %v", got, was)
	}
}
