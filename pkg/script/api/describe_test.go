package api

import (
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// The description of a drawing names every group, request and
// constraint as the lookups take them, with where things are.
func TestDescribe(t *testing.T) {
	d := New()
	if err := d.Params.Set("width", "40"); err != nil {
		t.Fatal(err)
	}
	if err := d.Params.Set("half", "width/2"); err != nil {
		t.Fatal(err)
	}
	sides := d.Sketch.Rect(V(0, 0, 0), V(40, 20, 0))
	d.Constrain.Coincident(sides[0].Start(), d.Origin())
	d.Constrain.Length(sides[0], 40)
	d.Constrain.Length(sides[1], 20)
	hole := d.Sketch.Circle(V(10, 10, 0), 2.5)
	d.Constrain.Diameter(hole, 5)
	guide := d.Sketch.Line(V(0, 30, 0), V(5, 30, 0))
	guide.SetConstruction(true)
	sketch := d.Active()
	d.Features.Extrude(4, "plate")
	d.Activate(sketch)

	const want = `sketch: unit mm, unsaved changes
parameters:
    width = 40
    half = width/2 = 20
groups, in the order the model is built:
    g002-sketch-in-plane, sketch-in-plane: solved, 6 DOF; active
    g003-plate, extrude: solved, 1 DOF, volume 3121.967 mm³, bounds (0, 0, 0) to (40, 20, 4)
active group g002-sketch-in-plane, sketch-in-plane: solved, 6 DOF
  requests:
    r004-line-segment: line (40, 0, 0) to (40, 20, 0)
    r005-line-segment: line (0, 0, 0) to (40, 0, 0)
    r006-line-segment: line (0, 20, 0) to (0, 0, 0)
    r007-line-segment: line (40, 20, 0) to (0, 20, 0)
    r008-circle: circle at (10, 10, 0), radius 2.5
    r009-line-segment: line (0, 30, 0) to (5, 30, 0), construction
  constraints:
    c001-pts-coincident, on r004-line-segment, r005-line-segment
`
	got := d.Describe()
	if !strings.HasPrefix(got, want) {
		t.Errorf("the description is\n%s\nwant it to start with\n%s", got, want)
	}
	for _, line := range []string{
		"c009-pts-coincident, on r005-line-segment, r001-#XY\n",
		"c00a-pt-pt-distance = 40, on r005-line-segment\n",
		"c00c-diameter = 5, on r008-circle\n",
		"c006-horizontal, on r005-line-segment\n",
	} {
		if !strings.Contains(got, line) {
			t.Errorf("the description lacks a constraint %q:\n%s", line, got)
		}
	}
	if strings.Contains(got, "selected:") {
		t.Errorf("nothing is selected without an interface, but the description says:\n%s", got)
	}

	// Every name it gives is one the lookups take.
	for line := range strings.SplitSeq(got, "\n") {
		name, _, _ := strings.Cut(strings.TrimSpace(line), ":")
		name, _, _ = strings.Cut(name, ",")
		name, _, _ = strings.Cut(name, " ")
		switch {
		case strings.HasPrefix(line, "    r"):
			if !d.Request(name).Valid() {
				t.Errorf("no request is called %q, from %q", name, line)
			}
		case strings.HasPrefix(line, "    c"):
			if !d.Constraint(name).Valid() {
				t.Errorf("no constraint is called %q, from %q", name, line)
			}
		case strings.HasPrefix(line, "    g"):
			if !d.Group(name).Valid() {
				t.Errorf("no group is called %q, from %q", name, line)
			}
		}
	}

	// A group's own description is what the document gives for the
	// active one.
	if own := sketch.Describe(); !strings.Contains(got, "active group "+own) {
		t.Errorf("the group describes itself as\n%s\nwhich the document's description lacks:\n%s", own, got)
	}
	plate := d.Group("plate").Describe()
	if !strings.HasPrefix(plate, "g003-plate, extrude: solved, 1 DOF, volume") || strings.Contains(plate, "requests:") {
		t.Errorf("the extrusion describes itself as\n%s", plate)
	}
}

// What is wrong shows in the description: a group that didn't solve,
// one that is left out, an expression without a value, the unit.
func TestDescribeTrouble(t *testing.T) {
	d := New()
	line := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	d.Constrain.Length(line, 10)
	// A second length is a reference where the window would make it one;
	// the group fails only once it is a driving dimension.
	d.Constrain.Length(line, 12).SetReference(false)
	if got := d.Describe(); !strings.Contains(got, "NOT SOLVED") {
		t.Errorf("two lengths on one line, and the description says:\n%s", got)
	}

	d = New()
	d.View.SetUnits(core.UnitInches)
	d.Sketch.Rect(V(0, 0, 0), V(10, 10, 0))
	box := d.Features.Extrude(5, "box")
	box.SetSuppress(SuppressWhole)
	text := d.Constrain.Comment(V(1, 1, 0), "a note")
	got := d.Describe()
	for _, part := range []string{"unit inch", "g003-box, extrude: left out (suppressed)", `-comment "a note"`} {
		if !strings.Contains(got, part) {
			t.Errorf("the description lacks %q:\n%s", part, got)
		}
	}
	if !text.Valid() {
		t.Errorf("the comment is gone")
	}

	var deleted Group
	func() {
		defer func() {
			if _, ok := recover().(error); !ok {
				t.Errorf("describing no group didn't panic with an error")
			}
		}()
		_ = deleted.Describe()
	}()
}
