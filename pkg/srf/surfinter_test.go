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
// tolerance of 0.1, and pins the result's volume and counts.
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

	for _, tc := range []struct {
		name      string
		shells    func() (*SShell, *SShell)
		op        SurfaceCombineAs
		vol       float64
		surfaces  int
		curves    int
		triangles int
	}{
		// Recorded; they change only when the kernel means them to. A
		// difference drops the cutter's faces wholly outside, with the
		// curves on them no trim uses (roadmap 13.3).
		{"coaxial-union", coaxial, SurfaceUnion, 8481.244959586, 12, 30, 268},
		{"coaxial-difference", coaxial, SurfaceDiff, 2237.031816644, 8, 19, 132},
		{"crossing-union", crossing, SurfaceUnion, 14851.712352551, 12, 32, 364},
		{"crossing-difference", crossing, SurfaceDiff, 10379.792222337, 10, 24, 232},
		{"seam-difference", seam, SurfaceDiff, 4.591632342, 9, 36, 76},
	} {
		a, b := tc.shells()
		var r SShell
		r.MakeFromBoolean(a, b, tc.op)
		var m SMesh
		r.TriangulateInto(&m)
		vol := m.CalculateVolume()
		if r.BooleanFailed || math.Abs(vol/tc.vol-1) > 1e-6 ||
			len(r.Surface) != tc.surfaces || len(r.Curve) != tc.curves || len(m.Triangles) != tc.triangles {
			t.Errorf("%s: vol %.9f failed %v surfaces %d curves %d triangles %d; want vol %.9f surfaces %d curves %d triangles %d",
				tc.name, vol, r.BooleanFailed, len(r.Surface), len(r.Curve), len(m.Triangles),
				tc.vol, tc.surfaces, tc.curves, tc.triangles)
		}
	}
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
