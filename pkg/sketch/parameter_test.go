package sketch

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// lineSketch is a new file with one line segment in the drawing
// group, from (0,0) to (5,1), and a distance between its ends.
func lineSketch(t *testing.T) (*Sketch, core.HRequest, core.HConstraint) {
	t.Helper()
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	hg := sk.GroupOrder[len(sk.GroupOrder)-1]
	sk.Active = hg
	hr := sk.AddRequest(Request{Type: RequestLineSegment, Group: hg, Workplane: hg.Entity(0)})
	sk.GenerateAll()
	sk.MustGetEntity(hr.Entity(1)).PointForceTo(sk, geom.Vector{})
	sk.MustGetEntity(hr.Entity(2)).PointForceTo(sk, geom.Vector{X: 5, Y: 1})
	hc := sk.AddConstraint(core.Constraint{
		Type: core.ConstraintPtPtDistance, Group: hg, Workplane: hg.Entity(0),
		PtA: hr.Entity(1), PtB: hr.Entity(2), ValA: 10})
	sk.GenerateAll()
	return sk, hr, hc
}

func lineLength(sk *Sketch, hr core.HRequest) float64 {
	a := sk.MustGetEntity(hr.Entity(1)).PointGetNum(sk)
	b := sk.MustGetEntity(hr.Entity(2)).PointGetNum(sk)
	return a.Minus(b).Magnitude()
}

