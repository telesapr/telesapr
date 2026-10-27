package sketch_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// What a feature is built from is changed after it is made (ROADMAP
// F7): each setter below is one undo step, is saved, and refuses what
// the constructor refuses, leaving the feature as it was.

// sound requires g sound with no problem, and returns its volume.
func sound(t *testing.T, g api.Group, what string) float64 {
	t.Helper()
	r := g.Analyze().Check()
	if p := g.Problem(); p != "" || !r.Sound() {
		t.Fatalf("%s: problem %q, %s", what, p, r.String())
	}
	return r.Volume
}

// refusedWith requires f to fail, as an api call does, with a message
// holding want.
func refusedWith(t *testing.T, want string, f func()) {
	t.Helper()
	var got error
	func() {
		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); ok {
					got = err
				} else {
					got = fmt.Errorf("%v", r)
				}
			}
		}()
		f()
	}()
	if got == nil {
		t.Errorf("not refused, want %q", want)
		return
	}
	if !strings.Contains(got.Error(), want) {
		t.Errorf("refused with %q, want %q", got, want)
	}
}

// A fillet's edges changed: one top edge rounded, then the other two
// instead; the volume follows, undo and redo come back identical, and
// the file round-trips.
func TestFilletEdgesChanged(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	front := lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))
	back := lineAlong(t, ex, api.V(0, 10, 10), api.V(20, 10, 10))
	f := d.Features.Fillet([]api.Entity{front}, 2, "")
	one := sound(t, f, "one edge")
	cut := 20 * (4 - math.Pi) // what a fillet of 2 along 20 takes away
	if !nearRel(one, 2000-cut, 1e-3) {
		t.Fatalf("one edge: volume %g, want %g", one, 2000-cut)
	}
	step(d)
	before := saved(t, d)
	f.AsFillet().SetEdges([]api.Entity{back, front}, nil)
	two := sound(t, f, "two edges")
	if !nearRel(two, 2000-2*cut, 1e-3) {
		t.Errorf("two edges: volume %g, want %g", two, 2000-2*cut)
	}
	if es := f.Edges(); len(es) != 2 || es[0].H != back.H {
		t.Errorf("edges %v, want the back first", es)
	}
	undoRedo(t, d, before)
	saved := roundTrip(t, d)
	if g := saved.Group(f.Name()); len(g.Edges()) != 2 || math.Abs(g.Analyze().Check().Volume-two) > 1e-6 {
		t.Errorf("after a round trip: %d edges, volume %g", len(g.Edges()), g.Analyze().Check().Volume)
	}
	// By a name on the solid before the fillet.
	right := ex.Solid().EdgeAt(api.V(20, 5, 10))
	f.AsFillet().SetEdges(nil, []api.SolidEdge{right})
	if v := sound(t, f, "a solid edge"); !nearRel(v, 2000-10*(4-math.Pi), 1e-3) {
		t.Errorf("the right edge: volume %g", v)
	}
	if n := f.SolidEdges(); len(n) != 1 || n[0].Name() != right.Name() {
		t.Errorf("solid edges %v", n)
	}
}

