package api

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// plate draws a plate whose width, height and hole come from three
// parameters, and repeats it n times; it returns the plate's sketch
// group, its extrusion and the step and repeat.
func plate(t *testing.T, d *Doc) (Group, Group, Group) {
	t.Helper()
	for _, p := range [][2]string{{"width", "40"}, {"height", "width/2"}, {"hole", "height/4"}, {"n", "2"}} {
		if err := d.SetParameter(p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	sides := d.Rect(V(0, 0, 0), V(30, 20, 0))
	for _, c := range []struct {
		c    Constraint
		expr string
	}{
		{d.Length(sides[0], 30), "width"},
		{d.Length(sides[1], 20), "height"},
	} {
		if err := c.c.SetExpr(c.expr); err != nil {
			t.Fatal(err)
		}
	}
	d.Coincident(sides[0].Start(), d.Origin())
	hole := d.Circle(V(10, 10, 0), 3)
	if err := d.Diameter(hole, 6).SetExpr("hole"); err != nil {
		t.Fatal(err)
	}
	d.Distance(hole.Center(), sides[0], 10)
	d.Distance(hole.Center(), sides[3], 10)
	sk := d.Active()
	ex := d.Extrude(5, "plate")
	rep := d.Translate(3, V(0, 0, 10), "stack")
	if err := rep.SetExpr(sketch.GroupCopies, "n+1"); err != nil {
		t.Fatal(err)
	}
	return sk, ex, rep
}

func TestParameters(t *testing.T) {
	d := New()
	sk, ex, rep := plate(t, d)
	check := func(width, height, hole float64, copies int) {
		t.Helper()
		lo, hi := ex.Bounds()
		if !near(hi.X-lo.X, width) || !near(hi.Y-lo.Y, height) {
			t.Errorf("plate is %g by %g, want %g by %g", hi.X-lo.X, hi.Y-lo.Y, width, height)
		}
		if got := rep.Value(sketch.GroupCopies); int(got) != copies {
			t.Errorf("%v copies, want %d", got, copies)
		}
		want := (width*height - math.Pi*hole*hole/4) * 5 * float64(copies)
		if v := rep.Volume(); math.Abs(v-want) > want*0.01 {
			t.Errorf("volume %g, want about %g", v, want)
		}
	}
	check(40, 20, 5, 3)

	if err := d.SetParameter("width", "60"); err != nil {
		t.Fatal(err)
	}
	check(60, 30, 7.5, 3)
	if p, ok := d.Parameter("hole"); !ok || p.Value != 7.5 || p.Expr != "height/4" || p.Err != "" {
		t.Errorf("hole is %+v", p)
	}
	if got := len(d.Parameters()); got != 4 {
		t.Errorf("%d parameters", got)
	}
	if rep.Expr(sketch.GroupCopies) != "n+1" || rep.Expr(sketch.GroupScale) != "" {
		t.Errorf("copies %q", rep.Expr(sketch.GroupCopies))
	}
	var hole Constraint
	for _, c := range sk.Constraints() {
		if c.Expr() == "hole" {
			hole = c
		}
	}
	if !hole.Valid() || !near(hole.Value(), 7.5) {
		t.Fatalf("hole dimension %v", hole)
	}

	// Rename and delete.
	if err := d.RenameParameter("n", "count"); err != nil {
		t.Fatal(err)
	}
	if got := rep.Expr(sketch.GroupCopies); got != "count+1" {
		t.Errorf("copies %q after the rename", got)
	}
	if got := strings.Join(d.ParameterUsers("height"), "; "); got != "parameter hole; constraint c00a-pt-pt-distance" {
		t.Errorf("users of height: %s", got)
	}
	if err := d.DeleteParameter("height"); err == nil {
		t.Error("a used parameter was deleted")
	}
	if err := d.SetParameter("spare", "1"); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteParameter("spare"); err != nil {
		t.Fatal(err)
	}

	// The table and the expressions come back from a file, and a save
	// of the reopened file is the same.
	path := filepath.Join(t.TempDir(), "plate.spcs")
	if err := d.Save(path); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"param width = 60\n", "param height = width/2\n", "value = hole\n", "copies = count+1\n"} {
		if !bytes.Contains(saved, []byte(want)) {
			t.Errorf("the file has no %q", want)
		}
	}
	e, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	again := filepath.Join(t.TempDir(), "again.spcs")
	if err := e.Save(again); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(again); !bytes.Equal(b, saved) {
		t.Error("a save of the reopened file differs")
	}
	if err := e.SetParameter("count", "0"); err != nil {
		t.Fatal(err)
	}
	if g := e.Group("stack"); int(g.Value(sketch.GroupCopies)) != 1 {
		t.Errorf("reopened copies %v", g.Value(sketch.GroupCopies))
	}

	// Undo takes a parameter change, one step of a script, back.
	d.h.(interface{ BeginEval() }).BeginEval()
	if err := d.SetParameter("width", "80"); err != nil {
		t.Fatal(err)
	}
	d.Undo()
	check(60, 30, 7.5, 3)
}

func TestParameterMisuse(t *testing.T) {
	d := New()
	for _, tc := range []struct{ name, expr, want string }{
		{"9", "1", "can't name"},
		{"a", "b", "unknown parameter"},
		{"a", "1/0", "not a number"},
	} {
		if err := d.SetParameter(tc.name, tc.expr); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("SetParameter(%q, %q) = %v", tc.name, tc.expr, err)
		}
	}
	if err := d.SetParameter("a", "1"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetParameter("b", "a"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetParameter("a", "b"); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("cycle gave %v", err)
	}
	if err := d.RenameParameter("zz", "y"); err == nil {
		t.Error("rename of a missing parameter")
	}
	if err := d.DeleteParameter("zz"); err == nil {
		t.Error("delete of a missing parameter")
	}
	if _, ok := d.Parameter("zz"); ok {
		t.Error("a missing parameter was found")
	}

	line := d.Line(V(0, 0, 0), V(10, 0, 0))
	length := d.Length(line, 10)
	if err := length.SetExpr("q*2"); err == nil {
		t.Error("an unknown name was taken")
	}
	if err := length.SetExpr("a*20"); err != nil {
		t.Fatal(err)
	}
	if !near(line.Length(), 20) || length.Expr() != "a*20" {
		t.Errorf("line %g, expr %q", line.Length(), length.Expr())
	}
	if err := length.SetExpr("2*"); err == nil {
		t.Error("a bad expression was taken")
	}
	if errs := d.ExprErrors(); len(errs) != 0 {
		t.Errorf("errors %v", errs)
	}
	d.With(func(sk *sketch.Sketch) {
		sk.GetConstraint(length.H).Expr = "gone"
		sk.MarkGroupDirty(sk.GetConstraint(length.H).Group, false)
	})
	if errs := d.ExprErrors(); len(errs) != 1 || !strings.Contains(errs[0], `unknown parameter "gone"`) {
		t.Errorf("errors %v", errs)
	}

	g := d.Active()
	for name, fn := range map[string]func(){
		"copies of a sketch": func() { _ = g.SetExpr(sketch.GroupCopies, "2") },
		"value of a horizontal": func() {
			_ = d.Horizontal(line).SetExpr("1")
		},
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s did not fail", name)
				}
			}()
			fn()
		}()
	}
	if err := g.SetExpr(sketch.GroupScale, "0"); err == nil {
		t.Error("a scale of 0 was taken")
	}
}

