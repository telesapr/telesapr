package sketch_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// Blocks assembled beside a first one: one 2 mm off its side, one
// across a corner, 3 mm off along each of two axes, and one overlapping
// it. Flat faces are their meshes, so each gap is exact, and the nearest
// points are that far apart.
func TestClearancesOfBlocks(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 10, 10, 10)
	at := func(x0, y0, x1, y1 float64, name string) api.Group {
		d.Features.SketchIn(d.Workplane("XY"), "")
		d.Sketch.Rect(api.V(x0, y0, 0), api.V(x1, y1, 0))
		g := d.Features.Extrude(10, name)
		g.SetCombine(api.Assemble)
		return g
	}
	beside := at(12, 0, 20, 10, "beside")
	corner := at(-13, -13, -3, -3, "corner")
	all := d.Assembly.Clearances(0)
	// Three parts, three pairs: the block and beside 2 apart, beside and
	// corner further, the block and corner √18.
	if len(all) != 3 {
		t.Fatalf("%d pairs: %+v", len(all), all)
	}
	if c := all[0]; c.B.H != beside.H || math.Abs(c.Gap-2) > 1e-9 || math.Abs(c.PA.Minus(c.PB).Magnitude()-2) > 1e-9 {
		t.Errorf("nearest pair %s and %s, %g apart at %v %v", c.A, c.B, c.Gap, c.PA, c.PB)
	}
	if c := all[1]; c.B.H != corner.H || math.Abs(c.Gap-math.Sqrt(18)) > 1e-9 {
		t.Errorf("second pair %s and %s, %g apart", c.A, c.B, c.Gap)
	}
	if near := d.Assembly.Clearances(3); len(near) != 1 || near[0].B.H != beside.H {
		t.Errorf("below 3: %+v", near)
	}
	if none := d.Assembly.Clearances(1); len(none) != 0 {
		t.Errorf("below 1: %+v", none)
	}
	at(5, 5, 15, 15, "overlapping")
	if near := d.Assembly.Clearances(0.5); len(near) != 2 || near[0].Gap != 0 || near[1].Gap != 0 {
		t.Errorf("overlapping and touching: %+v", near)
	}
}

// A pin 9 across in a bore 10 across, coaxial: their meshes' gap is
// off the true 0.5 by up to their chord tolerances, and followed onto
// the cylinders it comes to 0.5.
func TestClearanceOfAPinInABore(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(-15, -15, 0), api.V(15, 15, 0))
	d.Features.Extrude(10, "plate").SetCombine(api.Assemble)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 5)
	d.Features.Extrude(10, "bore").SetCombine(api.Difference)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 4.5)
	pin := d.Features.Extrude(10, "pin")
	pin.SetCombine(api.Assemble)
	cs := d.Assembly.Clearances(0)
	if len(cs) != 1 {
		t.Fatalf("%d pairs: %+v", len(cs), cs)
	}
	c := cs[0]
	if c.B.H != pin.H || !c.Refined || math.Abs(c.Gap-0.5) > 1e-6 || c.Tolerance <= 0 {
		t.Fatalf("pin in bore: %+v", c)
	}
	if r := math.Hypot(c.PB.X, c.PB.Y); math.Abs(r-4.5) > 1e-6 {
		t.Errorf("the pin's point %v is %g off the axis", c.PB, r)
	}
}

// One part has nothing to be clear of.
func TestClearancesOfOnePart(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 10, 10, 10)
	if cs := d.Assembly.Clearances(0); len(cs) != 0 {
		t.Errorf("one part: %+v", cs)
	}
}

// A block open at its top, walls 1.5 thick, has no wall thinner: its
// floor and sides measure 1.5, and the rim the walls' height.
func TestWallThicknessOfAShelledBox(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	top := ex.Solid().FaceAt(api.V(10, 5, 10))
	sh := d.Features.Shell([]api.Entity{top.Entity()}, 1.5, "")
	r := sh.Analyze().WallThickness(2)
	if math.Abs(r.Thinnest.Thickness-1.5) > 1e-9 || r.Unmeasured != 0 || r.Samples == 0 {
		t.Fatalf("shelled box: thinnest %g, %d of %d unmeasured", r.Thinnest.Thickness, r.Unmeasured, r.Samples)
	}
	if len(r.Thin) == 0 || r.Thin[0].Thickness != r.Thinnest.Thickness {
		t.Errorf("%d thin spots, first %+v", len(r.Thin), r.Thin)
	}
	for _, s := range r.Thin {
		if math.Abs(s.Thickness-1.5) > 1e-9 || math.Abs(s.To.Minus(s.From).Magnitude()-1.5) > 1e-9 {
			t.Errorf("thin spot %+v", s)
		}
	}
	if r := sh.Analyze().WallThickness(1.4); len(r.Thin) != 0 {
		t.Errorf("thinner than 1.4: %d spots", len(r.Thin))
	}
	// The block before it is solid: its thinnest is its height.
	if r := ex.Analyze().WallThickness(0); math.Abs(r.Thinnest.Thickness-10) > 1e-9 || len(r.Thin) != 0 {
		t.Errorf("the block: thinnest %g, %d thin", r.Thinnest.Thickness, len(r.Thin))
	}
}

