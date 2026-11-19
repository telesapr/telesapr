package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// An arc of 156° and radius 16.43, the far arc of the cutter round a
// drafted disc, is cut into as many chords as a curve may be, which at
// a fine tolerance sag past it: a point inside the arc but past its chords is within the band the
// trim may be wrong in, where the tolerance alone left it outside; one
// further in is clear of it, and an arc whose chords sag less than the
// tolerance keeps the tolerance as its band.
func TestCurvedClearanceCoarseChords(t *testing.T) {
	t.Parallel()
	const r, half = 16.43, 78 * math.Pi / 180
	tol := Tol(0.0005, 32)
	at := func(d, a float64) geom.Vector { return geom.Vector{X: d * math.Cos(a), Y: d * math.Sin(a)} }
	arc := SBezierFrom3(at(r, -half), at(r/math.Cos(half), 0), at(r, half))
	arc.Weight[1] = math.Cos(half)
	var el SEdgeList
	arc.MakePwlInto(&el, tol, 1)
	e := el.Edges[len(el.Edges)/2]
	l := e.B.Minus(e.A).Magnitude()
	sag := chordSag(&el, len(el.Edges)/2)
	want := r - math.Sqrt(r*r-l*l/4)
	if math.Abs(sag-want) > 0.05*want || sag <= tol.ChordTolMm() {
		t.Fatalf("sag %.5f, want %.5f, past the tolerance", sag, want)
	}
	// Midway between two corners, just inside the true arc.
	m := e.A.Plus(e.B).ScaledBy(0.5)
	p := at(r-want/4, math.Atan2(m.Y, m.X))
	if d := distanceToEdges(p, &el); d < tol.ChordTolMm() {
		t.Fatalf("point %.4f from the chords, within the tolerance", d)
	}
	if c := curvedClearance(p, &el, tol.ChordTolMm()); c >= 0 {
		t.Errorf("point inside the arc, past its chords: clearance %.4f, want it within the band", c)
	}
	if c := curvedClearance(at(r-0.1, 0), &el, tol.ChordTolMm()); c <= 0 {
		t.Errorf("point 0.1 inside: clearance %.4f, want it clear", c)
	}
	var fine SEdgeList
	arc.MakePwlInto(&fine, Tol(0.0005, 1000), 1)
	q := at(r-0.0004, 0)
	if c, d := curvedClearance(q, &fine, 0.0005), distanceToEdges(q, &fine); math.Abs(c-(d-0.0005)) > 1e-12 {
		t.Errorf("fine chords: clearance %.6f, want %.6f", c, d-0.0005)
	}
}
