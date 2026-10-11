package topo

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

func rbv(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }

// rbCylinder is a cylinder of radius rad along z from 0 to h, its side
// four quarter patches.
func rbCylinder(rad, h float64) *srf.SShell {
	var out []srf.SBezier
	for k := range 4 {
		a := float64(k) * math.Pi / 2
		b := a + math.Pi/2
		pa := rbv(rad*math.Cos(a), rad*math.Sin(a), 0)
		pb := rbv(rad*math.Cos(b), rad*math.Sin(b), 0)
		pm := rbv(rad*(math.Cos(a)+math.Cos(b)), rad*(math.Sin(a)+math.Sin(b)), 0)
		out = append(out, arc(pa, pm, pb, math.Sqrt2/2))
	}
	var sh srf.SShell
	var ls srf.SBezierLoopSet
	ls.Outer.Curves = out
	ls.Normal = rbv(0, 0, -1)
	sh.MakeFromExtrusionOf(&ls, rbv(0, 0, 0), rbv(0, 0, h), core.NewRGBA(100, 100, 100, 255))
	return &sh
}

// The fillet's surface lies on the envelope of the balls: every point of
// it r from the nearest centre along the edge, within the tolerance it
// is fitted to; for a branch joined to a pipe and for a hole cut into
// it.
func TestRollingBallOnEnvelope(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.003)
	for _, hole := range []bool{false, true} {
		var mainT, brT, u srf.SShell
		mainT.MakeFromTransformationOf(rbCylinder(10, 40), rbv(0, 0, -20), geom.QuaternionIdentity, 1)
		brT.MakeFromTransformationOf(rbCylinder(5, 25), rbv(0, 0, 0), geom.QuaternionFromAxisAngle(rbv(0, 1, 0), math.Pi/2), 1)
		if hole {
			u.MakeFromDifferenceOf(&mainT, &brT)
		} else {
			u.MakeFromUnionOf(&mainT, &brT)
		}
		tp, err := Build(&u)
		if err != nil {
			t.Fatal(err)
		}
		const r = 2.0
		rb, err := tp.newRollingBall(tp.curvedEdgeAt(rbv(math.Sqrt(91), 3, 4)), r, false)
		if err != nil {
			t.Fatal(err)
		}
		const n = 2000
		cs := make([]geom.Vector, n)
		for i := range cs {
			sec, err := rb.section(rb.length * float64(i) / n)
			if err != nil {
				t.Fatal(err)
			}
			cs[i] = sec.c
		}
		// The distance to the centres' polyline, less r.
		off := func(p geom.Vector) float64 {
			best := math.Inf(1)
			for i := range cs {
				a, b := cs[i], cs[(i+1)%n]
				ab := b.Minus(a)
				k := math.Max(0, math.Min(1, p.Minus(a).Dot(ab)/ab.Dot(ab)))
				best = math.Min(best, p.Minus(a.Plus(ab.ScaledBy(k))).Magnitude())
			}
			return best - r
		}
		sf, _, _, err := rb.blend()
		if err != nil {
			t.Fatal(err)
		}
		worst := 0.0
		for i := range 9 {
			for j := range 201 {
				worst = math.Max(worst, math.Abs(off(sf.PointAt(float64(i)/8, float64(j)/200))))
			}
		}
		// The centres' chords cut inside their curve by a little more.
		if allow := 3*rb.tol + 1e-4; worst > allow {
			t.Errorf("hole %t: the fillet strays %g from the balls, more than %g", hole, worst, allow)
		}
	}
}

// A mitre copied to the chain whose shared face is its other side runs
// the other way: the reversed curve's point at u is the curve's at
// 1 - u, its weights and its knots between the spans carried over.
func TestReversedCurve(t *testing.T) {
	t.Parallel()
	c := srf.BSplineCurve{Deg: 2, Knots: []float64{0, 0, 0, 0.3, 1, 1, 1},
		Ctrl:   []geom.Vector{rbv(0, 0, 0), rbv(1, 2, 0), rbv(3, 2, 1), rbv(4, 0, 2)},
		Weight: []float64{1, 0.7, 1.4, 1}}
	r := reversedCurve(c)
	for i := range 11 {
		u := float64(i) / 10
		if d := r.PointAt(u).Minus(c.PointAt(1 - u)).Magnitude(); d > 1e-12 {
			t.Errorf("at %g: %v, want %v", u, r.PointAt(u), c.PointAt(1-u))
		}
	}
}
