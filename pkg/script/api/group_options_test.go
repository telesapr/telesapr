package api

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A pattern along a path, an extrusion and a step and repeat of it, made
// for the tests of the group options.
func optionsDoc(t *testing.T) (d *Doc, post, rep, pat Group) {
	t.Helper()
	d = New()
	d.Sketch.Circle(V(-20, 0, 0), 2)
	d.Sketch.Line(V(-20, 20, 0), V(20, 20, 0)).SetConstruction(true)
	post = d.Features.Extrude(80, "")
	var path Entity
	for _, e := range post.Entities() {
		if e.Kind() == "line" && e.Start().Pos().Z == 80 && e.End().Pos().Z == 80 {
			path = e
		}
	}
	pat = d.Features.PatternAlongPath([]Entity{path}, 3, "")
	d.Activate(post)
	rep = d.Features.Translate(2, V(10, 0, 0), "")
	return d, post, rep, pat
}

// Skewed keeps the one-sided or two-sided mode, and the other way.
func TestExtrudeSkewed(t *testing.T) {
	t.Parallel()
	d := New()
	d.Sketch.Rect(V(0, 0, 0), V(2, 2, 0))
	g := d.Features.Extrude(2, "")
	ex := g.AsExtrude()
	sk := d.Host().Sketch()
	if ex.Skewed() {
		t.Fatal("a new extrusion is skewed")
	}
	ex.SetSkewed(true)
	if !ex.Skewed() || sk.Group(g.H).Subtype != sketch.GroupOneSkewed {
		t.Errorf("subtype %d after SetSkewed(true)", sk.Group(g.H).Subtype)
	}
	g.SetTwoSided(true)
	if !ex.Skewed() || sk.Group(g.H).Subtype != sketch.GroupTwoSkewed {
		t.Errorf("subtype %d after SetTwoSided keeps the skew", sk.Group(g.H).Subtype)
	}
	step := d.Host().(*SketchHost).BeginEval
	step()
	ex.SetSkewed(false)
	if ex.Skewed() || sk.Group(g.H).Subtype != sketch.GroupTwoSided || !g.TwoSided() {
		t.Errorf("subtype %d after SetSkewed(false)", sk.Group(g.H).Subtype)
	}
	if !d.Edit.Undo() || d.Host().Sketch().Group(g.H).Subtype != sketch.GroupTwoSkewed {
		t.Errorf("undo of SetSkewed left subtype %d", d.Host().Sketch().Group(g.H).Subtype)
	}
}

// Skewed is for an extrusion.
func TestSkewedWrongKind(t *testing.T) {
	t.Parallel()
	_, _, rep, _ := optionsDoc(t)
	failsWith(t, "extrude", func() { rep.AsExtrude().SetSkewed(true) })
}

// SkipFirst is the group screen's "start with copy #1", for a one-sided
// step and repeat, and one undo step.
func TestRepeatSkipFirst(t *testing.T) {
	t.Parallel()
	d, _, rg, pat := optionsDoc(t)
	rep := rg.AsRepeat()
	if rep.SkipFirst() {
		t.Fatal("a new repeat skips its first")
	}
	step := d.Host().(*SketchHost).BeginEval
	step()
	rep.SetSkipFirst(true)
	if !rep.SkipFirst() {
		t.Error("SetSkipFirst(true) didn't")
	}
	if !d.Edit.Undo() || rep.SkipFirst() {
		t.Error("undo didn't put the original back")
	}
	rg.SetTwoSided(true)
	failsWith(t, "two-sided", func() { rep.SetSkipFirst(true) })
	failsWith(t, "step and repeat", func() { pat.AsRepeat().SetSkipFirst(true) })
	failsWith(t, "step and repeat", func() { pat.AsRepeat().SkipFirst() })
}