// An edge's own radius stays with its edge when the fillet's edges are
// changed: reordered, one dropped, one added.
func TestFilletEdgesCarryRadii(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	front := lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))
	back := lineAlong(t, ex, api.V(0, 10, 10), api.V(20, 10, 10))
	// The bottom front edge meets neither top edge, so no corner asks
	// the two radii to agree.
	left := lineAlong(t, ex, api.V(0, 0, 0), api.V(20, 0, 0))
	f := d.Features.Fillet([]api.Entity{front, back}, 1, "")
	f.AsFillet().SetEdgeRadius(1, 3)
	sound(t, f, "front 1, back 3")
	// The back first, the front dropped, the left added: the back keeps
	// 3 at its new place, the left takes the fillet's.
	f.AsFillet().SetEdges([]api.Entity{back, left}, nil)
	sound(t, f, "back 3, left 1")
	if r := f.AsFillet().EdgeRadius(0); r != 3 {
		t.Errorf("the back's radius moved: %g, want 3", r)
	}
	if r := f.AsFillet().EdgeRadius(1); r != 1 {
		t.Errorf("the left's radius: %g, want the fillet's 1", r)
	}
	// The back dropped: its radius goes with it.
	f.AsFillet().SetEdges([]api.Entity{left}, nil)
	if r := f.AsFillet().EdgeRadius(0); r != 1 {
		t.Errorf("the left's radius after the back went: %g", r)
	}
	d.With(func(sk *sketch.Sketch) {
		if n := len(sk.Group(f.H).EdgeRadii); n != 0 {
			t.Errorf("%d own radii left, want none", n)
		}
	})
	// An own radius too large for its edge is refused by the trial, and
	// nothing changes.
	f.AsFillet().SetEdgeRadius(0, 12)
	if p := f.Problem(); p == "" {
		t.Fatal("a radius of 12 on a block 10 wide should be a problem")
	}
	refusedWith(t, "", func() { f.AsFillet().SetEdges([]api.Entity{left, front}, nil) })
	if es := f.Edges(); len(es) != 1 {
		t.Errorf("after the refusal: %d edges, want 1", len(es))
	}
}

// A chamfer's edges changed, with two distances kept.
func TestChamferEdgesChanged(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	front := lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))
	back := lineAlong(t, ex, api.V(0, 10, 10), api.V(20, 10, 10))
	c := d.Features.Chamfer([]api.Entity{front}, 1, "")
	c.AsChamfer().SetSecondDistance(2)
	one := sound(t, c, "one edge")
	if math.Abs(one-(2000-20)) > 1e-6 {
		t.Fatalf("one edge: volume %g, want 1980", one)
	}
	step(d)
	before := saved(t, d)
	c.AsChamfer().SetEdges([]api.Entity{front, back}, nil)
	if v := sound(t, c, "two edges"); math.Abs(v-(2000-40)) > 1e-6 {
		t.Errorf("two edges: volume %g, want 1960", v)
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	// The same edges again change nothing, and leave no undo step for
	// a script.
	c.AsChamfer().SetEdges([]api.Entity{front, back}, nil)
	if v := c.Analyze().Check().Volume; math.Abs(v-1960) > 1e-6 {
		t.Errorf("the same edges: volume %g", v)
	}
}