// Entities tell how much of them their constraints fix, as the
// drawing colours them.
func TestEntityFree(t *testing.T) {
	d := New()
	line := d.Line(V(0, 0, 0), V(10, 5, 0))
	if f := line.Free(); f != sketch.FreedomFree {
		t.Errorf("an unconstrained line is %v", f)
	}
	d.Coincident(line.Start(), d.Origin())
	if f := line.Free(); f != sketch.FreedomPartly {
		t.Errorf("a line held at one end is %v", f)
	}
	if f := line.Start().Free(); f != sketch.FreedomFixed {
		t.Errorf("the held end is %v", f)
	}
	d.Horizontal(line)
	d.Length(line, 10)
	if f := line.Free(); f != sketch.FreedomFixed {
		t.Errorf("a fully constrained line is %v", f)
	}
	if s := sketch.FreedomPartly.String(); s != "partly free" {
		t.Errorf("String %q", s)
	}
	if s := sketch.FreedomUnknown.String() + sketch.FreedomFixed.String() + sketch.FreedomFree.String(); s != "unknownfixedfree" {
		t.Errorf("String %q", s)
	}
	// Over-constrained: the group does not solve, so nothing is known.
	d.Length(line, 12)
	if f := line.Free(); f != sketch.FreedomUnknown {
		t.Errorf("a line of a group that did not solve is %v", f)
	}
}

// A group that does not solve names the constraints to blame.
func TestConflicts(t *testing.T) {
	d := New()
	line := d.Line(V(0, 0, 0), V(10, 5, 0))
	g := d.Active()
	d.Horizontal(line)
	if c := g.Conflicts(); len(c) != 0 {
		t.Errorf("a solved group has conflicts %v", c)
	}
	a := d.Length(line, 10)
	d.Vertical(line)
	b := d.Length(line, 12)
	got := g.Conflicts()
	if g.Solved() || len(got) == 0 {
		t.Fatalf("solved %v, conflicts %v", g.Solved(), got)
	}
	names := map[uint32]bool{}
	for _, c := range got {
		names[c.H.V] = true
	}
	if !names[a.H.V] && !names[b.H.V] {
		t.Errorf("conflicts %v name neither length", got)
	}
}

// Without a window there is no menu to run a command from.
func TestRunCommandWithoutWindow(t *testing.T) {
	d := New()
	if err := d.RunCommand("View > Zoom To Fit"); err == nil || !strings.Contains(err.Error(), "no window") {
		t.Errorf("RunCommand: %v", err)
	}
	saved := commandList
	t.Cleanup(func() { commandList = saved })
	SetCommandList(nil)
	if c := Commands(); c != nil {
		t.Errorf("commands %v with no interface", c)
	}
	SetCommandList(func() []string { return []string{"A > B"} })
	if c := Commands(); len(c) != 1 || c[0] != "A > B" {
		t.Errorf("commands %v", c)
	}
}
