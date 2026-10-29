package topo

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The eased ramp leaves the ball's arc length at the mean pace, keeps
// the mean of the three as the even ramp's, ends where the even ramp
// does, and moves each one way all along, the slowest at a fifth of the
// mean as on the raised band's chamfer.
func TestRampSEased(t *testing.T) {
	t.Parallel()
	rb := &rollingBall{}
	rb.target[1] = 4
	rb.endS[1] = [3]float64{5.88, 22.2, 14}
	rb.easeRamp = true
	const n = 200
	prev := rb.rampS(1, 1)
	for i := range 3 {
		if prev[i] != 4 {
			t.Fatalf("at the ball: %v", prev)
		}
	}
	for j := n - 1; j >= 0; j-- {
		lam := float64(j) / n
		ss := rb.rampS(1, lam)
		mean := (ss[0] + ss[1] + ss[2]) / 3
		lin := (1-lam)*(5.88+22.2+14)/3 + lam*4
		if math.Abs(mean-lin) > 1e-9 {
			t.Fatalf("lam %g: mean %g, the even ramp's %g", lam, mean, lin)
		}
		for i := range 3 {
			if !(ss[i] > prev[i]) {
				t.Fatalf("lam %g: %d goes back, %g after %g", lam, i, ss[i], prev[i])
			}
		}
		if j == n-1 {
			// The first step: every foot at about the mean pace.
			for i := range 3 {
				if pace := (ss[i] - 4) / (mean - 4); math.Abs(pace-1) > 0.05 {
					t.Errorf("first step: %d at %g of the mean pace", i, pace)
				}
			}
		}
		prev = ss
	}
	for i := range 3 {
		if math.Abs(prev[i]-rb.endS[1][i]) > 1e-9 {
			t.Errorf("at the face: %v, want %v", prev, rb.endS[1])
		}
	}
}

// A skin's side that overshoots along itself and comes back runs back;
// one evenly along does not.
func TestFeetRunBack(t *testing.T) {
	t.Parallel()
	skin := func(x [4]float64) *srf.BSplineSurface {
		sf := &srf.BSplineSurface{DegU: 1, DegV: 3, KnotsU: []float64{0, 0, 1, 1},
			KnotsV: []float64{0, 0, 0, 0, 1, 1, 1, 1}}
		for i := range 2 {
			row, w := make([]geom.Vector, 4), make([]float64, 4)
			for j := range 4 {
				row[j], w[j] = geom.Vector{X: float64(j) / 3, Y: float64(i)}, 1
				if i == 0 {
					row[j].X = x[j]
				}
			}
			sf.Ctrl, sf.Weight = append(sf.Ctrl, row), append(sf.Weight, w)
		}
		return sf
	}
	if feetRunBack(skin([4]float64{0, 1.0 / 3, 2.0 / 3, 1}), []float64{0, 1}) {
		t.Error("an even skin runs back")
	}
	if !feetRunBack(skin([4]float64{0, 2, 0.99, 1}), []float64{0, 1}) {
		t.Error("a hooked side doesn't run back")
	}
}
