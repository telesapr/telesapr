package api

import (
	"math"
	"testing"
)

// A two-sided extrusion keeps its depth and puts the sketch in its
// middle; it comes back one-sided.
func TestTwoSidedExtrusion(t *testing.T) {
	d := New()
	d.Rect(V(0, 0, 0), V(10, 20, 0))
	ex := d.Extrude(6, "block")
	if ex.TwoSided() {
		t.Fatalf("a new extrusion is two-sided")
	}
	ex.SetTwoSided(true)
	if !ex.TwoSided() {
		t.Errorf("the extrusion isn't two-sided after SetTwoSided(true)")
	}
	lo, hi := ex.Bounds()
	if !near(lo.Z, -3) || !near(hi.Z, 3) {
		t.Errorf("the two-sided extrusion runs from z %g to %g, want -3 to 3", lo.Z, hi.Z)
	}
	if v := ex.Volume(); math.Abs(v-1200) > 1e-6 {
		t.Errorf("the two-sided extrusion's volume is %g, want 1200", v)
	}
	if r := ex.Check(); !r.Sound() {
		t.Errorf("the two-sided extrusion is not sound: %s", r.String())
	}
	ex.SetTwoSided(true) // no change
	ex.SetTwoSided(false)
	if lo, hi := ex.Bounds(); !near(lo.Z, 0) || !near(hi.Z, 6) {
		t.Errorf("the one-sided extrusion again runs from z %g to %g, want 0 to 6", lo.Z, hi.Z)
	}
}

// A step and repeat lays its copies on both sides.
func TestTwoSidedTranslate(t *testing.T) {
	d := New()
	d.Circle(V(0, 0, 0), 1)
	d.Extrude(2, "")
	tr := d.Translate(3, V(5, 0, 0), "row")
	tr.SetTwoSided(true)
	if !tr.TwoSided() {
		t.Errorf("the step and repeat isn't two-sided")
	}
	if lo, _ := tr.Bounds(); lo.X > -1-1e-6 {
		t.Errorf("a two-sided step and repeat reaches only x %g to the left", lo.X)
	}
}

func TestTwoSidedMisuse(t *testing.T) {
	d := New()
	sketch := d.Active()
	for name, fn := range map[string]func(){
		"two-sided sketch":      func() { sketch.SetTwoSided(true) },
		"is a sketch two-sided": func() { sketch.TwoSided() },
	} {
		func() {
			defer func() {
				if _, ok := recover().(error); !ok {
					t.Errorf("%s: recovered no error", name)
				}
			}()
			fn()
		}()
	}
}
