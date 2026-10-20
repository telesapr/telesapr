package srf

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// TestGuessGridClosestPointSame finds the closest points of random
// points on curved surfaces with the cache off, as a Boolean's loops
// do, with and without the guess grid, and requires them to the bit;
// and a copy of a surface changed since its grid was made to search as
// if it had none.
func TestGuessGridClosestPointSame(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(3, 4))
	k := math.Cos(math.Pi / 4)
	quarter := SBezier{Degree: 2,
		Ctrl:   [MaxDegree + 1]geom.Vector{{X: 1}, {X: 1, Y: 1}, {Y: 1}},
		Weight: [MaxDegree + 1]float64{1, k, 1}}
	cubic := SBezier{Degree: 3,
		Ctrl:   [MaxDegree + 1]geom.Vector{{}, {X: 1, Y: 2}, {X: 2, Y: -1}, {X: 3, Z: 1}},
		Weight: [MaxDegree + 1]float64{1, 1, 1, 1}}
	var sh SShell
	sh.Surface = []SSurface{
		FromExtrusionOf(&quarter, geom.Vector{}, geom.Vector{Z: 2}),
		FromExtrusionOf(&cubic, geom.Vector{}, geom.Vector{Y: 1, Z: 3}),
		FromRevolutionOf(&cubic, geom.Vector{}, geom.Vector{X: 1}, 0, math.Pi/2, 0, 0),
	}
	for i := range sh.Surface {
		sh.Surface[i].noCache = true
	}
	v := func() geom.Vector {
		return geom.Vector{X: rng.Float64()*4 - 1, Y: rng.Float64()*4 - 1, Z: rng.Float64()*4 - 1}
	}
	points := make([]geom.Vector, 200)
	for i := range points {
		points[i] = v()
	}
	closest := func() [][]geom.Point2d {
		out := make([][]geom.Point2d, len(sh.Surface))
		for i := range sh.Surface {
			for _, p := range points {
				var uv geom.Point2d
				sh.Surface[i].ClosestPointTo(p, &uv, true)
				out[i] = append(out[i], uv)
			}
		}
		return out
	}

	without := closest()
	makeGuessGrids(true, &sh)
	for i := range sh.Surface {
		if sh.Surface[i].guessPoints() == nil {
			t.Fatalf("surface %d has no grid", i)
		}
	}
	with := closest()
	for i := range without {
		for j := range without[i] {
			a, b := without[i][j], with[i][j]
			if math.Float64bits(a.U) != math.Float64bits(b.U) || math.Float64bits(a.V) != math.Float64bits(b.V) {
				t.Fatalf("surface %d, point %v: %v without the grid, %v with it", i, points[j], a, b)
			}
		}
	}

	moved := sh.Surface[1]
	moved.Ctrl[1][0] = moved.Ctrl[1][0].Plus(geom.Vector{Z: 1})
	if moved.guessPoints() != nil {
		t.Error("a surface changed since its grid was made still uses it")
	}
	// A cell serves the next loop of the Boolean, and goes with its
	// cleanup.
	cell := sh.Surface[0].guess
	makeGuessGrids(false, &sh)
	makeGuessGrids(true, &sh)
	if sh.Surface[0].guess != cell {
		t.Error("the grids were made again for the next loop")
	}
	sh.CleanupAfterBoolean()
	if sh.Surface[0].guess != nil || sh.Surface[0].rays != nil {
		t.Error("the cells were kept after the Boolean")
	}
}
