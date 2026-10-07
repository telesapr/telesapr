package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// boxLoopSet returns a square loop set in the z=z0 plane, wound
// clockwise about its +Z normal as C++ FindOuterFacesFrom leaves
// outer loops.
func boxLoopSet(x0, y0, x1, y1, z0 float64) *SBezierLoopSet {
	p := []geom.Vector{{X: x0, Y: y0, Z: z0}, {X: x0, Y: y1, Z: z0}, {X: x1, Y: y1, Z: z0}, {X: x1, Y: y0, Z: z0}}
	sbls := &SBezierLoopSet{Normal: geom.Vector{Z: 1}, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, SBezierFrom2(p[i], p[(i+1)%4]))
	}
	return sbls
}

func extrudedBox(lo, hi geom.Vector) *SShell {
	sh := &SShell{}
	sh.MakeFromExtrusionOf(boxLoopSet(lo.X, lo.Y, hi.X, hi.Y, 0),
		geom.Vector{Z: lo.Z}, geom.Vector{Z: hi.Z}, core.RGBA{})
	return sh
}

func shellVolume(sh *SShell) float64 {
	var m SMesh
	sh.TriangulateInto(&m)
	return m.CalculateVolume()
}

// TestBooleanTwoBoxesVolume runs the SShell-level boolean on two
// overlapping 2x2x2 boxes offset by (1,1,1): union 15, difference 7,
// intersection 1.
func TestBooleanTwoBoxesVolume(t *testing.T) {
	if v := shellVolume(extrudedBox(geom.Vector{}, geom.Vector{X: 2, Y: 2, Z: 2})); math.Abs(v-8) > 1e-9 {
		t.Fatalf("single box volume = %v, want 8", v)
	}
	cases := []struct {
		name string
		op   SurfaceCombineAs
		want float64
	}{
		{"union", SurfaceUnion, 15},
		{"difference", SurfaceDiff, 7},
		{"intersection", SurfaceInter, 1},
	}
	for _, c := range cases {
		a := extrudedBox(geom.Vector{}, geom.Vector{X: 2, Y: 2, Z: 2})
		b := extrudedBox(geom.Vector{X: 1, Y: 1, Z: 1}, geom.Vector{X: 3, Y: 3, Z: 3})
		var r SShell
		r.MakeFromBoolean(a, b, c.op)
		v := shellVolume(&r)
		if r.BooleanFailed || math.Abs(v-c.want) > 1e-9 {
			t.Errorf("%s: volume=%v want=%v failed=%v", c.name, v, c.want, r.BooleanFailed)
		}
	}
}
