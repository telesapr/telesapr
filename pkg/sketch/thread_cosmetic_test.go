package sketch

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// band is an M10 thread 18 long running down the Z axis from z = 30,
// as a bolt's from its free end.
var band = ThreadBand{
	Origin: geom.Vector{Z: 30}, Axis: geom.Vector{Z: -1}, Length: 18,
	Crest: 5, Root: 4.188, Inner: 4.038, Outer: 5,
	Groups: []core.HGroup{{V: 7}, {V: 9}},
}

// TestThreadBandHolds places points in the band and out of it, within
// a tolerance along the axis and across it, and checks the band's other
// measures: how far along it a point is, on what cylinder, and which
// faces its thread made.
func TestThreadBandHolds(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		p    geom.Vector
		tol  float64
		want bool
	}{
		{geom.Vector{X: 4.5, Z: 20}, 1e-3, true},
		{geom.Vector{X: 4.5, Z: 30.0005}, 1e-3, true},
		{geom.Vector{X: 4.5, Z: 30.01}, 1e-3, false},
		{geom.Vector{X: 4.5, Z: 11.995}, 1e-3, false},
		{geom.Vector{Y: 5.005, Z: 13}, 1e-3, false},
		{geom.Vector{Y: 5.005, Z: 13}, 0.01, true},
		{geom.Vector{X: 4.03, Z: 13}, 0.01, true},
		{geom.Vector{X: 3.9, Z: 13}, 0.01, false},
	} {
		if got := band.Holds(c.p, c.tol); got != c.want {
			t.Errorf("Holds(%v, %g) = %v, want %v", c.p, c.tol, got, c.want)
		}
	}
	if a := band.Along(geom.Vector{X: 1, Z: 12}); math.Abs(a-18) > 1e-12 {
		t.Errorf("Along at the far end = %g, want 18", a)
	}
	if !band.OnCylinder(geom.Vector{X: 3, Y: 4, Z: -40}, 5, 1e-9) || band.OnCylinder(geom.Vector{X: 4.188}, 5, 1e-3) {
		t.Error("OnCylinder takes a point by its distance from the axis alone")
	}
	if p := band.ToCrest(geom.Vector{X: 0, Y: 4.2, Z: 20}); p.Minus(geom.Vector{Y: 5, Z: 20}).Magnitude() > 1e-12 {
		t.Errorf("ToCrest = %v", p)
	}
	if !band.Makes(core.HGroup{V: 9}.Entity(3).V) || band.Makes(core.HGroup{V: 8}.Entity(3).V) {
		t.Error("Makes takes what the thread's groups made, and nothing else")
	}
}

// A chamfered band's surface runs in at 45° to its chamfered ends, out
// from the axis on a screw and in towards it in a nut, its root drawn
// from where the root leaves the cone; without a chamfer the surface
// is the crest all along and the root runs from the end to the run-out.
func TestThreadBandChamfer(t *testing.T) {
	t.Parallel()
	screw := ThreadBand{Axis: geom.Vector{Z: 1}, Length: 20, Crest: 5, Root: 4.2, Chamfer: 3.5, ChamferFar: true}
	nut := ThreadBand{Axis: geom.Vector{Z: 1}, Length: 10, Crest: 4.2, Root: 5, Chamfer: 5.6, Internal: true}
	plain := ThreadBand{Axis: geom.Vector{Z: 1}, Length: 10, Crest: 5, Root: 4.2, RunOut: 2}
	for _, c := range []struct {
		b    ThreadBand
		t    float64
		want float64
	}{
		{screw, 0, 3.5}, {screw, 1, 4.5}, {screw, 1.5, 5}, {screw, 10, 5}, {screw, 19.5, 4}, {screw, 20, 3.5},
		{nut, 0, 5.6}, {nut, 1, 4.6}, {nut, 1.4, 4.2}, {nut, 10, 4.2},
		{plain, 0, 5}, {plain, 10, 5},
	} {
		if got := c.b.CrestAt(c.t); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("%+v at %g: %g, want %g", c.b, c.t, got, c.want)
		}
	}
	for _, c := range []struct {
		b        ThreadBand
		from, to float64
	}{{screw, 0.7, 19.3}, {nut, 0.6, 10}, {plain, 0, 8}} {
		if from, to := c.b.RootSpan(); math.Abs(from-c.from) > 1e-12 || math.Abs(to-c.to) > 1e-12 {
			t.Errorf("%+v: root from %g to %g, want %g to %g", c.b, from, to, c.from, c.to)
		}
	}
	if p := screw.ToCrest(geom.Vector{X: 1, Z: 1}); math.Abs(p.X-4.5) > 1e-12 || p.Z != 1 {
		t.Errorf("onto the cone: %v", p)
	}
}
