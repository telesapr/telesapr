package srf

import (
	"math"
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// quarterCylinder is a quarter of a cylinder of radius r about the z
// axis, h high: rational quadratic round, straight up.
func quarterCylinder(r, h float64) SSurface {
	arc := quarterArcs(r, 0)[0]
	var p SSurface
	p.DegM, p.DegN = 2, 1
	for i := 0; i <= 2; i++ {
		for j := 0; j <= 1; j++ {
			p.Ctrl[i][j] = arc.Ctrl[i].Plus(geom.Vector{Z: h * float64(j)})
			p.Weight[i][j] = arc.Weight[i]
		}
	}
	return p
}

// An offset fitted within its tolerance: of a quarter cylinder, a
// cylinder of the radius moved by the offset; of a bicubic bump, points
// the offset's distance from the bump, wherever looked at.
func TestOffsetSurface(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	cyl := quarterCylinder(5, 10)
	// Which way the patch's normal points: the offset's radius.
	n := cyl.NormalAt(0.5, 0.5)
	out := cyl.PointAt(0.5, 0.5)
	sign := 1.0
	if n.Dot(geom.Vector{X: out.X, Y: out.Y}) < 0 {
		sign = -1
	}
	for _, d := range []float64{2, -2} {
		for _, tol := range []float64{1e-3, 1e-5} {
			bs, err := cyl.OffsetSurface(d, tol, 129)
			if err != nil {
				t.Fatal(err)
			}
			want := 5 + sign*d
			for range 400 {
				p := bs.PointAt(r.Float64(), r.Float64())
				if got := math.Hypot(p.X, p.Y); math.Abs(got-want) > 2*tol {
					t.Fatalf("offset %g within %g: a point %g from the axis, want %g", d, tol, got, want)
				}
			}
		}
	}
	var bump SSurface
	bump.DegM, bump.DegN = 3, 3
	for i := 0; i <= 3; i++ {
		for j := 0; j <= 3; j++ {
			bump.Ctrl[i][j] = geom.Vector{X: 10 * float64(i), Y: 10 * float64(j), Z: 6 * (r.Float64() - 0.5)}
			bump.Weight[i][j] = 1
		}
	}
	const d, tol = 1.5, 1e-4
	bs, err := bump.OffsetSurface(d, tol, 129)
	if err != nil {
		t.Fatal(err)
	}
	for range 200 {
		u, v := 0.05+0.9*r.Float64(), 0.05+0.9*r.Float64()
		p := bs.PointAt(u, v)
		// The nearest point of the bump to p, from where p came.
		cu, cv := u, v
		bump.ClosestPointToUV(p, &cu, &cv, true)
		if got := bump.PointAt(cu, cv).Minus(p).Magnitude(); math.Abs(got-d) > 2*tol {
			t.Fatalf("a point of the offset %g from the bump, want %g", got, d)
		}
	}
	if _, err := bump.OffsetSurface(d, 1e-12, 9); err == nil {
		t.Error("an offset within 1e-12 on a grid of 9")
	}
}

func BenchmarkOffsetSurface(b *testing.B) {
	cyl := quarterCylinder(5, 10)
	for b.Loop() {
		if _, err := cyl.OffsetSurface(2, 1e-4, 129); err != nil {
			b.Fatal(err)
		}
	}
}
