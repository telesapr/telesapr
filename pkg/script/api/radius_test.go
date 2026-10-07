package api

import (
	"strings"
	"testing"
)

// A radius is named a radius, so its name and the value it shows agree;
// a diameter stays a diameter.
func TestRadiusName(t *testing.T) {
	d := New()
	a := d.Circle(V(0, 0, 0), 2)
	b := d.Circle(V(10, 0, 0), 2)
	r := d.Radius(a, 3)
	dia := d.Diameter(b, 8)
	if got := r.String(); !strings.HasSuffix(got, "-radius = 3") {
		t.Errorf("the radius describes itself as %q, want ...-radius = 3", got)
	}
	if got := dia.String(); !strings.HasSuffix(got, "-diameter = 8") {
		t.Errorf("the diameter describes itself as %q, want ...-diameter = 8", got)
	}
	if v := r.Value(); !near(v, 3) {
		t.Errorf("the radius's value is %g, want 3", v)
	}
	if !strings.Contains(d.Describe(), r.String()+", on ") {
		t.Errorf("the description lacks %q:\n%s", r.String(), d.Describe())
	}
	if name, _, _ := strings.Cut(r.String(), " "); !d.Constraint(name).Valid() {
		t.Errorf("no constraint is called %q", name)
	}
}
