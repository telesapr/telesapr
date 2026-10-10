package srf

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/libm"
)

// helixRiseDegree is the degree along the turn of a helix's patches. A
// rational quadratic carries the circle exactly, but its angle is no
// linear function of its parameter, so a rise put in its three control
// points strays from the true helix by 2% of a quarter turn's rise (0.1
// mm on a helix rising 30 mm a turn, whatever the chord tolerance). The
// circle raised to degree 5 is the same circle, and its six rise values,
// fitted, keep within 3e-6 of the quarter turn's rise; degree 3 keeps
// within 1.5e-4, and 4 gains nothing on 3, the fit being symmetric.
const helixRiseDegree = 5

// helixRiseSamples is how many parameters along the turn the rise is
// fitted at.
const helixRiseSamples = 64

// withHelicalRise is s, a patch revolved through dtheta with no rise,
// raised to helixRiseDegree along the turn and lifted along axis by rise
// in proportion to the angle turned, as a helix rises.
func (s SSurface) withHelicalRise(axis geom.Vector, dtheta, rise float64) SSurface {
	up, err := s.ElevatedTo(s.DegM, helixRiseDegree)
	if err != nil {
		panic(err) // the degree is a constant within MaxDegree
	}
	var w [MaxDegree + 1]float64
	for j := 0; j <= up.DegN; j++ {
		w[j] = up.Weight[0][j] / up.Weight[0][0]
	}
	var c [MaxDegree + 1]float64
	if dtheta == 0 {
		for j := 0; j <= up.DegN; j++ {
			c[j] = float64(j) / float64(up.DegN) // no turn: a straight rise
		}
	} else {
		c = helixRiseFractions(w, up.DegN, dtheta)
	}
	for i := 0; i <= up.DegM; i++ {
		for j := 0; j <= up.DegN; j++ {
			up.Ctrl[i][j] = up.Ctrl[i][j].Plus(axis.ScaledBy(rise * c[j]))
		}
	}
	return up
}

// helixRiseFractions are the fractions of a patch's rise at its control
// points along the turn, for a circular arc of degree deg with weights w
// turning through dtheta: those whose rational blend follows the angle
// most closely in least squares. The arc is symmetric, so the fractions
// are too (c[deg-j] = 1-c[j]), which leaves at most two unknowns.
//
// The angle of the rational quadratic arc, and so of any raising of it,
// is dtheta/2 + 2 atan((2t-1) tan(dtheta/4)) at parameter t.
func helixRiseFractions(w [MaxDegree + 1]float64, deg int, dtheta float64) [MaxDegree + 1]float64 {
	var c [MaxDegree + 1]float64
	c[deg] = 1
	if deg%2 == 0 {
		c[deg/2] = 0.5
	}
	m := (deg - 1) / 2
	tq := libm.Tan(dtheta / 4)

	// The blend is sum_j b_j c_j / den. With c[deg-j] = 1-c[j], the
	// unknown c[j] (j <= m) takes b_j - b_{deg-j}, and the fixed part is
	// b_{deg-j} for each of them, b_deg, and half the middle one's at an
	// even degree. Least squares by the normal equations.
	var ata [2][2]float64
	var atb [2]float64
	for k := 1; k < helixRiseSamples; k++ {
		t := float64(k) / helixRiseSamples
		var b [MaxDegree + 1]float64
		den := 0.0
		for j := 0; j <= deg; j++ {
			b[j] = bernstein(j, deg, t) * w[j]
			den += b[j]
		}
		base := 0.0
		for j := deg - m; j <= deg; j++ {
			base += b[j]
		}
		if deg%2 == 0 {
			base += 0.5 * b[deg/2]
		}
		g := 0.5 + 2*libm.Atan((2*t-1)*tq)/dtheta
		var phi [2]float64
		for j := 1; j <= m; j++ {
			phi[j-1] = (b[j] - b[deg-j]) / den
		}
		r := g - base/den
		for a := range m {
			atb[a] += phi[a] * r
			for e := range m {
				ata[a][e] += phi[a] * phi[e]
			}
		}
	}
	x := solveSmall(ata, atb, m)
	for j := 1; j <= m; j++ {
		c[j] = x[j-1]
		c[deg-j] = 1 - x[j-1]
	}
	return c
}

// solveSmall solves the n by n (n at most 2) symmetric positive system
// a x = b.
func solveSmall(a [2][2]float64, b [2]float64, n int) [2]float64 {
	switch n {
	case 1:
		return [2]float64{b[0] / a[0][0]}
	case 2:
		det := a[0][0]*a[1][1] - a[0][1]*a[1][0]
		return [2]float64{
			(b[0]*a[1][1] - a[0][1]*b[1]) / det,
			(a[0][0]*b[1] - b[0]*a[1][0]) / det,
		}
	}
	return [2]float64{}
}
