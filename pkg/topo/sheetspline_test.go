package topo_test

import (
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// TestGridSplineSmooth cuts smooth B-splines, rational and not, of
// knots spaced unevenly, into their Bezier patches and gives them,
// shuffled, to GridSpline: the surface it writes has the knots the
// B-spline had, no more (package 27.1), and is the same surface.
// Patches that meet at a crease keep their knot.
func TestGridSplineSmooth(t *testing.T) {
	r := rand.New(rand.NewSource(271))
	for trial := range 24 {
		du, dv := 1+r.Intn(4), 1+r.Intn(4)
		nu, nv := du+2+r.Intn(4), dv+2+r.Intn(4)
		s := lidSurface(r, du, dv, nu, nv, 1)
		rational := trial%2 == 1
		if rational {
			for i := range s.Weight {
				for j := range s.Weight[i] {
					s.Weight[i][j] = 0.5 + r.Float64()
				}
			}
		}
		crease := trial%3 == 2 && du >= 2 && nu-du-1 > 0
		ps, err := s.Patches()
		if err != nil {
			t.Fatal(err)
		}
		wantU, wantV := len(s.KnotsU), len(s.KnotsV)
		if crease {
			// The points beside the first joint along u lifted, in every
			// patch before it: a crease there, whose knot the writer
			// must keep at full multiplicity.
			for b := range ps[0] {
				for j := 0; j <= dv; j++ {
					ps[0][b].Ctrl[du-1][j].Z++
				}
			}
			wantU += du - 1
		}
		var flat []srf.SSurface
		for a := range ps {
			for b := range ps[a] {
				p := ps[a][b]
				p.H = core.HSSurface{V: uint32(len(flat) + 1)}
				flat = append(flat, p)
			}
		}
		r.Shuffle(len(flat), func(i, j int) { flat[i], flat[j] = flat[j], flat[i] })
		g, err := topo.GridSpline(flat)
		if err != nil {
			t.Fatalf("trial %d: %v", trial, err)
		}
		bs := g.Surface
		us, vs := s.Breaks()
		gu, gv := bs.Breaks()
		if len(gu) != len(us) || len(gv) != len(vs) {
			t.Fatalf("trial %d: %d by %d spans, want %d by %d", trial, len(gu)-1, len(gv)-1, len(us)-1, len(vs)-1)
		}
		if len(bs.KnotsU) != wantU || len(bs.KnotsV) != wantV {
			t.Errorf("trial %d (degree %d,%d rational %v crease %v): %d by %d knots, want %d by %d: %v",
				trial, du, dv, rational, crease, len(bs.KnotsU), len(bs.KnotsV), wantU, wantV, bs.KnotsU)
		}
		// The same surface: at each patch's corners and middle, by the
		// patches it was made of.
		for _, c := range g.Cells {
			var p *srf.SSurface
			for k := range flat {
				if flat[k].H == c.H {
					p = &flat[k]
				}
			}
			for _, uv := range [][2]float64{{0.5, 0.5}, {0.2, 0.7}} {
				q := p.PointAt(uv[0], uv[1])
				var u, v float64
				best := 1e9
				pp, _ := bs.Patches()
				for a := range pp {
					for b := range pp[a] {
						pp[a][b].ClosestPointToUV(q, &u, &v, true)
						best = min(best, pp[a][b].PointAt(u, v).Minus(q).Magnitude())
					}
				}
				if best > 1e-6 {
					t.Errorf("trial %d: a patch's point is %g off the surface", trial, best)
				}
			}
		}
	}
}
