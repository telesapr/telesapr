package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// circleCurves is a circle of radius r about c in the plane spanned by
// u and v, as four rational quadratic arcs.
func circleCurves(c, u, v geom.Vector, r float64) *SBezierList {
	w := math.Sqrt(2) / 2
	sbl := &SBezierList{}
	for i := range 4 {
		a0, a1 := float64(i)*math.Pi/2, float64(i+1)*math.Pi/2
		am := (a0 + a1) / 2
		p0 := c.Plus(u.ScaledBy(r * math.Cos(a0))).Plus(v.ScaledBy(r * math.Sin(a0)))
		p2 := c.Plus(u.ScaledBy(r * math.Cos(a1))).Plus(v.ScaledBy(r * math.Sin(a1)))
		p1 := c.Plus(u.ScaledBy(r / w * math.Cos(am))).Plus(v.ScaledBy(r / w * math.Sin(am)))
		sb := SBezierFrom3(p0, p1, p2)
		sb.Weight[1] = w
		sbl.Curves = append(sbl.Curves, sb)
	}
	return sbl
}

func polylineCurves(p ...geom.Vector) *SBezierList {
	sbl := &SBezierList{}
	for i := range p {
		sbl.Curves = append(sbl.Curves, SBezierFrom2(p[i], p[(i+1)%len(p)]))
	}
	return sbl
}

// noFaces is a ShellGroup without a remap table: every face is 0.
type noFaces struct{}

func (noFaces) Remap(core.HEntity, int) core.HEntity { return core.NoEntity }
func (noFaces) EntityExists(core.HEntity) bool       { return false }

