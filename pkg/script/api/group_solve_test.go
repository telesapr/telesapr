package api

import (
	"math"
	"strings"
	"testing"
)

// fixedRect draws a 20 by 10 rectangle, holds it where it is drawn
// with its two lengths, and returns the sketch group and its four
// lines.
func fixedRect(d *Doc) (Group, []Entity) {
	lines := d.Sketch.Rect(V(0, 0, 0), V(20, 10, 0))
	d.Constrain.Horizontal(lines[0])
	d.Constrain.Horizontal(lines[2])
	d.Constrain.Vertical(lines[1])
	d.Constrain.Vertical(lines[3])
	d.Constrain.Lock(lines[0].Start())
	d.Constrain.Length(lines[0], 20)
	d.Constrain.Length(lines[1], 10)
	return d.Active(), lines
}

// Every solve option of a group is what the window's group screen
// sets, and is read back as it was left.
func TestGroupSolveOptions(t *testing.T) {
	d := New()
	g, _ := fixedRect(d)
	for _, c := range []struct {
		name string
		get  func() bool
		set  func(bool)
	}{
		{"relax constraints", g.RelaxConstraints, g.SetRelaxConstraints},
		{"allow redundant", g.AllowRedundant, g.SetAllowRedundant},
		{"suppress dof", g.SuppressDof, g.SetSuppressDof},
		{"all dims reference", g.AllDimsReference, g.SetAllDimsReference},
		{"force to mesh", g.ForceToMesh, g.SetForceToMesh},
	} {
		if c.get() {
			t.Errorf("%s is on in a new group", c.name)
		}
		c.set(true)
		if !c.get() {
			t.Errorf("%s did not take", c.name)
		}
		c.set(false)
		if c.get() {
			t.Errorf("%s did not come back off", c.name)
		}
	}
	// Each of them is a step of its own, undone as it was made.
	step := d.Host().(*SketchHost).BeginEval
	step()
	g.SetRelaxConstraints(true)
	if !d.Edit.Undo() || g.RelaxConstraints() {
		t.Error("undo left the option on")
	}
}

// A group with a constraint more than its sketch can use solves where
// the redundancy is allowed, and reports the surplus to remove where it
// is not.
func TestGroupAllowRedundant(t *testing.T) {
	d := New()
	g, lines := fixedRect(d)
	// The rectangle's diagonal is already known from its two sides, so
	// holding it as well is one constraint too many.
	// A script makes it a reference as the window does, so it is made a
	// driving dimension again to be too many.
	d.Constrain.Distance(lines[0].Start(), lines[1].End(), math.Hypot(20, 10)).SetReference(false)
	if g.Solved() {
		t.Fatal("a redundant constraint solves")
	}
	if n := len(g.Conflicts()); n == 0 {
		t.Error("no constraint reported to remove")
	}
	g.SetAllowRedundant(true)
	if !g.Solved() {
		t.Errorf("the group does not solve with the redundancy allowed: %s", g.Problem())
	}
	g.SetAllowRedundant(false)
	if g.Solved() {
		t.Error("the group still solves with the redundancy not allowed")
	}
}

// A relaxed group drops the constraint it cannot use and solves, where
// the group that requires it does not.
func TestGroupRelaxConstraints(t *testing.T) {
	d := New()
	g, lines := fixedRect(d)
	d.Constrain.Distance(lines[0].Start(), lines[1].End(), math.Hypot(20, 10))
	g.SetRelaxConstraints(true)
	if !g.Solved() {
		t.Errorf("the relaxed group does not solve: %s", g.Problem())
	}
}

// With every dimension a reference dimension, the dimension measures
// the line without holding it, so the end of the line goes where it is
// put.
func TestGroupAllDimsReference(t *testing.T) {
	d := New()
	line := d.Sketch.Line(V(0, 0, 0), V(30, 0, 0))
	d.Constrain.Lock(line.Start())
	d.Constrain.Horizontal(line)
	d.Constrain.Length(line, 30)
	g := d.Active()
	if got := line.End().Pos().X; !near(got, 30) {
		t.Fatalf("the dimension does not hold the line: its end is at %v", got)
	}
	g.SetAllDimsReference(true)
	line.End().MoveTo(V(50, 0, 0))
	if got := line.End().Pos().X; !near(got, 50) {
		t.Errorf("the dimension still holds the line: its end is at %v", got)
	}
	g.SetAllDimsReference(false)
	// The dimension took the length the line had when it was a
	// reference one, so it holds the line at that now.
	if got := line.End().Pos().X; !near(got, 50) {
		t.Errorf("with the dimension holding again, the line's end is at %v", got)
	}
	line.End().MoveTo(V(60, 0, 0))
	if got := line.End().Pos().X; !near(got, 50) {
		t.Errorf("the dimension does not hold the line at 50: its end is at %v", got)
	}
}

// A group left out of the freedom count is described as such, so the
// count it does not give is not read as a sketch held.
func TestGroupSuppressDof(t *testing.T) {
	d := New()
	g, _ := fixedRect(d)
	if got := g.Dof(); got != 0 {
		t.Errorf("a fully constrained sketch has the freedom %d", got)
	}
	g.SetSuppressDof(true)
	if got := g.Dof(); got >= 0 {
		t.Errorf("a group left out of the count has the freedom %d", got)
	}
	if s := g.Describe(); !strings.Contains(s, "DOF not counted") {
		t.Errorf("a group left out of the count describes itself as %q", s)
	}
	g.SetSuppressDof(false)
	if s := g.Describe(); strings.Contains(s, "DOF not counted") {
		t.Errorf("the group still describes itself as %q", s)
	}
}