// Opacity and colour keep each other, validate, and are refused by a
// group that has no colour of its own.
func TestGroupOpacityAndColor(t *testing.T) {
	t.Parallel()
	d, post, rep, _ := optionsDoc(t)
	if post.Opacity() != 1 {
		t.Errorf("a new solid is %g opaque", post.Opacity())
	}
	post.SetOpacity(0.5)
	step := d.Host().(*SketchHost).BeginEval
	step()
	if got := post.Opacity(); math.Abs(got-0.5) > 0.01 {
		t.Errorf("opacity %g after 0.5", got)
	}
	step()
	post.SetColor(10, 20, 30)
	if r, g, b := post.Color(); r != 10 || g != 20 || b != 30 {
		t.Errorf("colour %d %d %d", r, g, b)
	}
	if got := post.Opacity(); math.Abs(got-0.5) > 0.01 {
		t.Errorf("SetColor changed the opacity to %g", got)
	}
	if !d.Edit.Undo() {
		t.Fatal("undo")
	}
	if r, _, _ := post.Color(); r == 10 {
		t.Error("undo left the colour")
	}
	for _, bad := range []float64{-0.1, 1.1, math.NaN()} {
		failsWith(t, "between zero and one", func() { post.SetOpacity(bad) })
	}
	failsWith(t, "0 to 255", func() { post.SetColor(0, 0, 256) })
	failsWith(t, "no colour of its own", func() { rep.SetColor(1, 2, 3) })
	failsWith(t, "no colour of its own", func() { rep.SetOpacity(0.5) })
}

// A group takes the options its kind of solid has and refuses the
// others, as the group screen shows only those.
func TestGroupOptionsByKind(t *testing.T) {
	t.Parallel()
	_, post, rep, pat := optionsDoc(t)

	post.SetCombine(sketch.CombineAssemble)
	if post.Combine() != sketch.CombineAssemble {
		t.Errorf("combine %d", post.Combine())
	}
	failsWith(t, "no combine mode of its own", func() { rep.SetCombine(sketch.CombineDifference) })
	failsWith(t, "no combine mode of its own", func() { pat.SetCombine(sketch.CombineDifference) })
	if rep.Combine() != post.Combine() {
		t.Errorf("a repeat has combine %d, its source's is %d", rep.Combine(), post.Combine())
	}

	rep.SetAppearance("metal")
	if rep.Appearance() != "metal" {
		t.Errorf("appearance %q", rep.Appearance())
	}
	rep.SetSuppress(SuppressSolid)
	if rep.Suppression() != SuppressSolid {
		t.Error("a repeat can't leave its solid out")
	}
}

// A sketch and the groups that edit the solid so far have no solid of
// their own to leave out, and a sketch gives no appearance.
func TestGroupOptionsNoSolid(t *testing.T) {
	t.Parallel()
	d := New()
	sketchGroup := d.Active()
	d.Sketch.Rect(V(0, 0, 0), V(2, 2, 0))
	failsWith(t, "no solid of its own", func() { sketchGroup.SetSuppress(SuppressSolid) })
	failsWith(t, "makes no solid", func() { sketchGroup.SetAppearance("matte") })
	sketchGroup.SetSuppress(Unsuppressed)
}

// The kinds of group and their options: what the screen offers and
// what a script may set are the one table.
func TestGroupOptionKinds(t *testing.T) {
	t.Parallel()
	type opts struct{ combine, color, appearance, suppress bool }
	for typ, want := range map[sketch.GroupType]opts{
		sketch.GroupDrawing3D:        {false, false, false, false},
		sketch.GroupDrawingWorkplane: {false, false, false, false},
		sketch.GroupExtrude:          {true, true, true, true},
		sketch.GroupLathe:            {true, true, true, true},
		sketch.GroupRevolve:          {true, true, true, true},
		sketch.GroupHelix:            {true, true, true, true},
		sketch.GroupSweep:            {true, true, true, true},
		sketch.GroupLoft:             {true, true, true, true},
		sketch.GroupLinked:           {true, false, true, true},
		sketch.GroupTranslate:        {false, false, true, true},
		sketch.GroupRotate:           {false, false, true, true},
		sketch.GroupMirror:           {false, false, true, true},
		sketch.GroupPathPattern:      {false, false, true, true},
		sketch.GroupChamfer:          {false, false, true, true},
		sketch.GroupFillet:           {false, false, true, false},
		sketch.GroupShell:            {false, false, true, false},
	} {
		g := sketch.Group{Type: typ}
		if got := (opts{g.HasCombine(), g.HasColor(), g.HasAppearance(), g.HasSolidSuppress()}); got != want {
			t.Errorf("group type %d: options %+v, want %+v", typ, got, want)
		}
	}
}

// A pattern's copies can be an expression, as the group screen lets
// them be.
func TestPatternCopiesExpression(t *testing.T) {
	t.Parallel()
	_, _, _, pat := optionsDoc(t)
	if err := pat.SetExpr(sketch.GroupCopies, "2+2"); err != nil {
		t.Fatal(err)
	}
	if n := pat.AsRepeat().Copies(); n != 4 {
		t.Errorf("%d copies, want 4", n)
	}
	if e := pat.Expr(sketch.GroupCopies); e != "2+2" {
		t.Errorf("expression %q", e)
	}
}
