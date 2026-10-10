package export

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// TestPointCellsFirst holds the point index to the search it replaces:
// the first point of the list within stepPrecision, for points on and
// beside cell boundaries, repeated within the tolerance, and too far
// out for a cell.
func TestPointCellsFirst(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewPCG(5, 6))
	var points []stepPointAlias
	var pc pointCells
	jitter := func() float64 { return (rng.Float64()*2 - 1) * 1.5 * stepPrecision }
	near := func(v geom.Vector) geom.Vector {
		return geom.Vector{X: v.X + jitter(), Y: v.Y + jitter(), Z: v.Z + jitter()}
	}
	random := func() geom.Vector {
		switch rng.IntN(5) {
		case 0:
			// On a cell boundary.
			k := float64(rng.IntN(20))
			return geom.Vector{X: k * stepPrecision, Y: -k * stepPrecision, Z: 0}
		case 1:
			if len(points) > 0 {
				return near(points[rng.IntN(len(points))].v)
			}
		case 2:
			return geom.Vector{X: 1e30, Y: rng.Float64(), Z: math.Inf(1)}
		}
		return geom.Vector{X: float64(rng.IntN(5)) * 1e-5, Y: float64(rng.IntN(5)) * 1e-5, Z: rng.Float64() * 1e-5}
	}
	linear := func(v geom.Vector) int {
		for i := range points {
			if points[i].v.EqualsTol(v, stepPrecision) {
				return i
			}
		}
		return -1
	}
	for range 3000 {
		v := random()
		want := linear(v)
		if got := pc.first(points, v); got != want {
			t.Fatalf("point %v: index found %d, search %d", v, got, want)
		}
		if want < 0 {
			pc.add(v, len(points))
			points = append(points, stepPointAlias{v: v})
		}
	}
	if len(pc.far) == 0 || len(points) < 100 {
		t.Fatalf("%d points, %d far: the test reached too little", len(points), len(pc.far))
	}
}

func TestAliasIndexFirstEntry(t *testing.T) {
	t.Parallel()
	var x aliasIndex
	x.add(7, 5)
	x.add(7, 2)
	x.add(7, 9)
	if e, ok := x.find(7); !ok || e != 2 {
		t.Errorf("find(7) = %d, %v; want the first entry, 2", e, ok)
	}
	if _, ok := x.find(8); ok {
		t.Error("find(8) found an entry")
	}
}
