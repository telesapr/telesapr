package srf

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// quarterArcs is a circle of radius r at height z as four rational
// quadratics.
func quarterArcs(r, z float64) []SBezier {
	var out []SBezier
	for k := 0; k < 4; k++ {
		a0 := float64(k) * math.Pi / 2
		a1 := a0 + math.Pi/2
		sb := SBezierFrom3(geom.Vector{X: r * math.Cos(a0), Y: r * math.Sin(a0), Z: z},
			geom.Vector{X: r * math.Sqrt2 * math.Cos(a0+math.Pi/4), Y: r * math.Sqrt2 * math.Sin(a0+math.Pi/4), Z: z},
			geom.Vector{X: r * math.Cos(a1), Y: r * math.Sin(a1), Z: z})
		sb.Weight[1] = math.Sqrt2 / 2
		out = append(out, sb)
	}
	return out
}

func TestChainCurve(t *testing.T) {
	chain := quarterArcs(5, 0)
	for _, deg := range []int{2, 3, 5} {
		c, err := ChainCurve(chain, deg)
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Ctrl) != 4*deg+1 || c.Ctrl[len(c.Ctrl)-1] != c.Ctrl[0] {
			t.Errorf("degree %d: %d control points, closed %v", deg, len(c.Ctrl), c.Ctrl[len(c.Ctrl)-1] == c.Ctrl[0])
		}
		for i := 0; i <= 100; i++ {
			u := float64(i) / 100
			piece := min(int(u*4), 3)
			want := chain[piece].PointAt(u*4 - float64(piece))
			if got := c.PointAt(u); !got.EqualsTol(want, 1e-9) {
				t.Fatalf("degree %d at %.2f: %v, want %v", deg, u, got, want)
			}
		}
	}
	if _, err := ChainCurve(nil, 2); err == nil {
		t.Error("a chain of nothing made a curve")
	}
	broken := append([]SBezier{SBezierFrom2(geom.Vector{X: 9}, geom.Vector{X: 10})}, chain[1:]...)
	if _, err := ChainCurve(broken, 2); err == nil {
		t.Error("a broken chain made a curve")
	}
	if _, err := ChainCurve(chain, 1); err == nil {
		t.Error("arcs went down to lines")
	}
}

// A skin goes through each of its sections at their parameters, and two
// sections make it ruled.
func TestSkin(t *testing.T) {
	for _, k := range []int{2, 3, 4, 6} {
		var secs []BSplineCurve
		var loops [][]SBezier
		for s := 0; s < k; s++ {
			l := quarterArcs(4+float64(s*s%5), 3*float64(s)+float64(s*s)/4)
			c, err := ChainCurve(l, 2)
			if err != nil {
				t.Fatal(err)
			}
			secs, loops = append(secs, c), append(loops, l)
		}
		bs, ts, err := Skin(secs)
		if err != nil {
			t.Fatal(err)
		}
		if bs.DegV != min(3, k-1) || len(ts) != k || ts[0] != 0 || ts[k-1] != 1 {
			t.Errorf("%d sections: degree %d across, parameters %v", k, bs.DegV, ts)
		}
		for s := range secs {
			for i := 0; i <= 40; i++ {
				u := float64(i) / 40
				if got, want := bs.PointAt(u, ts[s]), secs[s].PointAt(u); !got.EqualsTol(want, 1e-9) {
					t.Fatalf("%d sections: section %d at %.3f is %v, want %v", k, s, u, got, want)
				}
			}
		}
	}
	a, _ := ChainCurve(quarterArcs(5, 0), 2)
	b, _ := ChainCurve(quarterArcs(5, 0), 3)
	if _, _, err := Skin([]BSplineCurve{a, b}); err == nil {
		t.Error("sections of two degrees skinned")
	}
	if _, _, err := Skin([]BSplineCurve{a, a}); err == nil {
		t.Error("sections on each other skinned")
	}
}
