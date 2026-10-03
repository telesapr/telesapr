package srf

import (
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// A curve's or patch's polynomial over another interval, within it or
// past its ends, is the same curve point for point.
func TestSub(t *testing.T) {
	arcs := quarterArcs(5, 1)
	c := arcs[1]
	c3 := SBezierFrom4(geom.Vector{}, geom.Vector{X: 1, Y: 2}, geom.Vector{X: 3, Y: -1, Z: 1}, geom.Vector{X: 4})
	for _, b := range []SBezier{c, c3} {
		for _, iv := range [][2]float64{{0, 1}, {0.2, 0.7}, {-0.1, 1.2}, {0.9, 0.1}} {
			s := b.Sub(iv[0], iv[1])
			for k := 0; k <= 10; k++ {
				x := float64(k) / 10
				want := b.PointAt(iv[0] + (iv[1]-iv[0])*x)
				if got := s.PointAt(x); !got.EqualsTol(want, 1e-9) {
					t.Fatalf("over %v at %g: %v, want %v", iv, x, got, want)
				}
			}
		}
	}
	// A patch: a quarter of a cylinder, rational one way.
	var p SSurface
	p.DegM, p.DegN = 2, 1
	for i := 0; i <= 2; i++ {
		for j := 0; j <= 1; j++ {
			p.Ctrl[i][j] = c.Ctrl[i].Plus(geom.Vector{Z: 3 * float64(j)})
			p.Weight[i][j] = c.Weight[i]
		}
	}
	s := p.Sub(-0.05, 1.1, 0.3, 1.5)
	for i := 0; i <= 8; i++ {
		for j := 0; j <= 8; j++ {
			x, y := float64(i)/8, float64(j)/8
			want := p.PointAt(-0.05+1.15*x, 0.3+1.2*y)
			if got := s.PointAt(x, y); !got.EqualsTol(want, 1e-9) {
				t.Fatalf("patch at %g, %g: %v, want %v", x, y, got, want)
			}
		}
	}
}