// A shell's faces changed: open at the top, then at the top and one
// end.
func TestShellFacesChanged(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	top := ex.Solid().FaceAt(api.V(10, 5, 10))
	end := ex.Solid().FaceAt(api.V(20, 5, 5))
	sh := d.Features.ShellSolid([]api.SolidFace{top}, 1, "")
	open := sound(t, sh, "open at the top")
	wantTop := 2000.0 - 18*8*9
	if math.Abs(open-wantTop) > 1e-6 {
		t.Fatalf("open at the top: volume %g, want %g", open, wantTop)
	}
	step(d)
	before := saved(t, d)
	sh.AsShell().SetFaces([]api.SolidFace{top, end})
	wantTwo := 2000.0 - 19*8*9
	if v := sound(t, sh, "open at the top and the end"); math.Abs(v-wantTwo) > 1e-6 {
		t.Errorf("two faces: volume %g, want %g", v, wantTwo)
	}
	if fs := sh.AsShell().Faces(); len(fs) != 2 || fs[1].ID() != end.ID() {
		t.Errorf("faces %v", fs)
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	refusedWith(t, "no faces to take away", func() { sh.AsShell().SetFaces(nil) })
}

// A sweep's path changed from one line to a line and a bend.
func TestSweepPathChanged(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Features.SketchIn(d.Workplane("ZX"), "path")
	line := d.Sketch.Line(api.V(3, 0, 0), api.V(3, 0, 10))
	bend := d.Sketch.Arc(api.V(9, 0, 10), api.V(3, 0, 10), api.V(9, 0, 16))
	d.Features.SketchIn(d.Workplane("XY"), "profile")
	d.Sketch.Rect(api.V(1, -2, 0), api.V(5, 2, 0))
	sw := d.Features.Sweep([]api.Entity{line}, "")
	straight := sound(t, sw, "a line")
	if math.Abs(straight-160) > 1e-6 {
		t.Fatalf("along the line: volume %g, want 160", straight)
	}
	step(d)
	before := saved(t, d)
	sw.AsSweep().SetPath([]api.Entity{line, bend}, nil)
	bent := sound(t, sw, "a line and a bend")
	// The bend adds a quarter torus of the profile: Pappus, 16 by the
	// centroid's quarter turn at radius 6.
	if want := 160 + 16*math.Pi/2*6; math.Abs(bent-want) > 1e-3*want {
		t.Errorf("round the bend: volume %g, want %g", bent, want)
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	refusedWith(t, "no path", func() { sw.AsSweep().SetPath(nil, nil) })
}

// A pattern's path changed: three posts along a line, then along
// another twice as long.
func TestPatternPathChanged(t *testing.T) {
	t.Parallel()
	d := api.New()
	d.Sketch.Circle(api.V(-20, 0, 0), 2)
	short := d.Sketch.Line(api.V(-20, 20, 0), api.V(0, 20, 0))
	long := d.Sketch.Line(api.V(-20, 30, 0), api.V(20, 30, 0))
	short.SetConstruction(true)
	long.SetConstruction(true)
	d.Features.Extrude(10, "")
	p := d.Features.PatternAlongPath([]api.Entity{short}, 3, "")
	sound(t, p, "along the short line")
	step(d)
	before := saved(t, d)
	p.AsRepeat().SetPath([]api.Entity{long})
	r := p.Analyze().Check()
	if p.Problem() != "" || !r.Sound() || math.Abs(r.Max.X-22) > 1e-6 {
		t.Errorf("along the long line: problem %q, %s", p.Problem(), r.String())
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	refusedWith(t, "SetPath", func() { d.Features.Translate(2, api.V(1, 0, 0), "").AsRepeat().SetPath([]api.Entity{long}) })
}

// A loft's profiles changed: between two, then through three, the
// middle one's tangency kept when it stays.
func TestLoftProfilesChanged(t *testing.T) {
	t.Parallel()
	d := api.New()
	ps := ringOfCircles(d, 3, 20, 4)
	l := d.Features.LoftThrough(ps, "")
	sound(t, l, "three profiles")
	l.AsLoft().SetProfileTangency(ps[1], 1)
	three := sound(t, l, "held square to the middle")
	// The profiles reversed keep the middle and its tangency.
	l.AsLoft().SetProfiles([]api.Group{ps[2], ps[1], ps[0]})
	if v := sound(t, l, "reversed"); !nearRel(v, three, 1e-3) {
		t.Errorf("reversed: volume %g, want %g", v, three)
	}
	if tg := l.AsLoft().ProfileTangency(ps[1]); tg != 1 {
		t.Errorf("reversed, the middle's tangency %g, want 1", tg)
	}
	step(d)
	before := saved(t, d)
	l.AsLoft().SetProfiles([]api.Group{ps[0], ps[2]})
	if v := sound(t, l, "two profiles"); nearRel(v, three, 1e-6) {
		t.Errorf("two profiles: volume %g as three", v)
	}
	if n := len(l.AsLoft().Profiles()); n != 2 {
		t.Errorf("%d profiles, want 2", n)
	}
	undoRedo(t, d, before)
	d.With(func(sk *sketch.Sketch) { sk.ChordTolPct = 0 })
	roundTrip(t, d)
	refusedWith(t, "twice", func() { l.AsLoft().SetProfiles([]api.Group{ps[0], ps[0]}) })
	refusedWith(t, "two profiles at least", func() { l.AsLoft().SetProfiles([]api.Group{ps[0]}) })
	// Closed, the loft keeps three profiles at least.
	l.AsLoft().SetProfiles(ps)
	l.AsLoft().SetClosed(true)
	refusedWith(t, "three profiles", func() { l.AsLoft().SetProfiles([]api.Group{ps[0], ps[2]}) })
}

// A mirror's plane changed from one workplane to another.
func TestMirrorPlaneChanged(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 10, 5, 20, 15, 5)
	m := d.Features.Mirror(d.Workplane("YZ"), "")
	r := m.Analyze().Check()
	if !r.Sound() || math.Abs(r.Min.X+20) > 1e-6 {
		t.Fatalf("in YZ: %s", r.String())
	}
	step(d)
	before := saved(t, d)
	m.AsMirror().SetPlane(d.Workplane("ZX"))
	r = m.Analyze().Check()
	if m.Problem() != "" || !r.Sound() || math.Abs(r.Min.Y+15) > 1e-6 || math.Abs(r.Min.X-10) > 1e-6 {
		t.Errorf("in ZX: problem %q, %s", m.Problem(), r.String())
	}
	if m.AsMirror().Plane().H != d.Workplane("ZX").H {
		t.Error("the plane isn't ZX")
	}
	undoRedo(t, d, before)
	roundTrip(t, d)
	refusedWith(t, "a plane", func() { m.AsMirror().SetPlane(ex.Entities()[0]) })
	// A face of the mirror's own copies comes after it.
	refusedWith(t, "at or after", func() { m.AsMirror().SetPlane(m.Faces()[0]) })
}

// The wrong kind of group, an edge drawn after the feature, and an
// edge the kernel can't round: each refused, the feature unchanged.
func TestFeatureInputsMisuse(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	front := lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))
	f := d.Features.Fillet([]api.Entity{front}, 2, "")
	sound(t, f, "the fillet")
	refusedWith(t, "fillet", func() { ex.AsFillet().SetEdges([]api.Entity{front}, nil) })
	refusedWith(t, "no edges to fillet", func() { f.AsFillet().SetEdges(nil, nil) })
	// The rim where the fillet's own cylinder meets the top is made by
	// the fillet: after it.
	later := f.Solid().EdgeAt(api.V(10, 2, 10))
	refusedWith(t, "at or after", func() { f.AsFillet().SetEdges(nil, []api.SolidEdge{later}) })
	// A circle drawn after the fillet.
	d.Features.SketchIn(d.Workplane("XY"), "after")
	circle := d.Sketch.Circle(api.V(10, 5, 0), 3)
	refusedWith(t, "at or after", func() { f.AsFillet().SetEdges([]api.Entity{circle}, nil) })
	// A radius too large for the block: the trial refuses it.
	f.AsFillet().SetRadius(12)
	if f.Problem() == "" {
		t.Fatal("radius 12 on a block 10 wide should be a problem")
	}
	back := lineAlong(t, ex, api.V(0, 10, 10), api.V(20, 10, 10))
	refusedWith(t, "", func() { f.AsFillet().SetEdges([]api.Entity{front, back}, nil) })
	if es := f.Edges(); len(es) != 1 || es[0].H != front.H {
		t.Errorf("after the refusals: edges %v, want the front alone", es)
	}
}

