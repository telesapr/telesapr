package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// touchRingPair is a disc of radius 12 from z = 0 to 4 with a conical
// hole about (cx, 0) of radius z + 3.5, and a ring of radii 8 and 16
// from z = 1 to 8 about the origin: at z = 1, the ring's bottom, the
// hole's circle of radius 4.5 lies inside the ring's inner circle,
// gap from it at (8, 0, 1), and above it the hole crosses into the
// ring.
func touchRingPair(gap, turnA, turnB float64, tol Tolerance) (a, b *SShell) {
	disc := extrudedCurves(circleCurves(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1}, 12),
		geom.Vector{}, geom.Vector{Z: 4}, tol)
	cone := lathedProfile([]geom.Vector{{Z: -1}, {X: 2.5, Z: -1}, {X: 8.5, Z: 5}, {Z: 5}}, tol)
	cx := 3.5 - gap
	moved := &SShell{Tol: tol}
	moved.MakeFromTransformationOf(cone, geom.Vector{X: cx}, geom.QuaternionFromAxisAngle(geom.Vector{Z: 1}, turnA*math.Pi/180), 1)
	a = &SShell{Tol: tol}
	a.MakeFromBoolean(disc, moved, SurfaceDiff)
	ring := circleCurves(geom.Vector{Z: 1}, geom.Vector{X: 1}, geom.Vector{Y: 1}, 16)
	inner := circleCurves(geom.Vector{Z: 1}, geom.Vector{X: 1}, geom.Vector{Y: 1}, 8)
	for _, c := range inner.Curves {
		c.Reverse()
		ring.Curves = append(ring.Curves, c)
	}
	b = extrudedCurves(ring, geom.Vector{}, geom.Vector{Z: 7}, tol)
	if turnB != 0 {
		turned := &SShell{Tol: tol}
		turned.MakeFromTransformationOf(b, geom.Vector{}, geom.QuaternionFromAxisAngle(geom.Vector{Z: 1}, turnB*math.Pi/180), 1)
		b = turned
	}
	return a, b
}

// touchRingVolumes are the volumes of a less b, a and b's intersection,
// and the volume of a, by integrating the sections up z.
func touchRingVolumes(gap float64) (diff, inter, aV float64) {
	cx := 3.5 - gap
	const n = 4000
	for i := range n {
		z := 4 * (float64(i) + 0.5) / n
		rho := z + 3.5
		inDisc := math.Pi*144 - lensArea(12, rho, cx)
		aV += inDisc * 4 / n
		if z > 1 {
			in8 := math.Pi*64 - lensArea(8, rho, cx)
			diff += in8 * 4 / n
			inter += (inDisc - in8) * 4 / n
		} else {
			diff += inDisc * 4 / n
		}
	}
	return diff, inter, aV
}

// TestBooleanTouchingRingSweep cuts a ring from a solid, intersects
// them and unites them, the solid's conical hole meeting the plane of
// the ring's bottom in a circle that lies just inside the ring's inner
// circle, one to twenty chord tolerances from it: two circles in one
// plane, one just inside the other. The ring's circle, in quarters of
// 32 chords, sags 0.0024 off its arc, and its chords crossed the hole's
// cone where the circle does not: split there, the cone's circle got
// seeds and pieces between the chords and the arc, inside the ring's
// bottom, and the Booleans failed or their meshes crossed, 42 of the
// 168 (missesCurve, refineNearMisses, 2026-11-19). The hole and the
// ring are turned about their axes so that the touching point falls on
// a seam of either or inside a patch. Nearer than the chord tolerance
// the two are fused (snapBetween), as near contacts are; touching
// exactly is TestBooleanTouchingRingExact.
func TestBooleanTouchingRingSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("chord", 0.0005, 0.002),
			sweep.Floats("gap", 1, 2, 3, 4, 6, 10, 20),
			sweep.Floats("turn hole", 0, 30),
			sweep.Floats("turn ring", 0, 40),
			sweep.Strings("op", "difference", "intersection", "union"),
		},
		MaxFailRate: 0,
		Sample:      24,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		return touchRingCase(p.Float("chord"), p.Float("gap")*p.Float("chord"),
			p.Float("turn hole"), p.Float("turn ring"), p.String("op"))
	}).Require(t)
}

// TestBooleanTouchingRingExact is TestBooleanTouchingRingSweep with the
// hole's circle touching the ring's inner circle exactly at (8, 0, 1).
// The two lie within LengthEps of each other for 0.009 along them, and
// the ring's chord crossed the hole's cone twice a hair either side of
// the point: each crossing split the circle and seeded a march, which
// stopped at once at the other, and the hole's circle in the ring's
// bottom, between the ring's chords and its arc beside the point, was
// kept inside the ring's face, dangling from it. All 24 failed. The
// point is found where the circle's tangent lies in the cone
// (exactTouch), split at and marched to (touchSinks), the crossings'
// seeds beside it dropped (dropGrazingSeeds), and the side of the
// ring's trim beside it taken from the exact circle (outsideExactTrim):
// all 24 sound (2026-11-27).
func TestBooleanTouchingRingExact(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("chord", 0.0005, 0.002),
			sweep.Floats("turn hole", 0, 30),
			sweep.Floats("turn ring", 0, 40),
			sweep.Strings("op", "difference", "intersection", "union"),
		},
		MaxFailRate: 0,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		return touchRingCase(p.Float("chord"), 0, p.Float("turn hole"), p.Float("turn ring"), p.String("op"))
	}).Require(t)
}

// touchRingCase makes the Boolean op of touchRingPair at chord
// tolerance ct and the gap and turns given, and checks it closes into a
// sound mesh of the volume touchRingVolumes integrates.
func touchRingCase(ct, gap, turnHole, turnRing float64, op string) error {
	tol := Tol(ct, 20)
	a, b := touchRingPair(gap, turnHole, turnRing, tol)
	diff, inter, aV := touchRingVolumes(gap)
	var out SShell
	var want float64
	switch op {
	case "difference":
		out.MakeFromBoolean(a, b, SurfaceDiff)
		want = diff
	case "intersection":
		out.MakeFromBoolean(a, b, SurfaceInter)
		want = inter
	default:
		out.MakeFromBoolean(a, b, SurfaceUnion)
		want = aV - inter + shellVolume(b)
	}
	if out.BooleanFailed {
		return errUnsound(true)
	}
	got, sound := soundVolume(&out)
	if !sound {
		return errUnsound(false)
	}
	if math.Abs(got-want) > 2e-3*want {
		return fmt.Errorf("%w: %g, want %g", errNearVolume, got, want)
	}
	return nil
}
