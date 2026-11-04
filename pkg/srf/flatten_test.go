package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A quadratic patch lying in a plane becomes a plane patch spanning its
// control points, its normal as it was; one bulging out of it stays.
func TestFlattenPlanarPatches(t *testing.T) {
	t.Parallel()
	flat := SSurface{DegM: 2, DegN: 2}
	bent := SSurface{DegM: 2, DegN: 2}
	for i := range 3 {
		for j := range 3 {
			p := geom.Vector{X: float64(i) + 0.3*float64(j*j), Y: float64(j), Z: 5}
			flat.Ctrl[i][j], flat.Weight[i][j] = p, 1
			bent.Ctrl[i][j], bent.Weight[i][j] = p.Plus(geom.Vector{Z: float64(i * j)}), 1
		}
	}
	n0 := flat.NormalAt(0.5, 0.5).WithMagnitude(1)
	sh := SShell{Surface: []SSurface{flat, bent}}
	sh.FlattenPlanarPatches()
	f := &sh.Surface[0]
	if f.DegM != 1 || f.DegN != 1 {
		t.Fatalf("the flat patch is of degree %d, %d, want 1, 1", f.DegM, f.DegN)
	}
	if n := f.NormalAt(0.5, 0.5).WithMagnitude(1); n.Minus(n0).Magnitude() > 1e-12 {
		t.Errorf("the plane's normal %v, want %v", n, n0)
	}
	for i := range 2 {
		for j := range 2 {
			if z := f.Ctrl[i][j].Z; math.Abs(z-5) > 1e-12 {
				t.Errorf("corner %d %d off the plane: z %g", i, j, z)
			}
		}
	}
	// Every control point of the patch lies within the plane patch.
	for i := range 3 {
		for j := range 3 {
			p := flat.Ctrl[i][j]
			lo, hi := f.Ctrl[0][0], f.Ctrl[1][1]
			if p.X < math.Min(lo.X, hi.X)-1e-9 || p.X > math.Max(lo.X, hi.X)+1e-9 ||
				p.Y < math.Min(lo.Y, hi.Y)-1e-9 || p.Y > math.Max(lo.Y, hi.Y)+1e-9 {
				t.Errorf("control point %v outside the plane patch %v to %v", p, lo, hi)
			}
		}
	}
	if b := &sh.Surface[1]; b.DegM != 2 || b.DegN != 2 {
		t.Errorf("the bent patch was flattened: degree %d, %d", b.DegM, b.DegN)
	}
}
