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
	t.Parallel()
	d := New()
	sketch := d.Active()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	ex := d.Features.Extrude(5, "box")
	in3d := d.Features.Sketch3D("other")
	d.Sketch.Line(V(0, 0, 0), V(5, 5, 5))

	if got := names(ex.History().DependsOn()); got != sketch.String() {
		t.Fatalf("the extrusion depends on %q", got)
	}
	if got := names(sketch.History().UsedBy()); got != "g003-box" {
		t.Fatalf("the sketch is used by %q", got)
	}
	if got := in3d.History().DependsOn(); len(got) != 0 {
		t.Fatalf("the sketch in 3d depends on %v", names(got))
	}

	// Refused: nothing changes and there is no undo step.
	step := d.Host().(*SketchHost).BeginEval
	step()
	before := names(d.Groups())
	err := ex.History().MoveBefore(sketch)
	if err == nil || err.Error() != "g003-box can't move before g002-sketch-in-plane, which it uses (operand)" {
		t.Fatalf("moving the extrusion before its sketch: %v", err)
	}
	if err := sketch.History().MoveToEnd(); err == nil || !strings.Contains(err.Error(), "which uses it") {
		t.Fatalf("moving the sketch to the end: %v", err)
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("refused moves left %q", got)
	}

	step()
	if err := in3d.History().MoveBefore(sketch); err != nil {
		t.Fatal(err)
	}
	if got, want := names(d.Groups()), "g004-other g002-sketch-in-plane g003-box"; got != want {
		t.Fatalf("groups %q, want %q", got, want)
	}
	if v := ex.Analyze().Check().Volume; v < 499 || v > 501 {
		t.Fatalf("the box after the move has volume %v", v)
	}
	step()
	if err := ex.History().MoveBefore(sketch); err == nil {
		t.Fatal("moved the extrusion before its sketch")
	}
	if !d.Edit.Undo() {
		t.Fatal("no undo step")
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("undo left %q, want %q", got, before)
	}
	if !d.Edit.Redo() {
		t.Fatal("no redo step")
	}
	step()
	if err := in3d.History().MoveToEnd(); err != nil {
		t.Fatal(err)
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("moved to the end: %q, want %q", got, before)
	}
}