// loopsOf assembles curves into loop sets as a sketch's loops are.
func loopsOf(sbl *SBezierList) *SBezierLoopSetSet {
	var sblss SBezierLoopSetSet
	var poly SPolygon
	var allClosed, allCoplanar bool
	var notClosed SEdge
	var notCoplanar geom.Vector
	sblss.FindOuterFacesFrom(sbl, &poly, nil, Tol(0.1, 20), &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	return &sblss
}

// extrudedCurves is the solid of sbl's loops extruded from t0 to t1 at
// tol.
func extrudedCurves(sbl *SBezierList, t0, t1 geom.Vector, tol Tolerance) *SShell {
	sh := &SShell{Tol: tol}
	sblss := loopsOf(sbl)
	for i := range sblss.Sets {
		sh.MakeFromExtrusionOf(&sblss.Sets[i], t0, t1, core.RGBA{})
	}
	return sh
}

// TestIntersectionBranches runs a boolean through each of
// IntersectAgainst's branches that the fixtures don't reach, at a chord
// tolerance of 0.1. The result's surfaces and curves are the kernel's and
// are pinned; its mesh is checked closed, and its volume against the
// exact solid's within what chords of the tolerance can lose over the
// curved faces, so that either mesher passes (doc/dev-notes/k4-cdt-spike.md).
func TestIntersectionBranches(t *testing.T) {
	t.Parallel()
	tol := Tol(0.1, 20)
	x, y, z := geom.Vector{X: 1}, geom.Vector{Y: 1}, geom.Vector{Z: 1}
	coaxial := func() (*SShell, *SShell) {
		// Two parallel cylinders: their sides meet along lines.
		a := extrudedCurves(circleCurves(geom.Vector{}, x, y, 10), geom.Vector{}, geom.Vector{Z: 10}, tol)
		b := extrudedCurves(circleCurves(geom.Vector{X: 12}, x, y, 10), geom.Vector{Z: -5}, geom.Vector{Z: 15}, tol)
		return a, b
	}
	crossing := func() (*SShell, *SShell) {
		// Two perpendicular cylinders: the general, marching case.
		a := extrudedCurves(circleCurves(geom.Vector{Z: -20}, x, y, 10), geom.Vector{}, geom.Vector{Z: 40}, tol)
		b := extrudedCurves(circleCurves(geom.Vector{X: -20}, y, z, 6), geom.Vector{}, geom.Vector{X: 40}, tol)
		return a, b
	}
	seam := func() (*SShell, *SShell) {
		// A lathed tube, cut by a box whose face is the plane of the
		// tube's seams: the plane meets the revolved surfaces along
		// their trim curves.
		tube := &SShell{Tol: tol}
		sblss := loopsOf(polylineCurves(geom.Vector{X: 1}, geom.Vector{X: 2}, geom.Vector{X: 2, Z: 1}, geom.Vector{X: 1, Z: 1}))
		for i := range sblss.Sets {
			tube.MakeFromRevolutionOf(&sblss.Sets[i], geom.Vector{}, z, core.RGBA{}, noFaces{})
		}
		box := extrudedCurves(polylineCurves(geom.Vector{X: -3, Z: -1}, geom.Vector{X: 3, Z: -1},
			geom.Vector{X: 3, Y: 3, Z: -1}, geom.Vector{X: -3, Y: 3, Z: -1}), geom.Vector{}, geom.Vector{Z: 3}, tol)
		return tube, box
	}

	// The coaxial cylinders, r 10 with centres 12 apart, overlap in a
	// lens 10 high; the crossing ones, r 10 along z and r 6 along x, in
	// the solid whose sections across y are rectangles 2√(36-y²) by
	// 2√(100-y²).
	lens := 200*math.Acos(0.6) - 6*math.Sqrt(400-144)
	cross := simpson(func(y float64) float64 { return 4 * math.Sqrt(36-y*y) * math.Sqrt(100-y*y) }, -6, 6, 20000)
	for _, tc := range []struct {
		name     string
		shells   func() (*SShell, *SShell)
		op       SurfaceCombineAs
		vol      float64 // the exact solid's
		curved   float64 // the curved faces' area, at most
		surfaces int
		curves   int
	}{
		// Recorded; they change only when the kernel means them to. A
		// difference drops the cutter's faces wholly outside, with the
		// curves on them no trim uses (roadmap 13.3). In the seam's
		// difference 16 curves whose other face was dropped ran along
		// the same points as curves between two kept faces; the trims
		// on them move onto those, and they go (moveOrphanTrims).
		{"coaxial-union", coaxial, SurfaceUnion, 1000*math.Pi + 2000*math.Pi - 10*lens, 2 * math.Pi * 10 * 30, 12, 30},
		{"coaxial-difference", coaxial, SurfaceDiff, 1000*math.Pi - 10*lens, 2 * math.Pi * 10 * 20, 8, 19},
		{"crossing-union", crossing, SurfaceUnion, 4000*math.Pi + 1440*math.Pi - cross, 2 * math.Pi * (10 + 6) * 40, 12, 32},
		{"crossing-difference", crossing, SurfaceDiff, 4000*math.Pi - cross, 2 * math.Pi * (10 + 6) * 40, 10, 24},
		{"seam-difference", seam, SurfaceDiff, 1.5 * math.Pi, math.Pi * (1 + 2 + 3), 9, 20},
	} {
		a, b := tc.shells()
		var r SShell
		r.MakeFromBoolean(a, b, tc.op)
		var m SMesh
		r.TriangulateInto(&m)
		vol := m.CalculateVolume()
		// Each triangle lies within the chord tolerance of its surface,
		// so the mesh's volume within that times the curved area.
		allow := tol.ChordTolMm() * tc.curved
		if r.BooleanFailed || !meshIsSound(&r) || math.Abs(vol-tc.vol) > allow ||
			len(r.Surface) != tc.surfaces || len(r.Curve) != tc.curves {
			t.Errorf("%s: vol %.6f (exact %.6f, allowed %.3f) failed %v sound %v surfaces %d curves %d; want surfaces %d curves %d",
				tc.name, vol, tc.vol, allow, r.BooleanFailed, meshIsSound(&r), len(r.Surface), len(r.Curve),
				tc.surfaces, tc.curves)
		}
	}
}

// simpson integrates f over [a, b] by Simpson's rule on n (even) steps.
func simpson(f func(float64) float64, a, b float64, n int) float64 {
	h := (b - a) / float64(n)
	sum := f(a) + f(b)
	for i := 1; i < n; i++ {
		w := 2.0
		if i%2 == 1 {
			w = 4
		}
		sum += w * f(a+float64(i)*h)
	}
	return sum * h / 3
}

// TestIntersectAgainstTwoPlanes verifies that two intersecting
// planes produce at least one intersection curve.
func TestIntersectAgainstTwoPlanes(t *testing.T) {
	t.Parallel()
	s1 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	s2 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Z: 1})
	s1.NewH = s1.H
	s2.NewH = s2.H
	into := &SShell{}
	s1.IntersectAgainst(&s2, nil, nil, into)
	// The intersection of the XY plane (z=0) and XZ plane (y=0) is
	// the X axis. We expect at least one curve.
	if len(into.Curve) == 0 {
		t.Fatalf("expected at least 1 intersection curve, got 0")
	}
}

// TestAddExactIntersectionCurve verifies that AddExactIntersectionCurve
// appends a curve to the result shell.
func TestAddExactIntersectionCurve(t *testing.T) {
	t.Parallel()
	s1 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1})
	s2 := FromPlane(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Z: 1})
	s1.NewH = s1.H
	s2.NewH = s2.H
	into := &SShell{}
	sb := SBezierFrom2(geom.Vector{}, geom.Vector{X: 1})
	s1.AddExactIntersectionCurve(&sb, &s2, nil, nil, into)
	if len(into.Curve) != 1 {
		t.Fatalf("expected 1 curve, got %d", len(into.Curve))
	}
}