// A cup turned by a lathe and shelled 1 thick at its open top: its
// walls measure 1 within the reported tolerance.
func TestWallThicknessOfATurnedCup(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 22))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(8, 0, 0), api.V(8, 0, 20), api.V(0, 0, 20))
	lathe := d.Features.Lathe(axis, "")
	top := lathe.Solid().FaceAt(api.V(1, 1, 20))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	if p := sh.Problem(); p != "" {
		t.Fatal(p)
	}
	r := sh.Analyze().WallThickness(0)
	if r.Tolerance <= 0 || math.Abs(r.Thinnest.Thickness-1) > r.Tolerance+1e-9 || r.Unmeasured != 0 {
		t.Fatalf("cup: thinnest %g within %g, %d of %d unmeasured", r.Thinnest.Thickness, r.Tolerance, r.Unmeasured, r.Samples)
	}
	// Followed onto the cup's surfaces, the thinnest is between two of
	// them, the wall's two sides, 1 apart.
	if !r.Thinnest.Refined || math.Abs(r.Thinnest.Thickness-1) > 1e-6 {
		t.Fatalf("cup: thinnest %g within %g, %d of %d unmeasured", r.Thinnest.Thickness, r.Tolerance, r.Unmeasured, r.Samples)
	}
}

// Parts are measured apart: a block assembled into another's side
// doesn't make the wall between them thicker or thinner, and a group
// with no solid has nothing to measure.
func TestWallThicknessOfParts(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 10, 10, 2)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	b := d.Features.Extrude(-3, "under")
	b.SetCombine(api.Assemble)
	r := b.Analyze().WallThickness(0)
	if math.Abs(r.Thinnest.Thickness-2) > 1e-9 {
		t.Errorf("thinnest %g, want the plate's 2", r.Thinnest.Thickness)
	}
	e := api.New()
	sk := e.Features.SketchIn(e.Workplane("XY"), "")
	e.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	if !refused(func() { sk.Analyze().WallThickness(0) }) {
		t.Error("a sketch with no solid was measured")
	}
}

// A pin taken as a mesh in a bore: the pin's triangles are its
// geometry, and the gap is followed from them onto the bore's cylinder,
// so it comes to the pin's mesh's distance from the bore, not the
// meshes' less both chord tolerances.
func TestClearanceOfAMeshPinInABore(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Rect(api.V(-15, -15, 0), api.V(15, 15, 0))
	d.Features.Extrude(10, "plate").SetCombine(api.Assemble)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 5)
	d.Features.Extrude(10, "bore").SetCombine(api.Difference)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(api.V(0, 0, 0), 4.5)
	pin := d.Features.Extrude(10, "pin")
	pin.SetCombine(api.Assemble)
	pin.SetForceToMesh(true)
	cs := d.Assembly.Clearances(0)
	if len(cs) != 1 {
		t.Fatalf("%d pairs: %+v", len(cs), cs)
	}
	c := cs[0]
	if !c.Refined || math.Abs(math.Hypot(c.PA.X, c.PA.Y)-5) > 1e-6 {
		t.Fatalf("the bore's point isn't on its cylinder: %+v", c)
	}
	// The pin's mesh lies inside its circle by no more than its chord
	// tolerance, so the gap is 0.5 and a little more.
	if c.Gap < 0.5-1e-9 || c.Gap > 0.5+c.Tolerance+1e-9 {
		t.Errorf("gap %g within %g", c.Gap, c.Tolerance)
	}
}

// One group assembled as two blocks 2 apart is one part to Clearances,
// and two solids to ClearancesWithin, which finds the gap between them
// and says which solid a gap to the part is to.
func TestClearancesWithinAPart(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 10, 10, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(13, 0, 0), api.V(20, 10, 0))
	d.Sketch.Rect(api.V(22, 0, 0), api.V(30, 10, 0))
	pair := d.Features.Extrude(10, "pair")
	pair.SetCombine(api.Assemble)
	if cs := d.Assembly.Clearances(0); len(cs) != 1 || math.Abs(cs[0].Gap-3) > 1e-9 || cs[0].SolidB != 0 {
		t.Fatalf("by parts: %+v", cs)
	}
	cs := d.Assembly.ClearancesWithin(0)
	if len(cs) != 3 {
		t.Fatalf("by solids: %d pairs: %+v", len(cs), cs)
	}
	if c := cs[0]; c.A.H != pair.H || c.B.H != pair.H || math.Abs(c.Gap-2) > 1e-9 || c.SolidA == c.SolidB || c.SolidA == 0 {
		t.Errorf("between the pair's solids: %+v", c)
	}
	if c := cs[1]; c.B.H != pair.H || math.Abs(c.Gap-3) > 1e-9 || c.SolidB == 0 {
		t.Errorf("the block to the nearer of the pair: %+v", c)
	}
}