func TestMoveGroupMisuse(t *testing.T) {
	t.Parallel()
	d := New()
	g := d.Active()
	gone := d.Features.Sketch3D("gone")
	gone.Delete()
	for name, fn := range map[string]func(){
		"move before no group":     func() { _ = g.History().MoveBefore(Group{}) },
		"move a deleted group":     func() { _ = gone.History().MoveToEnd() },
		"move before deleted":      func() { _ = g.History().MoveBefore(gone) },
		"dependencies of deleted":  func() { gone.History().DependsOn() },
		"users of a deleted group": func() { gone.History().UsedBy() },
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
	t.Parallel()
	d := New()
	sketch := d.Active()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	ex := d.Features.Extrude(5, "box")
	d.Activate(sketch)
	d.Activate(ex)
	box := ex.Analyze().Check().Volume

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
	if len(ex.Entities()) != 0 || ex.Analyze().Check().Volume != 0 {
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
	if ex.LeftOut() != "" || ex.Analyze().Check().Volume != box {
		t.Fatal("the box didn't come back")
	}
	step()
	ex.SetSuppress(SuppressSolid)
	if ex.Suppression() != SuppressSolid || ex.LeftOut() != "" || ex.Analyze().Check().Volume != 0 {
		t.Fatal("suppressing the solid")
	}
	if !d.Edit.Undo() || ex.Suppression() != Unsuppressed {
		t.Fatal("undo")
	}
}

func TestSuppressMisuse(t *testing.T) {
	t.Parallel()
	d := New()
	sketch := d.Active()
	d.Sketch.Line(V(0, 0, 0), V(5, 0, 0))
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
	t.Parallel()
	d := New()
	sketch := d.Active()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	d.Features.Extrude(5, "box")
	d.Features.Translate(2, V(20, 0, 0), "copies")
	if refs := d.Features.BrokenReferences(); len(refs) != 0 {
		t.Fatalf("broken before anything was deleted: %v", refs)
	}

	step := d.Host().(*SketchHost).BeginEval
	step()
	sketch.Delete()
	refs := d.Features.BrokenReferences()
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

	d.Features.ClearBrokenReferences()
	if len(d.Features.BrokenReferences()) != 0 {
		t.Fatal("cleared, the list isn't empty")
	}
	if !d.Edit.Undo() || len(d.Groups()) != 3 {
		t.Fatalf("undo left %s", names(d.Groups()))
	}
}

// Deleting the active group, or a group it uses, leaves the last group
// before it that is left active, so new groups come at the end.
func TestDeleteActiveGroup(t *testing.T) {
	t.Parallel()
	d := New()
	first := d.Active()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	box := d.Features.Extrude(5, "box")
	second := d.Features.SketchIn(first.Workplane(), "second")
	d.Sketch.Circle(V(20, 20, 0), 3)
	d.Features.Extrude(5, "pin")

	// The pin goes with its sketch; the box is what is left before it.
	second.Delete()
	if d.Active().H != box.H {
		t.Fatalf("after deleting the active group's sketch, %v is active, want %v", d.Active(), box)
	}
	d.Features.Sketch3D("after")
	if got, want := names(d.Groups()), "g002-sketch-in-plane g003-box g004-after"; got != want {
		t.Fatalf("groups %q, want %q", got, want)
	}

	// Deleting the active group itself activates the one before.
	d.Active().Delete()
	if d.Active().H != box.H {
		t.Fatalf("after deleting the active group, %v is active, want %v", d.Active(), box)
	}
	// A group that isn't active and isn't used leaves the active one.
	other := d.Features.Sketch3D("other")
	d.Activate(box)
	other.Delete()
	if d.Active().H != box.H {
		t.Fatalf("deleting another group made %v active", d.Active())
	}
}

// An extrusion moves to the end with its own sketch; a sketch that
// another group uses too stays.
func TestMoveWithSources(t *testing.T) {
	t.Parallel()
	d := New()
	first := d.Active()
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	box := d.Features.Extrude(5, "box")
	pinSketch := d.Features.SketchIn(first.Workplane(), "pin-sketch")
	d.Sketch.Circle(V(20, 20, 0), 3)
	pin := d.Features.Extrude(5, "pin")
	in3d := d.Features.Sketch3D("free")
	d.Sketch.Line(V(0, 0, 0), V(5, 5, 5))

	if got := names(pin.History().OwnSources()); got != "g004-pin-sketch g005-pin" {
		t.Fatalf("the pin's own sources are %q", got)
	}
	// The first sketch's workplane is the references'; the box's
	// sketch is its own.
	if got := names(box.History().OwnSources()); got != "g002-sketch-in-plane g003-box" {
		t.Fatalf("the box's own sources are %q", got)
	}
	step := d.Host().(*SketchHost).BeginEval

	step()
	if err := in3d.History().MoveBefore(pinSketch); err != nil {
		t.Fatal(err)
	}
	before := names(d.Groups())
	if before != "g002-sketch-in-plane g003-box g006-free g004-pin-sketch g005-pin" {
		t.Fatalf("groups %q", before)
	}
	// Moving the pin alone leaves its sketch behind; with its sources
	// both go.
	step()
	moved, err := pin.History().MoveWithSourcesBefore(in3d)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(moved); got != "g004-pin-sketch g005-pin" {
		t.Fatalf("moved %q", got)
	}
	if got, want := names(d.Groups()), "g002-sketch-in-plane g003-box g004-pin-sketch g005-pin g006-free"; got != want {
		t.Fatalf("groups %q, want %q", got, want)
	}
	step()
	if _, err := pin.History().MoveToEndWithSources(); err != nil {
		t.Fatal(err)
	}
	if got := names(d.Groups()); got != before {
		t.Fatalf("groups %q, want %q", got, before)
	}
	if v := pin.Analyze().Check().Volume; v < 638 || v > 642 { // the box and the pin
		t.Fatalf("the pin after the move has volume %v", v)
	}
	if !d.Edit.Undo() || names(d.Groups()) != "g002-sketch-in-plane g003-box g004-pin-sketch g005-pin g006-free" {
		t.Fatalf("undo left %q", names(d.Groups()))
	}

	// Refused: a group after them uses one of them, or the place is
	// among them.
	step()
	var corner Entity
	for _, e := range pin.Entities() {
		if e.Kind() == "point" {
			corner = e
			break
		}
	}
	d.Features.SketchAt(corner, V(1, 0, 0), V(0, 1, 0), "on-pin")
	if _, err := pin.History().MoveToEndWithSources(); err == nil || !strings.Contains(err.Error(), "can't move after") {
		t.Fatalf("moving the pin after a sketch on its face: %v", err)
	}
	if _, err := pin.History().MoveWithSourcesBefore(pinSketch); err == nil || !strings.Contains(err.Error(), "moves with it") {
		t.Fatalf("moving the pin before its own sketch: %v", err)
	}
	for name, fn := range map[string]func(){
		"move with sources before no group": func() { _, _ = pin.History().MoveWithSourcesBefore(Group{}) },
		"own sources of no group":           func() { Group{d: d, H: in3d.H}.History().OwnSources(); Group{d: d}.History().OwnSources() },
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