func TestParameterDrivesDimension(t *testing.T) {
	sk, hr, hc := lineSketch(t)
	if got := lineLength(sk, hr); math.Abs(got-10) > 1e-9 {
		t.Fatalf("line is %v long before any parameter", got)
	}
	for _, p := range [][2]string{{"wall", "2.5"}, {"len", "wall*4 + 1"}} {
		if err := sk.AddParameter(p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := sk.SetConstraintText(hc, "len"); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if got := lineLength(sk, hr); math.Abs(got-11) > 1e-9 {
		t.Fatalf("line is %v long, want 11 from len = wall*4+1", got)
	}
	if c := sk.GetConstraint(hc); c.Expr != "len" || c.ValA != 11 {
		t.Fatalf("constraint holds %q = %v", c.Expr, c.ValA)
	}

	// A change of the parameter regenerates the dimension.
	if err := sk.SetParameterExpr("wall", "3"); err != nil {
		t.Fatal(err)
	}
	if !sk.GetGroup(sk.GetConstraint(hc).Group).Clean == false {
		t.Fatal("the dimension's group should be dirty after the parameter changed")
	}
	sk.GenerateAll()
	if got := lineLength(sk, hr); math.Abs(got-13) > 1e-9 {
		t.Fatalf("line is %v long, want 13", got)
	}

	// Renaming rewrites the users.
	if err := sk.RenameParameter("wall", "t"); err != nil {
		t.Fatal(err)
	}
	if got := sk.Parameter("len").Expr; got != "t*4 + 1" {
		t.Fatalf("len = %q after the rename", got)
	}
	if err := sk.RenameParameter("len", "length"); err != nil {
		t.Fatal(err)
	}
	if got := sk.GetConstraint(hc).Expr; got != "length" {
		t.Fatalf("constraint expression %q after the rename", got)
	}

	// A plain number drops the expression.
	if err := sk.SetConstraintText(hc, "20"); err != nil {
		t.Fatal(err)
	}
	if c := sk.GetConstraint(hc); c.Expr != "" || c.ValA != 20 {
		t.Fatalf("constraint holds %q = %v after a number", c.Expr, c.ValA)
	}
	// Any other text is kept, in the document's unit.
	sk.Units = core.UnitInches
	if err := sk.SetConstraintText(hc, "1/2"); err != nil {
		t.Fatal(err)
	}
	if c := sk.GetConstraint(hc); c.Expr != "1/2" || c.ValA != 12.7 {
		t.Fatalf("constraint holds %q = %v after 1/2 inch", c.Expr, c.ValA)
	}
	// A plain number is in the document's unit too.
	if err := sk.SetConstraintText(hc, "2"); err != nil {
		t.Fatal(err)
	}
	if c := sk.GetConstraint(hc); c.Expr != "" || c.ValA != 50.8 {
		t.Fatalf("constraint holds %q = %v after 2 inches", c.Expr, c.ValA)
	}
}

// A change of unit keeps every size: plain values are millimetres
// already, and length expressions get the factor, which a change back
// takes off again. Angles, counts and scales are not lengths.
func TestSetUnits(t *testing.T) {
	sk, hr, hc := lineSketch(t)
	sk.Units = core.UnitInches
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(sk.AddParameter("w", "0.25"))
	must(sk.AddParameter("k", "w*2"))
	must(sk.AddParameter("n", "3"))
	must(sk.SetConstraintText(hc, "k"))
	sk.GenerateAll()
	if got := lineLength(sk, hr); math.Abs(got-12.7) > 1e-9 {
		t.Fatalf("line %v, want 12.7", got)
	}
	hg := sk.AddGroup(Group{Type: GroupDrawing3D, Order: 9, ValA: 3, Scale: 1, Name: "r"})
	sk.GenerateAll()
	must(sk.SetGroupText(hg, GroupPitch, "w+1"))
	must(sk.SetGroupText(hg, GroupCopies, "n"))
	angle := sk.AddConstraint(core.Constraint{
		Type: core.ConstraintAngle, Group: sk.GetConstraint(hc).Group, ValA: 30, Expr: "n*10"})

	check := sk.SetUnits(core.UnitMM)
	if strings.Join(check, ",") != "w,k" {
		t.Errorf("parameters to check %v", check)
	}
	g := sk.GetGroup(hg)
	if c := sk.GetConstraint(hc); c.Expr != "(k)*25.4" || g.ExprB != "(w+1)*25.4" || g.ExprA != "n" ||
		sk.GetConstraint(angle).Expr != "n*10" {
		t.Fatalf("after the change: %q %q %q %q", c.Expr, g.ExprB, g.ExprA, sk.GetConstraint(angle).Expr)
	}
	// The angle constrains nothing, so it can't regenerate.
	sk.Constraints = sk.Constraints[:len(sk.Constraints)-1]
	sk.GenerateAll()
	if got := lineLength(sk, hr); math.Abs(got-12.7) > 1e-9 {
		t.Fatalf("line %v after the change, want 12.7", got)
	}
	if g := sk.GetGroup(hg); math.Abs(g.ValB-31.75) > 1e-9 || g.ValA != 3 {
		t.Fatalf("pitch %v copies %v", g.ValB, g.ValA)
	}

	// Back to inches: the factor comes off.
	sk.SetUnits(core.UnitInches)
	if c := sk.GetConstraint(hc); c.Expr != "k" || sk.GetGroup(hg).ExprB != "w+1" {
		t.Fatalf("back: %q %q", c.Expr, sk.GetGroup(hg).ExprB)
	}
	// To metres from inches: a factor below 1 divides.
	sk.SetUnits(core.UnitMeters)
	if c := sk.GetConstraint(hc); c.Expr != "(k)/39.37007874015748" {
		t.Fatalf("to metres: %q", c.Expr)
	}
	if sk.SetUnits(core.UnitMeters) != nil {
		t.Error("no change reported parameters")
	}
	// Feet and inches count in inches, so no factor.
	sk.SetUnits(core.UnitInches)
	sk.SetUnits(core.UnitFeetInches)
	if c := sk.GetConstraint(hc); c.Expr != "k" {
		t.Fatalf("to feet and inches: %q", c.Expr)
	}
	if got := ConvertLengthExpr("(a)*2+(b)*2", 2, 1); got != "((a)*2+(b)*2)*2" {
		t.Errorf("unbalanced unwrap: %q", got)
	}
	if got := ConvertLengthExpr("(a)*2+(b)*2", 1, 2); got != "((a)*2+(b)*2)/2" {
		t.Errorf("no unwrap: %q", got)
	}
}

func TestParameterTableErrors(t *testing.T) {
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	for _, tc := range []struct{ name, expr, want string }{
		{"2x", "1", "can't name"},
		{"pi", "1", "can't name"},
		{"a", "b", "unknown parameter \"b\""},
		{"a", "1+", "not a valid"},
		{"a", "1/0", "not a number"},
	} {
		if err := sk.AddParameter(tc.name, tc.expr); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("AddParameter(%q, %q) = %v, want %q", tc.name, tc.expr, err, tc.want)
		}
	}
	if len(sk.Parameters) != 0 {
		t.Fatalf("refused parameters were kept: %v", sk.Parameters)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(sk.AddParameter("a", "1"))
	must(sk.AddParameter("b", "a+1"))
	must(sk.AddParameter("c", "b*2"))
	if err := sk.AddParameter("a", "2"); err == nil {
		t.Error("a second parameter a was accepted")
	}
	// A cycle is refused and the table left as it was.
	err := sk.SetParameterExpr("a", "c")
	if err == nil || !strings.Contains(err.Error(), "cycle: a -> c -> b -> a") {
		t.Fatalf("cycle gave %v", err)
	}
	if p := sk.Parameter("a"); p.Expr != "1" || p.Value != 1 || p.Err != "" {
		t.Fatalf("a after the refused cycle: %+v", *p)
	}
	if got := sk.Parameter("c").Value; got != 4 {
		t.Fatalf("c = %v", got)
	}
	if err := sk.SetParameterExpr("nope", "1"); err == nil {
		t.Error("set of a missing parameter succeeded")
	}
	// Delete refuses a parameter in use.
	err = sk.DeleteParameter("a")
	if err == nil || !strings.Contains(err.Error(), "parameter b") {
		t.Fatalf("delete of a used parameter gave %v", err)
	}
	must(sk.DeleteParameter("c"))
	must(sk.DeleteParameter("b"))
	must(sk.DeleteParameter("a"))
	if err := sk.DeleteParameter("a"); err == nil {
		t.Error("second delete succeeded")
	}
	// Rename checks.
	must(sk.AddParameter("x", "1"))
	must(sk.AddParameter("y", "x"))
	for _, tc := range [][2]string{{"zz", "q"}, {"x", "y"}, {"x", "sin"}} {
		if err := sk.RenameParameter(tc[0], tc[1]); err == nil {
			t.Errorf("RenameParameter(%q, %q) succeeded", tc[0], tc[1])
		}
	}
	must(sk.RenameParameter("x", "x"))
}

// A table edited by hand, as a file can hold it: cycles, unknown
// names and duplicates are reported, and every value keeps its last
// good value.
func TestEvaluateBrokenTable(t *testing.T) {
	sk := NewSketch()
	sk.Parameters = []Parameter{
		{Name: "a", Expr: "b", Value: 7},
		{Name: "b", Expr: "a", Value: 8},
		{Name: "c", Expr: "a+1"},
		{Name: "d", Expr: "zz", Value: 3},
		{Name: "d", Expr: "1"},
		{Name: "f", Expr: "(", Value: 5},
		{Name: "g", Expr: "-1", Value: 0},
	}
	sk.EvaluateParameters()
	want := []struct {
		v   float64
		err string
	}{
		{7, "cycle: a -> b -> a"},
		{8, "cycle: a -> b -> a"},
		{8, ""},
		{3, "unknown parameter \"zz\""},
		{0, "a second parameter \"d\""},
		{5, "expr:"},
		{-1, ""},
	}
	for i, w := range want {
		p := sk.Parameters[i]
		if p.Value != w.v || (w.err == "") != (p.Err == "") || !strings.Contains(p.Err, w.err) {
			t.Errorf("%s: value %v err %q, want %v %q", p.Name, p.Value, p.Err, w.v, w.err)
		}
	}
}

func TestStoredExpressionErrors(t *testing.T) {
	sk, _, hc := lineSketch(t)
	if err := sk.AddParameter("w", "4"); err != nil {
		t.Fatal(err)
	}
	if err := sk.SetConstraintText(hc, "w*3"); err != nil {
		t.Fatal(err)
	}
	if err := sk.SetConstraintText(hc, "nope*3"); err == nil || !strings.Contains(err.Error(), "unknown parameter") {
		t.Fatalf("unknown name gave %v", err)
	}
	if err := sk.SetConstraintText(hc, "w/0"); err == nil {
		t.Fatal("a value that is not a number was accepted")
	}
	if c := sk.GetConstraint(hc); c.Expr != "w*3" || c.ValA != 12 {
		t.Fatalf("refused text changed the constraint: %q = %v", c.Expr, c.ValA)
	}
	if err := sk.SetConstraintText(core.HConstraint{V: 0x999}, "1"); err == nil {
		t.Fatal("missing constraint accepted")
	}

	// A file edited by hand: the expression keeps the value it had.
	sk.GetConstraint(hc).Expr = "q+1"
	sk.GenerateAll()
	if len(sk.ExprErrors) != 1 || !strings.Contains(sk.ExprErrors[0].Error(), "unknown parameter \"q\"") {
		t.Fatalf("ExprErrors = %v", sk.ExprErrors)
	}
	if got := sk.ExprErrors[0].Error(); !strings.HasPrefix(got, "constraint c001 = q+1") {
		t.Fatalf("error reads %q", got)
	}
	if c := sk.GetConstraint(hc); c.ValA != 12 {
		t.Fatalf("broken expression changed the value to %v", c.ValA)
	}
	// A reference dimension ignores its expression.
	sk.GetConstraint(hc).Reference = true
	sk.GenerateAll()
	if len(sk.ExprErrors) != 0 {
		t.Fatalf("reference dimension evaluated: %v", sk.ExprErrors)
	}
}

func TestSignedAndRadiusText(t *testing.T) {
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	if err := sk.AddParameter("d", "6"); err != nil {
		t.Fatal(err)
	}
	hg := sk.GroupOrder[1]
	add := func(typ core.ConstraintType, v float64, other bool) *core.Constraint {
		hc := sk.AddConstraint(core.Constraint{
			Type: typ, Group: hg, ValA: v, Other: other})
		return sk.GetConstraint(hc)
	}

	far := add(core.ConstraintPtLineDistance, -3, false)
	if err := sk.SetConstraintText(far.H, "d"); err != nil {
		t.Fatal(err)
	}
	far = sk.GetConstraint(far.H)
	if far.Expr != "-(d)" || far.ValA != -6 {
		t.Fatalf("far side: %q = %v", far.Expr, far.ValA)
	}
	if got := ConstraintText(far); got != "d" {
		t.Fatalf("far side shows %q", got)
	}
	far.Expr = "d-10"
	if got := ConstraintText(far); got != "-(d-10)" {
		t.Fatalf("far side shows %q", got)
	}

	radius := add(core.ConstraintDiameter, 4, true)
	if err := sk.SetConstraintText(radius.H, "d/2"); err != nil {
		t.Fatal(err)
	}
	radius = sk.GetConstraint(radius.H)
	if radius.Expr != "2*(d/2)" || radius.ValA != 6 {
		t.Fatalf("radius: %q = %v", radius.Expr, radius.ValA)
	}
	if got := ConstraintText(radius); got != "d/2" {
		t.Fatalf("radius shows %q", got)
	}
	radius.Other = false
	if got := ConstraintText(radius); got != "2*(d/2)" {
		t.Fatalf("diameter shows %q", got)
	}
	radius.Other = true
	radius.Expr = "d"
	if got := ConstraintText(radius); got != "(d)/2" {
		t.Fatalf("radius shows %q", got)
	}

	// An always positive value takes the magnitude.
	angle := add(core.ConstraintAngle, 30, false)
	if err := sk.SetConstraintText(angle.H, "-d*10"); err != nil {
		t.Fatal(err)
	}
	if a := sk.GetConstraint(angle.H); a.ValA != 60 {
		t.Fatalf("angle = %v", a.ValA)
	}
	if got := ConstraintText(&core.Constraint{}); got != "" {
		t.Fatalf("no expression shows %q", got)
	}
	comment := add(core.ConstraintComment, 0, false)
	if err := sk.SetConstraintText(comment.H, "1"); err == nil {
		t.Fatal("a comment took a value")
	}
	for _, s := range []string{"-(a)+(b)", "-(a"} {
		if _, ok := unwrap(s, "-("); ok {
			t.Errorf("unwrap(%q) succeeded", s)
		}
	}
}

func TestGroupText(t *testing.T) {
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	// A group type that uses none of the values, so that it generates
	// whatever they are.
	hg := sk.AddGroup(Group{Type: GroupDrawing3D, Order: 5, ValA: 3, Scale: 1, Visible: true, Name: "rep"})
	sk.GenerateAll()
	if err := sk.AddParameter("n", "4"); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(sk.SetGroupText(hg, GroupCopies, "n+1"))
	g := sk.GetGroup(hg)
	if g.ValA != 5 || g.ExprA != "n+1" {
		t.Fatalf("copies %q = %v", g.ExprA, g.ValA)
	}
	for _, tc := range []struct {
		f    GroupValue
		text string
		want string
	}{
		{GroupCopies, "n-4", "fewer than 1"},
		{GroupCopies, "n*1000", "more than 999"},
		{GroupCopies, "0", "fewer than 1"},
		{GroupScale, "n*0", "cannot be zero"},
		{GroupScale, "0", "cannot be zero"},
		{GroupCopies, "m", "unknown parameter"},
		{GroupCopies, "+", "not a valid"},
	} {
		if err := sk.SetGroupText(hg, tc.f, tc.text); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("SetGroupText(%v, %q) = %v, want %q", tc.f.Field(), tc.text, err, tc.want)
		}
	}
	if g := sk.GetGroup(hg); g.ValA != 5 || g.ExprA != "n+1" {
		t.Fatalf("refused text changed copies: %q = %v", g.ExprA, g.ValA)
	}
	sk.Units = core.UnitInches
	must(sk.SetGroupText(hg, GroupPitch, "2"))
	sk.Units = core.UnitMM
	must(sk.SetGroupText(hg, GroupScale, "n/2"))
	must(sk.SetGroupText(hg, GroupValC, "n"))
	g = sk.GetGroup(hg)
	if g.ValB != 50.8 || g.ExprB != "" || g.Scale != 2 || g.ExprScale != "n/2" || g.ValC != 4 {
		t.Fatalf("values: pitch %q=%v scale %q=%v c %q=%v", g.ExprB, g.ValB, g.ExprScale, g.Scale, g.ExprC, g.ValC)
	}
	users := strings.Join(sk.ParameterUsers("n"), "; ")
	if users != "group rep copies; group rep val-c; group rep scale" {
		t.Fatalf("users: %s", users)
	}
	if err := sk.SetGroupText(core.HGroup{V: 0x77}, GroupCopies, "1"); err == nil {
		t.Fatal("missing group accepted")
	}

	// The parameter drives the group and dirties it.
	g.Clean = true
	must(sk.SetParameterExpr("n", "9"))
	if g := sk.GetGroup(hg); g.ValA != 10 || g.Clean {
		t.Fatalf("copies %v clean %v after n = 9", g.ValA, g.Clean)
	}
	// Then a value that the group refuses leaves it as it was.
	must(sk.SetParameterExpr("n", "2000"))
	if g := sk.GetGroup(hg); g.ValA != 10 {
		t.Fatalf("copies %v after n = 2000", g.ValA)
	}
	if len(sk.ExprErrors) != 1 || sk.ExprErrors[0].Field != "copies" ||
		!strings.HasPrefix(sk.ExprErrors[0].Error(), "group g") {
		t.Fatalf("ExprErrors %v", sk.ExprErrors)
	}
	if GroupValue(9).Field() != "GroupValue(9)" {
		t.Fatal("unknown field name")
	}
}

// Undo and redo bring the table and the expressions back.
func TestParameterUndo(t *testing.T) {
	sk, hr, hc := lineSketch(t)
	sk.UndoRemember()
	if err := sk.AddParameter("w", "7"); err != nil {
		t.Fatal(err)
	}
	if err := sk.SetConstraintText(hc, "w"); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if got := lineLength(sk, hr); math.Abs(got-7) > 1e-9 {
		t.Fatalf("line %v", got)
	}
	sk.UndoUndo()
	if len(sk.Parameters) != 0 || sk.GetConstraint(hc).Expr != "" {
		t.Fatalf("undo left %v and %q", sk.Parameters, sk.GetConstraint(hc).Expr)
	}
	if got := lineLength(sk, hr); math.Abs(got-10) > 1e-9 {
		t.Fatalf("line %v after undo", got)
	}
	sk.UndoRedo()
	if len(sk.Parameters) != 1 || sk.GetConstraint(hc).Expr != "w" {
		t.Fatalf("redo gave %v and %q", sk.Parameters, sk.GetConstraint(hc).Expr)
	}
	if got := lineLength(sk, hr); math.Abs(got-7) > 1e-9 {
		t.Fatalf("line %v after redo", got)
	}
	// Group values come back too.
	hg := sk.AddGroup(Group{Type: GroupDrawing3D, Order: 9, ValA: 3, Scale: 1, Name: "r"})
	sk.UndoRemember()
	if err := sk.SetGroupText(hg, GroupCopies, "w"); err != nil {
		t.Fatal(err)
	}
	sk.UndoUndo()
	if g := sk.GetGroup(hg); g.ExprA != "" || g.ValA != 3 {
		t.Fatalf("undo left copies %q = %v", g.ExprA, g.ValA)
	}
	sk.UndoRedo()
	if g := sk.GetGroup(hg); g.ExprA != "w" || g.ValA != 7 {
		t.Fatalf("redo gave copies %q = %v", g.ExprA, g.ValA)
	}
}

// A table changed behind the editing functions, as undo or a load
// does, is picked up by the next regeneration, which dirties what the
// change drives even when it regenerates only dirty groups.
func TestRegenerationEvaluates(t *testing.T) {
	sk, hr, hc := lineSketch(t)
	if err := sk.AddParameter("w", "7"); err != nil {
		t.Fatal(err)
	}
	if err := sk.SetConstraintText(hc, "w"); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	sk.Parameter("w").Expr = "8"
	for i := range sk.Groups {
		sk.Groups[i].Clean = true
	}
	sk.GenerateAllTyped(GenerateDirty)
	if got := lineLength(sk, hr); math.Abs(got-8) > 1e-9 {
		t.Fatalf("line %v, want 8", got)
	}
	if sk.EvaluateParameters() {
		t.Fatal("a second evaluation changed something")
	}
}

func TestSetConstraintValue(t *testing.T) {
	for _, tc := range []struct {
		typ   core.ConstraintType
		was   float64
		other bool
		v     float64
		want  float64
	}{
		{core.ConstraintPtPtDistance, 1, false, -5, 5},
		{core.ConstraintPtLineDistance, -1, false, 5, -5},
		{core.ConstraintPtLineDistance, -1, false, -5, 5},
		{core.ConstraintPtLineDistance, 1, false, -5, -5},
		{core.ConstraintDiameter, 1, false, -5, 5},
		{core.ConstraintDiameter, 1, true, 5, 10},
		{core.ConstraintAngle, 1, false, -30, 30},
	} {
		c := &core.Constraint{Type: tc.typ, ValA: tc.was, Other: tc.other, Expr: "x"}
		SetConstraintValue(c, tc.v)
		if c.ValA != tc.want || c.Expr != "" {
			t.Errorf("%v from %v with %v: %v %q, want %v", tc.typ, tc.was, tc.v, c.ValA, c.Expr, tc.want)
		}
	}
	if IsLengthValue(core.ConstraintAngle) || !IsLengthValue(core.ConstraintDiameter) {
		t.Fatal("IsLengthValue")
	}
}

func TestTextIsOneLine(t *testing.T) {
	sk := NewSketch()
	if err := sk.AddParameter("a", " 1 +\n\t2  "); err != nil {
		t.Fatal(err)
	}
	if got := sk.Parameter("a").Expr; got != "1 + 2" {
		t.Fatalf("stored %q", got)
	}
}

// A refused edit leaves no undo step; a taken one leaves one.
func TestUndoRememberFor(t *testing.T) {
	sk := NewSketch()
	sk.NewFile()
	sk.GenerateAll()
	if err := sk.UndoRememberFor(func() error { return sk.AddParameter("2bad", "1") }); err == nil {
		t.Fatal("bad name accepted")
	}
	if sk.Undo.Cnt != 0 {
		t.Fatalf("refused edit left %d undo steps", sk.Undo.Cnt)
	}
	if err := sk.UndoRememberFor(func() error { return sk.AddParameter("ok", "1") }); err != nil {
		t.Fatal(err)
	}
	if sk.Undo.Cnt != 1 {
		t.Fatalf("edit left %d undo steps", sk.Undo.Cnt)
	}
	sk.UndoUndo()
	if len(sk.Parameters) != 0 {
		t.Fatal("undo kept the parameter")
	}
}
