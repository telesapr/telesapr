package api

import "testing"

// A diameter is shown as a radius and as a diameter again, by
// SetShowAsRadius and by SetOther, and what it holds stays the diameter.
func TestDiameterShowAsRadius(t *testing.T) {
	t.Parallel()
	d := New()
	circle := d.Sketch.Circle(V(0, 0, 0), 5)
	c := d.Constrain.Diameter(circle, 10)
	if c.Other() || !near(c.Value(), 10) {
		t.Fatalf("a diameter shows %g, other %v", c.Value(), c.Other())
	}
	c.SetShowAsRadius(true)
	if !c.Other() || !near(c.Value(), 5) {
		t.Errorf("shown as a radius it reads %g, other %v; want 5", c.Value(), c.Other())
	}
	c.SetOther(false)
	if c.Other() || !near(c.Value(), 10) {
		t.Errorf("shown as a diameter it reads %g, other %v; want 10", c.Value(), c.Other())
	}
	c.SetOther(true)
	d.Edit.Regen()
	if r := circle.Radius(); !near(r, 5) {
		t.Errorf("the circle is %g in radius, want 5", r)
	}

	length := d.Constrain.Length(d.Sketch.Line(V(20, 0, 0), V(30, 0, 0)), 10)
	for name, fn := range map[string]func(){
		"radius of a length": func() { length.SetShowAsRadius(true) },
		"other of a length":  func() { length.SetOther(true) },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: want an error", name)
				}
			}()
			fn()
		}()
	}
}

// A label's place is read back and moved, for a dimension and for a
// comment; a constraint with no label refuses.
func TestLabelOffset(t *testing.T) {
	t.Parallel()
	d := New()
	l := d.Sketch.Line(V(0, 0, 0), V(10, 0, 0))
	dim := d.Constrain.Length(l, 10)
	dim.SetLabelOffset(V(1, 2, 3))
	if got := dim.LabelOffset(); got != V(1, 2, 3) {
		t.Errorf("a dimension's label is at %v, want (1, 2, 3)", got)
	}
	note := d.Constrain.Comment(V(4, 5, 0), "hi")
	if got := note.LabelOffset(); got != V(4, 5, 0) {
		t.Errorf("a comment is at %v, want (4, 5, 0)", got)
	}
	note.SetLabelOffset(V(7, 8, 0))
	if got := note.LabelOffset(); got != V(7, 8, 0) {
		t.Errorf("a comment is at %v, want (7, 8, 0)", got)
	}

	horizontal := d.Constrain.Horizontal(l)
	func() {
		defer func() {
			if _, ok := recover().(error); !ok {
				t.Errorf("a label for a horizontal constraint: want an error")
			}
		}()
		horizontal.SetLabelOffset(V(1, 1, 0))
	}()
}
