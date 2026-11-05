package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// TestDroppedSurfaceNamedByNone stacks two cylinders, joined where their
// caps meet and cancel, and carries the join through a Boolean of
// nothing, as a sketch's first group is, which numbers its surfaces
// afresh. A curve whose other surface a Boolean dropped (a side's edge
// along the cancelled cap) kept that surface's handle, which the next
// Boolean gave to another surface: the curve was then thinned against a
// patch it doesn't lie on, and along a two-sided draft's crease, at a
// steep draft, each side's curve differently, so that the meshes either
// side parted (pkg/topo's TestCreaseSeamAfterBoolean). Every curve that
// trims a surface must name only the surfaces it trims, or none.
func TestDroppedSurfaceNamedByNone(t *testing.T) {
	t.Parallel()
	tol := Tol(0.01, 20)
	circle := func() *SBezierList {
		return circleCurves(geom.Vector{}, geom.Vector{X: 1}, geom.Vector{Y: 1}, 6)
	}
	lo := extrudedCurves(circle(), geom.Vector{Z: -5}, geom.Vector{}, tol)
	hi := extrudedCurves(circle(), geom.Vector{}, geom.Vector{Z: 5}, tol)
	var joined, again SShell
	joined.MakeFromUnionOf(lo, hi)
	again.MakeFromUnionOf(&SShell{Tol: tol}, &joined)
	for _, sh := range []*SShell{&joined, &again} {
		if sh.BooleanFailed {
			t.Fatal("the Boolean failed")
		}
		checkCurveSurfaces(t, sh)
	}
}

// checkCurveSurfaces fails t where a curve of sh that trims a surface
// names one that it doesn't trim.
func checkCurveSurfaces(t *testing.T, sh *SShell) {
	t.Helper()
	by := map[core.HSCurve]map[core.HSSurface]bool{}
	for i := range sh.Surface {
		s := &sh.Surface[i]
		for _, tr := range s.Trim {
			if by[tr.Curve] == nil {
				by[tr.Curve] = map[core.HSSurface]bool{}
			}
			by[tr.Curve][s.H] = true
		}
	}
	for i := range sh.Curve {
		c := &sh.Curve[i]
		trims := by[c.H]
		if trims == nil {
			continue
		}
		for _, h := range []core.HSSurface{c.SurfA, c.SurfB} {
			if h.V != 0 && !trims[h] {
				t.Errorf("curve %v names surface %v, which it doesn't trim", c.H, h)
			}
		}
	}
}