// A later group built on an edge the fillet made loses it when the
// fillet's edges change: the chamfer reports the edge gone, and undo
// brings it back.
func TestFeatureInputsBreakLater(t *testing.T) {
	t.Parallel()
	d := api.New()
	ex := block(d, 0, 0, 20, 10, 10)
	front := lineAlong(t, ex, api.V(0, 0, 10), api.V(20, 0, 10))
	back := lineAlong(t, ex, api.V(0, 10, 10), api.V(20, 10, 10))
	f := d.Features.Fillet([]api.Entity{front}, 2, "")
	// The back top edge, by its name on the fillet's solid.
	rim := f.Solid().EdgeAt(api.V(10, 10, 10))
	c := d.Features.ChamferSolid([]api.SolidEdge{rim}, 0.5, "")
	sound(t, c, "the chamfer on the back edge")
	step(d)
	f.AsFillet().SetEdges([]api.Entity{back}, nil)
	sound(t, f, "the fillet moved to the back")
	// The fillet's cylinder now lies between the top and the back: the
	// edge the chamfer cuts is gone.
	if p := c.Problem(); p == "" {
		t.Error("the chamfer has no problem, though its edge is rounded away")
	}
	if !d.Edit.Undo() {
		t.Fatal("nothing to undo")
	}
	sound(t, c, "the chamfer after undo")
}
