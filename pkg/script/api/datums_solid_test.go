package api

import (
	"math"
	"path/filepath"
	"slices"
	"testing"
)

// saveRoundedPlate saves a plate 40 x 30 x 10 whose corner at (40, 30)
// is rounded by a fillet of 4, as plate.tlspr in dir, with datums on
// its solid: the fillet's face, which no entity makes, as rounded; the
// bottom face as base; the straight bottom edge along x as front. The
// plate's width is the parameter w.
func saveRoundedPlate(t *testing.T, dir string) string {
	t.Helper()
	p := New()
	p.Params.Set("w", "40")
	for _, l := range p.Sketch.Rect(V(0, 0, 0), V(40, 30, 0)) {
		a, b := l.Start(), l.End()
		switch {
		case a.Pos().Y == 0 && b.Pos().Y == 0:
			if err := p.Constrain.Length(l, 40).SetExpr("w"); err != nil {
				t.Fatal(err)
			}
		case a.Pos().X == 0 && b.Pos().X == 0:
			p.Constrain.Length(l, 30)
		}
		if a.Pos() == V(0, 0, 0) {
			p.Constrain.Coincident(a, p.Workplane("XY").Center())
		}
	}
	plate := p.Features.Extrude(10, "plate")
	corner := plate.Solid().EdgeAt(V(40, 30, 5))
	round := p.Features.FilletSolid([]SolidEdge{corner}, 4, "round")
	if pr := round.Problem(); pr != "" {
		t.Fatal(pr)
	}
	s := round.Solid()
	p.Datums.NameFace("rounded", s.FaceAt(V(40-4+4*math.Sqrt(0.5), 30-4+4*math.Sqrt(0.5), 5)))
	p.Datums.NameFace("base", s.FaceAt(V(20, 15, 0)))
	p.Datums.NameEdge("front", s.EdgeAt(V(20, 0, 0)))
	path := filepath.Join(dir, "plate.tlspr")
	if err := p.File.Save(path); err != nil {
		t.Fatal(err)
	}
	return path
}

// frameOf is the origin and unit normal of a workplane, or of a normal
// through a point, as a linked datum on a solid is.
func frameOf(e Entity) (at, n Vec) {
	at = e.Center().Pos()
	if e.Kind() == "workplane" {
		e = e.Normal()
	}
	u, v := e.Orientation()
	return at, u.Cross(v).WithMagnitude(1)
}

// A datum on a fillet's face is its axis, on a flat face a plane in it,
// on a straight edge a line along it; each follows its face or edge when
// the model changes, comes back from the file, and is one undo step.
func TestDatumsOnASolid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := saveRoundedPlate(t, dir)
	p := New()
	if err := p.File.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := p.Datums.Names(); !slices.Equal(got, []string{"rounded", "base", "front"}) {
		t.Fatalf("names %q", got)
	}
	if k := p.Datums.Get("rounded").Kind(); k != "workplane" {
		t.Fatalf("rounded is a %s", k)
	}
	at, n := frameOf(p.Datums.Get("rounded"))
	if math.Abs(at.X-36) > 1e-6 || math.Abs(at.Y-26) > 1e-6 || math.Abs(math.Abs(n.Z)-1) > 1e-9 {
		t.Errorf("the fillet's axis at %v along %v", at, n)
	}
	if at, n := frameOf(p.Datums.Get("base")); math.Abs(at.Z) > 1e-9 || math.Abs(n.Z+1) > 1e-9 {
		t.Errorf("the base at %v, its normal %v out of the plate", at, n)
	}
	if at, n := frameOf(p.Datums.Get("front")); math.Abs(at.Y) > 1e-9 || math.Abs(at.Z) > 1e-9 || math.Abs(math.Abs(n.X)-1) > 1e-9 {
		t.Errorf("the front edge from %v along %v", at, n)
	}
	// The last datum's group is active, as any new group is; the datums'
	// groups follow the fillet.
	if g := p.Active(); g.Name() != "front" {
		t.Errorf("active %s", g.Name())
	}

	// Wider by 10: the fillet's axis moves with the corner.
	step := p.Host().(*SketchHost).BeginEval
	step()
	p.Params.Set("w", "50")
	if at, _ := frameOf(p.Datums.Get("rounded")); math.Abs(at.X-46) > 1e-6 || math.Abs(at.Y-26) > 1e-6 {
		t.Errorf("after the edit the axis is at %v", at)
	}
	if !p.Edit.Undo() {
		t.Fatal("no undo")
	}
	if at, _ := frameOf(p.Datums.Get("rounded")); math.Abs(at.X-36) > 1e-6 {
		t.Errorf("undone, the axis is at %v", at)
	}

	// Naming is one undo step, the group and the name together.
	step()
	n0 := len(p.Groups())
	p.Datums.NameEdge("side", p.Group("round").Solid().EdgeAt(V(0, 15, 0)))
	if len(p.Groups()) != n0+1 || !slices.Contains(p.Datums.Names(), "side") {
		t.Fatalf("named: %d groups, %q", len(p.Groups()), p.Datums.Names())
	}
	if !p.Edit.Undo() {
		t.Fatal("no undo")
	}
	if len(p.Groups()) != n0 || slices.Contains(p.Datums.Names(), "side") {
		t.Errorf("undone: %d groups, %q", len(p.Groups()), p.Datums.Names())
	}
	failsWith(t, "can't name a datum", func() { p.Datums.NameFace("bad name", p.Group("round").Solid().FaceAt(V(20, 15, 10))) })
	if len(p.Groups()) != n0 {
		t.Errorf("a refused name left a group: %d", len(p.Groups()))
	}
}

// Datums on solids come through links and mates take them: two rounded
// plates stood one on the other by their fillets' axes and their bases.
func TestDatumsOnASolidThroughLinks(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := saveRoundedPlate(t, dir)
	d := New()
	if _, err := d.Assembly.Link(path, "a"); err != nil {
		t.Fatal(err)
	}
	b, err := d.Assembly.Link(path, "b")
	if err != nil {
		t.Fatal(err)
	}
	b.AsLink().Move(V(100, 50, 30))
	d.Assembly.Concentric(d.Datums.Get("a.rounded"), d.Datums.Get("b.rounded"))
	d.Assembly.Flush(d.Datums.Get("b.base"), d.Datums.Get("a.base"))
	ab, an := frameOf(d.Datums.Get("a.rounded"))
	bb, bn := frameOf(d.Datums.Get("b.rounded"))
	if bb.Minus(ab).Cross(an).Magnitude() > 1e-6 || math.Abs(math.Abs(an.Dot(bn))-1) > 1e-9 {
		t.Errorf("the axes at %v along %v and %v along %v", ab, an, bb, bn)
	}
	if pa, _ := frameOf(d.Datums.Get("a.base")); true {
		if pb, _ := frameOf(d.Datums.Get("b.base")); math.Abs(pa.Z-pb.Z) > 1e-6 {
			t.Errorf("the bases at z %g and %g", pa.Z, pb.Z)
		}
	}
}
