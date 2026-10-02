package api

import (
	"strings"
	"testing"
)

// names lists the groups' descriptions.
func names(groups []Group) string {
	var s []string
	for _, g := range groups {
		s = append(s, g.String())
	}
	return strings.Join(s, " ")
}

func TestMoveGroup(t *testing.T) {
	d := New()
	sketch := d.Active()
	d.Rect(V(0, 0, 0), V(10, 10, 0))
	ex := d.Extrude(5, "box")
	in3d := d.Sketch3D("other")
	d.Line(V(0, 0, 0), V(5, 5, 5))

	if got := names(ex.DependsOn()); got != sketch.String() {
		t.Fatalf("the extrusion depends on %q", got)
	}
	if got := names(sketch.UsedBy()); got != "g003-box" {
		t.Fatalf("the sketch is used by %q", got)
	}
	if got := in3d.DependsOn(); len(got) != 0 {
		t.Fatalf("the sketch in 3d depends on %v", names(got))
	}

	// Refused: nothing changes and there is no undo step.
	step := d.Host().(*SketchHost).BeginEval
	step()
	before := names(d.Groups())
	err := ex.MoveBefore(sketch)
	if err == nil || err.Error() != "g003-box can't move before g002-sketch-in-plane, which it uses (operand)" {
		t.Fatalf("moving the extrusion before its sketch: %v", err)
	}
	if err := sketch.MoveToEnd(); err == nil || !strings.Contains(err.Error(), "which uses it") {
		t.Fatalf("moving the sketch to the end: %v", err)
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("refused moves left %q", got)
	}

	step()
	if err := in3d.MoveBefore(sketch); err != nil {
		t.Fatal(err)
	}
	if got, want := names(d.Groups()), "g004-other g002-sketch-in-plane g003-box"; got != want {
		t.Fatalf("groups %q, want %q", got, want)
	}
	if v := ex.Check().Volume; v < 499 || v > 501 {
		t.Fatalf("the box after the move has volume %v", v)
	}
	step()
	if err := ex.MoveBefore(sketch); err == nil {
		t.Fatal("moved the extrusion before its sketch")
	}
	if !d.Undo() {
		t.Fatal("no undo step")
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("undo left %q, want %q", got, before)
	}
	if !d.Redo() {
		t.Fatal("no redo step")
	}
	step()
	if err := in3d.MoveToEnd(); err != nil {
		t.Fatal(err)
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("moved to the end: %q, want %q", got, before)
	}
}

func TestMoveGroupMisuse(t *testing.T) {
	d := New()
	g := d.Active()
	gone := d.Sketch3D("gone")
	gone.Delete()
	for name, fn := range map[string]func(){
		"move before no group":     func() { _ = g.MoveBefore(Group{}) },
		"move a deleted group":     func() { _ = gone.MoveToEnd() },
		"move before deleted":      func() { _ = g.MoveBefore(gone) },
		"dependencies of deleted":  func() { gone.DependsOn() },
		"users of a deleted group": func() { gone.UsedBy() },
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

func TestSuppressWhole(t *testing.T) {
	d := New()
	sketch := d.Active()
	d.Rect(V(0, 0, 0), V(10, 10, 0))
	ex := d.Extrude(5, "box")
	d.Activate(sketch)
	d.Activate(ex)
	box := ex.Check().Volume

	step := d.Host().(*SketchHost).BeginEval
	step()
	sketch.SetSuppress(SuppressWhole)
	if sketch.Suppression() != SuppressWhole || ex.Suppression() != Unsuppressed {
		t.Fatalf("modes %v, %v", sketch.Suppression(), ex.Suppression())
	}
	if got := sketch.LeftOut(); got != "suppressed" {
		t.Fatalf("the sketch is left out: %q", got)
	}
	if got := ex.LeftOut(); got != "uses g002-sketch-in-plane, which is suppressed" {
		t.Fatalf("the extrusion is left out: %q", got)
	}
	if len(ex.Entities()) != 0 || ex.Check().Volume != 0 {
		t.Fatal("the extrusion is still there")
	}
	if d.Active().H.V != 1 {
		t.Fatalf("active group %v, want the references: nothing else is in", d.Active())
	}

	// Back, with the extrusion made active again: the active group had
	// to move to the references.
	step()
	sketch.SetSuppress(Unsuppressed)
	d.Activate(ex)
	if ex.LeftOut() != "" || ex.Check().Volume != box {
		t.Fatal("the box didn't come back")
	}
	step()
	ex.SetSuppress(SuppressSolid)
	if ex.Suppression() != SuppressSolid || ex.LeftOut() != "" || ex.Check().Volume != 0 {
		t.Fatal("suppressing the solid")
	}
	if !d.Undo() || ex.Suppression() != Unsuppressed {
		t.Fatal("undo")
	}
}

func TestSuppressMisuse(t *testing.T) {
	d := New()
	sketch := d.Active()
	d.Line(V(0, 0, 0), V(5, 0, 0))
	sketch.SetSuppress(SuppressWhole)
	for name, fn := range map[string]func(){
		"activate a group left out": func() { d.Activate(sketch) },
		"suppress the references":   func() { d.Group("g001").SetSuppress(SuppressWhole) },
		"no such mode":              func() { sketch.SetSuppress(7) },
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

func TestBrokenReferences(t *testing.T) {
	d := New()
	sketch := d.Active()
	d.Rect(V(0, 0, 0), V(10, 10, 0))
	d.Extrude(5, "box")
	d.Translate(2, V(20, 0, 0), "copies")
	if refs := d.BrokenReferences(); len(refs) != 0 {
		t.Fatalf("broken before anything was deleted: %v", refs)
	}

	step := d.Host().(*SketchHost).BeginEval
	step()
	sketch.Delete()
	refs := d.BrokenReferences()
	var lines []string
	for _, r := range refs {
		lines = append(lines, r.Text)
	}
	want := "deleted g003-box, which used g002-sketch-in-plane\n" +
		"deleted g004-copies, which used g003-box"
	if got := strings.Join(lines, "\n"); got != want {
		t.Fatalf("broken:\n%s\nwant:\n%s", got, want)
	}
	if refs[0].Item != "g003-box" || refs[0].Missing != "g002-sketch-in-plane" || refs[0].Group.H.V != 3 {
		t.Fatalf("%+v", refs[0])
	}
	if refs[0].Group.String() != "deleted group g003" {
		t.Fatalf("the deleted group is %q", refs[0].Group.String())
	}

	d.ClearBrokenReferences()
	if len(d.BrokenReferences()) != 0 {
		t.Fatal("cleared, the list isn't empty")
	}
	if !d.Undo() || len(d.Groups()) != 3 {
		t.Fatalf("undo left %s", names(d.Groups()))
	}
}
