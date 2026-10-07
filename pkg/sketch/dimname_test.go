package sketch

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// A second line beside lineSketch's, and a distance between its ends.
func secondLine(t *testing.T, sk *Sketch) (core.HRequest, core.HConstraint) {
	t.Helper()
	hg := sk.Active
	hr := sk.AddRequest(Request{Type: RequestLineSegment, Group: hg, Workplane: hg.Entity(0)})
	sk.GenerateAll()
	sk.MustGetEntity(hr.Entity(1)).PointForceTo(sk, geom.Vector{Y: 5})
	sk.MustGetEntity(hr.Entity(2)).PointForceTo(sk, geom.Vector{X: 3, Y: 5})
	hc := sk.AddConstraint(core.Constraint{
		Type: core.ConstraintPtPtDistance, Group: hg, Workplane: hg.Entity(0),
		PtA: hr.Entity(1), PtB: hr.Entity(2), ValA: 3})
	sk.GenerateAll()
	return hr, hc
}

// A dimension read by name (roadmap 16.1): one line's length named
// width drives another's through "width/2", and follows a change of it,
// a rename, a change of unit; a name in use can't be taken away, names
// are shared with the parameters, and a cycle through a name is an
// error on the expression.
func TestDimensionByName(t *testing.T) {
	sk, hr, hw := lineSketch(t)
	hr2, hh := secondLine(t, sk)
	if err := sk.SetConstraintName(hw, "width"); err != nil {
		t.Fatal(err)
	}
	if err := sk.SetConstraintText(hh, "width/2"); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if a, b := lineLength(sk, hr), lineLength(sk, hr2); math.Abs(a-10) > 1e-9 || math.Abs(b-5) > 1e-9 {
		t.Fatalf("lines %g and %g, want 10 and 5", a, b)
	}
	if err := sk.SetConstraintText(hw, "14"); err != nil {
		t.Fatal(err)
	}
	sk.GenerateAll()
	if b := lineLength(sk, hr2); math.Abs(b-7) > 1e-9 {
		t.Fatalf("after width = 14 the second line is %g", b)
	}
	// A parameter and the dimension name each other's values.
	if err := sk.AddParameter("half", "width/2"); err != nil {
		t.Fatal(err)
	}
	if v := sk.Parameter("half").Value; v != 7 {
		t.Errorf("half = %g", v)
	}
	if err := sk.AddParameter("width", "1"); err == nil || !strings.Contains(err.Error(), "dimension") {
		t.Errorf("a parameter named as the dimension: %v", err)
	}
	if err := sk.SetConstraintName(hh, "half"); err == nil || !strings.Contains(err.Error(), "parameter") {
		t.Errorf("a dimension named as the parameter: %v", err)
	}
	// A rename rewrites the users; a name in use stays.
	if err := sk.SetConstraintName(hw, "w"); err != nil {
		t.Fatal(err)
	}
	if c := sk.GetConstraint(hh); c.Expr != "w/2" || sk.Parameter("half").Expr != "w/2" {
		t.Errorf("after the rename: %q, %q", c.Expr, sk.Parameter("half").Expr)
	}
	if err := sk.SetConstraintName(hw, ""); err == nil || !strings.Contains(err.Error(), "used by") {
		t.Errorf("taking a used name away: %v", err)
	}
	// A cycle: the width from the second line's, which is from it.
	if err := sk.SetConstraintText(hw, "half*2"); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Errorf("a cycle: %v", err)
	}
	// In inches, the lengths stay.
	sk.SetUnits(core.UnitInches)
	sk.GenerateAll()
	if a, b := lineLength(sk, hr), lineLength(sk, hr2); math.Abs(a-14) > 1e-9 || math.Abs(b-7) > 1e-9 {
		t.Errorf("in inches the lines are %g and %g mm", a, b)
	}
	sk.SetUnits(core.UnitMM)
	sk.GenerateAll()
	if b := lineLength(sk, hr2); math.Abs(b-7) > 1e-9 {
		t.Errorf("back in mm the second line is %g mm (%q)", b, sk.GetConstraint(hh).Expr)
	}
	// What can't be named.
	sk.GetConstraint(hh).Reference = true
	if err := sk.SetConstraintName(hh, "h"); err == nil || !strings.Contains(err.Error(), "reference") {
		t.Errorf("a reference dimension named: %v", err)
	}
	if err := sk.SetConstraintName(hw, "2x"); err == nil || !strings.Contains(err.Error(), "can't name") {
		t.Errorf("a bad name: %v", err)
	}
}
