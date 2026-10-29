package topo

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// apexPyramid is the pyramid on the square (0, 0) to (20, 20) to the
// apex (10, 10, 10), at the tolerance tol.
func apexPyramid(t *testing.T, tol srf.Tolerance) *srf.SShell {
	t.Helper()
	base := []geom.Vector{{}, {Y: 20}, {X: 20, Y: 20}, {X: 20}}
	apex := geom.Vector{X: 10, Y: 10, Z: 10}
	pyr := &srf.SShell{Tol: tol}
	faces := [][]geom.Vector{base}
	for i := range base {
		faces = append(faces, []geom.Vector{base[(i+1)%len(base)], base[i], apex})
	}
	for _, f := range faces {
		var l []srf.SBezier
		for i := range f {
			l = append(l, srf.SBezierFrom2(f[i], f[(i+1)%len(f)]))
		}
		s := srf.FromPlane(f[0].Minus(f[1].Minus(f[0])).Minus(f[len(f)-1].Minus(f[0])),
			f[len(f)-1].Minus(f[0]).ScaledBy(3), f[1].Minus(f[0]).ScaledBy(3))
		if _, err := AddFace(pyr, s, [][]srf.SBezier{l}); err != nil {
			t.Fatal(err)
		}
	}
	for i := range pyr.Surface {
		pyr.Surface[i].Reverse()
	}
	return pyr
}

// TestChamferConeAsWedges chamfers some of the edges at a pyramid's
// apex, the rest left sharp, and takes the same from the pyramid as the
// chamfers' wedges do, cut by a Boolean as a chamfer the edit can't
// make was before (the wedges of pkg/sketch's chamferWedge): the edit
// cuts the corner as the planes do.
func TestChamferConeAsWedges(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.002, 20)
	apex := geom.Vector{X: 10, Y: 10, Z: 10}
	bs := []geom.Vector{{}, {X: 20}, {X: 20, Y: 20}, {Y: 20}}
	for _, ds := range [][4]float64{{1, 0, 0, 0}, {1, 1, 0, 0}, {1, 0, 0.5, 0}, {1, 1, 1, 0}, {1, 0.5, 0, 0}, {0.5, 1, 1.5, 0}, {2, 0, 1, 0}, {0.5, 2, 0.5, 0}} {
		sh := apexPyramid(t, tol)
		tp, err := Build(sh)
		if err != nil {
			t.Fatal(err)
		}
		var cut srf.SShell
		var picks []FilletPick
		for i, d := range ds {
			if d == 0 {
				continue
			}
			picks = append(picks, FilletPick{A: apex, B: bs[i], R: d})
			e, err := tp.wholeEdge(apex, bs[i])
			if err != nil {
				t.Fatal(err)
			}
			fe, err := tp.filletEdgeOf(e, d, true)
			if err != nil {
				t.Fatal(err)
			}
			a, b := tp.Vertices[fe.v[0]].P, tp.Vertices[fe.v[1]].P
			p := a.Minus(fe.dir.ScaledBy(2 * d))
			tri := []geom.Vector{p.Plus(fe.u[0].ScaledBy(d)), p.Plus(fe.u[1].ScaledBy(d)), p.Minus(fe.u[0].ScaledBy(d).Plus(fe.u[1].ScaledBy(d)))}
			w := srf.SShell{Tol: tol}
			w.MakePrism(tri, geom.Vector{}, fe.dir.ScaledBy(b.Minus(a).Magnitude()+4*d), core.NewRGBA(1, 1, 1, 255))
			if len(cut.Surface) == 0 {
				cut = w
				continue
			}
			var u srf.SShell
			u.MakeFromUnionOf(&cut, &w)
			cut = u
		}
		var wedged srf.SShell
		wedged.MakeFromDifferenceOf(sh, &cut)
		var mw srf.SMesh
		wedged.TriangulateInto(&mw)

		if err := FilletPicks(sh, picks, 1, 1<<30, true); err != nil {
			t.Errorf("%v: %v", ds, err)
			continue
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if got, want := m.CalculateVolume(), mw.CalculateVolume(); math.Abs(got-want) > 1e-6 {
			t.Errorf("%v: %.9f mm³, the wedges' %.9f", ds, got, want)
		}
	}
}
