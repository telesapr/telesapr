package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// betweenPair is a pair of round faces off each other by eps (positive:
// sunk in), for TestBetweenContactSweep, and the area their contact has
// at most: a pin in a bore (lathed or drilled), a cone in a countersink,
// a ball in a seat, a ball on a plate, a cylinder lying on one.
func betweenPair(fx string, eps float64, tol Tolerance) (a, b *SShell, area float64) {
	v := func(x, z float64) geom.Vector { return geom.Vector{X: x, Z: z} }
	x, y := geom.Vector{X: 1}, geom.Vector{Y: 1}
	plate := func() *SShell {
		return extrudedBox(geom.Vector{X: -5, Y: -5, Z: -2}, geom.Vector{X: 5, Y: 5, Z: 0}, tol)
	}
	switch fx {
	case "pin":
		a = lathedProfile([]geom.Vector{v(3, 0), v(5, 0), v(5, 6), v(3, 6)}, tol)
		b = lathedProfile([]geom.Vector{v(0, -1), v(3+eps, -1), v(3+eps, 7), v(0, 7)}, tol)
		return a, b, 2 * math.Pi * 3 * 8
	case "drilled":
		blk := extrudedBox(geom.Vector{X: -6, Y: -6, Z: 0}, geom.Vector{X: 6, Y: 6, Z: 6}, tol)
		drill := extrudedCurves(circleCurves(geom.Vector{}, x, y, 3), geom.Vector{Z: -1}, geom.Vector{Z: 7}, tol)
		a = &SShell{Tol: tol}
		a.MakeFromBoolean(blk, drill, SurfaceDiff)
		b = extrudedCurves(circleCurves(geom.Vector{}, x, y, 3+eps), geom.Vector{Z: -1}, geom.Vector{Z: 7}, tol)
		return a, b, 2 * math.Pi * 3 * 8
	case "cone", "seat":
		a, b = coaxialPair(fx, eps, tol)
		return a, b, 4 * math.Pi * 9
	case "ball":
		return plate(), patchUpSphere(3, geom.Vector{Z: 3 - eps}, tol), 4 * math.Pi * 9
	}
	c := lathedProfile([]geom.Vector{v(0, -4), v(2, -4), v(2, 4), v(0, 4)}, tol)
	b = &SShell{Tol: tol}
	b.MakeFromTransformationOf(c, geom.Vector{Z: 2 - eps}, geom.QuaternionFromAxisAngle(geom.Vector{Y: 1}, math.Pi/2), 1)
	return plate(), b, 2 * math.Pi * 2 * 8
}

// TestBetweenContactSweep is roadmap K3's "left" band: round faces off
// each other by more than the tenth of the chord tolerance the first fuse
// takes and less than the tolerance, sunk in or standing off, turned, at
// two tolerances, by the three Booleans. On 2026-11-05 before snapBetween,
// of 576 such Booleans of pins, bores, cones, seats, balls and lying
// cylinders (the planes' 384, TestNearContactSweep's fixtures, were
// right), 143 failed and 31
// left the mesh open, silently; after, every one is made and sound, its
// volume the two's taken as touching (off by at most the contact's area
// times the offset), or, where the two as drawn came out right, that.
func TestBetweenContactSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("fixture", "pin", "drilled", "cone", "seat", "ball", "lying"),
			sweep.Floats("f", -0.9, -0.6, -0.3, -0.15, 0.15, 0.3, 0.6, 0.9),
			sweep.Floats("chord", 0.1, 0.01),
			sweep.Floats("turn", 0, 35),
			sweep.Strings("op", "union", "difference", "intersection"),
		},
		MaxFailRate: 0,
		Sample:      60,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		chord := p.Float("chord")
		tol, eps := Tol(chord, 20), p.Float("f")*chord
		a, b, area := betweenPair(p.String("fixture"), eps, tol)
		a, b = nearTurn(a, p.Float("turn")), nearTurn(b, p.Float("turn"))
		aV, bV := shellVolume(a), shellVolume(b)
		var op SurfaceCombineAs
		var want float64
		switch p.String("op") {
		case "union":
			op, want = SurfaceUnion, aV+bV
		case "difference":
			op, want = SurfaceDiff, aV
		default:
			op = SurfaceInter
		}
		slack := 2e-3*math.Max(want, 1) + area*math.Abs(eps)
		var out SShell
		out.MakeFromBoolean(a, b, op)
		if out.BooleanFailed {
			return errUnsound(true)
		}
		got, sound := soundVolume(&out)
		if want <= slack {
			if math.Abs(got) > slack {
				return fmt.Errorf("%w: %g, want none", errNearVolume, got)
			}
			return nil
		}
		if !sound {
			return errUnsound(false)
		}
		if math.Abs(got-want) > slack {
			return fmt.Errorf("%w: %g, want %g", errNearVolume, got, want)
		}
		return nil
	}).Require(t)
}

// TestBetweenFusedSaysHowFar unites a pin 0.03 mm over a bore's size at a
// chord tolerance of 0.1 mm: as drawn the Boolean fails, fused the pin
// fills the bore, and the shell says by how much it moved (Fused); a pin
// of the bore's size, or 0.2 mm over it, moves nothing.
func TestBetweenFusedSaysHowFar(t *testing.T) {
	t.Parallel()
	tol := Tol(0.1, 20)
	for _, c := range []struct{ eps, fused float64 }{{0.03, 0.03}, {0, 0}, {0.2, 0}} {
		a, b, _ := betweenPair("pin", c.eps, tol)
		var out SShell
		out.MakeFromBoolean(a, b, SurfaceUnion)
		if math.Abs(out.Fused-c.fused) > 1e-6 {
			t.Errorf("eps %g: fused %g, want %g", c.eps, out.Fused, c.fused)
		}
		if out.BooleanFailed || !meshIsSound(&out) {
			t.Errorf("eps %g: failed %v, sound %v", c.eps, out.BooleanFailed, meshIsSound(&out))
		}
	}
}