// A cone turned to a point: its wall thins to nothing at the tip, which
// the middles of the triangles about it, a step or more from it, miss;
// the finer pieces measure it far thinner.
func TestWallThicknessOfAConeTip(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "")
	axis := d.Sketch.Line(api.V(0, 0, -2), api.V(0, 0, 22))
	axis.SetConstruction(true)
	d.Sketch.Polyline(true, api.V(0, 0, 0), api.V(8, 0, 0), api.V(0, 0, 20))
	lathe := d.Features.Lathe(axis, "")
	r := lathe.Analyze().WallThickness(0)
	// The middle of a triangle from the tip to the base's rim is a third
	// of the way up from the base: there the cone is 2·8·2/3 across.
	if r.Thinnest.Thickness > 1 || r.Unmeasured > r.Samples/100 {
		t.Errorf("cone: thinnest %g at %v, %d of %d unmeasured", r.Thinnest.Thickness, r.Thinnest.From, r.Unmeasured, r.Samples)
	}
	// Its rim is as sharp as its tip.
	if p := r.Thinnest.From; p.Z < 18 && (p.Z > 1 || math.Hypot(p.X, p.Y) < 7) {
		t.Errorf("the thinnest is at %v, not by the tip or the rim", p)
	}
}

// A block cut by a narrow V-groove from its top, its tip 0.2 above the
// floor: the wall under the tip is 0.2 thick, and it thickens by eleven
// times the distance from the tip's line, so the middles of the pieces
// the floor is cut into, none on that line, measure it a good deal
// thicker. The thinnest is found where the floor meets the groove's
// tip, a vertex of where the rays from one floor triangle meet one of
// the groove's.
func TestWallThicknessUnderAGroove(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 30, 10, 10)
	d.Features.SketchIn(d.Workplane("ZX"), "")
	w := 9.8 * math.Tan(5*math.Pi/180)
	d.Sketch.Polyline(true, api.V(15-w, 0, 10), api.V(15+w, 0, 10), api.V(15, 0, 0.2))
	cut := d.Features.Extrude(10, "groove")
	cut.SetCombine(api.Difference)
	if p := cut.Problem(); p != "" {
		t.Fatal(p)
	}
	r := cut.Analyze().WallThickness(0.5)
	if math.Abs(r.Thinnest.Thickness-0.2) > 1e-6 {
		t.Fatalf("thinnest %g at %v, want 0.2 under the groove's tip", r.Thinnest.Thickness, r.Thinnest.From)
	}
	if p := r.Thinnest.From; math.Abs(p.X-15) > 1e-6 {
		t.Errorf("the thinnest is at %v, not under the tip", p)
	}
	if len(r.Thin) == 0 {
		t.Error("no spot listed thinner than 0.5")
	}
}

// The pair of TestClearancesWithinAPart taken as a mesh: its triangles
// fall in two pieces, joined by the corners they share, which
// ClearancesWithin takes apart as it takes a shell's solids.
func TestClearancesWithinAMeshPart(t *testing.T) {
	t.Parallel()
	d := api.New()
	block(d, 0, 0, 10, 10, 10)
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(api.V(13, 0, 0), api.V(20, 10, 0))
	d.Sketch.Rect(api.V(22, 0, 0), api.V(30, 10, 0))
	pair := d.Features.Extrude(10, "pair")
	pair.SetCombine(api.Assemble)
	pair.SetForceToMesh(true)
	if cs := d.Assembly.Clearances(0); len(cs) != 1 || math.Abs(cs[0].Gap-3) > 1e-9 || cs[0].SolidB != 0 {
		t.Fatalf("by parts: %+v", cs)
	}
	cs := d.Assembly.ClearancesWithin(0)
	if len(cs) != 3 {
		t.Fatalf("by pieces: %d pairs: %+v", len(cs), cs)
	}
	if c := cs[0]; c.A.H != pair.H || c.B.H != pair.H || math.Abs(c.Gap-2) > 1e-9 || c.SolidA == c.SolidB || c.SolidA == 0 {
		t.Errorf("between the mesh's pieces: %+v", c)
	}
	if c := cs[1]; c.B.H != pair.H || math.Abs(c.Gap-3) > 1e-9 || c.SolidB == 0 {
		t.Errorf("the block to the nearer piece: %+v", c)
	}
}
