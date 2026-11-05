package api

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/sketch"
)

// A pin drawn a few hundredths over a bore's size, closer than the chord
// tolerance but further than a tenth of it, is united with the plate:
// as drawn, the two sides cut each other in curves the mesh cannot tell
// apart, and the Boolean failed or left the mesh open. The pin is made to
// fill the bore, the group saying by how much (roadmap K3).
func TestPinOverBoreFused(t *testing.T) {
	t.Parallel()
	for _, f := range []float64{0.3, 0.6} {
		d, _ := plateWithHole(t, 10)
		var chord float64
		d.With(func(sk *sketch.Sketch) { chord = sk.Tol().ChordTolMm() })
		gap := f * chord
		d.Features.SketchIn(d.Workplane("XY"), "pin")
		d.Sketch.Circle(V(0, 0, 0), 5+gap)
		g := d.Features.Extrude(14, "pin")
		if p := g.Problem(); p != "" {
			t.Fatalf("gap %g of chord %g: %s", gap, chord, p)
		}
		if rep := g.Analyze().Check(); !rep.Sound() {
			t.Errorf("gap %g of chord %g: not sound: %s", gap, chord, rep.Summary())
		}
		// The plate with the bore filled, and the pin past it, of the
		// bore's size: the plate's whole volume and 4 mm of pin.
		want := 30*30*10 + math.Pi*25*4
		if got := g.Analyze().Volume(); math.Abs(got-want) > 2e-3*want {
			t.Errorf("gap %g of chord %g: volume %g, want %g", gap, chord, got, want)
		}
		if n := g.Note(); !strings.Contains(n, "made to meet") {
			t.Errorf("gap %g of chord %g: note %q", gap, chord, n)
		}
	}
}
