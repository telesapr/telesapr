package sketch_test

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// pathCurve is a cubic Bézier of the test's path, a line as one too.
type pathCurve [4]api.Vec

func (c pathCurve) at(t float64) api.Vec {
	s := 1 - t
	return c[0].ScaledBy(s * s * s).Plus(c[1].ScaledBy(3 * s * s * t)).Plus(c[2].ScaledBy(3 * s * t * t)).Plus(c[3].ScaledBy(t * t * t))
}

func (c pathCurve) tangent(t float64) api.Vec {
	s := 1 - t
	return c[1].Minus(c[0]).ScaledBy(3 * s * s).Plus(c[2].Minus(c[1]).ScaledBy(6 * s * t)).Plus(c[3].Minus(c[2]).ScaledBy(3 * t * t))
}

func pathLine(a, b api.Vec) pathCurve {
	d := b.Minus(a)
	return pathCurve{a, a.Plus(d.ScaledBy(1.0 / 3)), a.Plus(d.ScaledBy(2.0 / 3)), b}
}

// sweeps tells whether p lies in a disc of radius r square to the curve
// at one of its points: in the tube a circle swept along it makes, its
// ends flat.
func (c pathCurve) sweeps(p api.Vec, r float64) bool {
	const n = 300
	f := func(t float64) float64 { return p.Minus(c.at(t)).Dot(c.tangent(t)) }
	t0, f0 := 0.0, f(0)
	for i := 1; i <= n; i++ {
		t1 := float64(i) / n
		f1 := f(t1)
		if f0*f1 <= 0 {
			lo, hi := t0, t1
			for range 30 {
				if m := (lo + hi) / 2; f(m)*f0 <= 0 {
					hi = m
				} else {
					lo = m
				}
			}
			if p.Minus(c.at((lo+hi)/2)).Magnitude() <= r {
				return true
			}
		}
		t0, f0 = t1, f1
	}
	return false
}

// cubicCornerRoundVolume is the volume of a circle of radius 1 swept
// along cubicCornerPath, by sampling the box lo..hi at n points: in the
// tube along either piece, or in the corner's fill, where the profile at
// the corner carried on along the first piece's tangent overlaps the
// profile carried back along the second's, as far as the sweep carries
// them (cubicCornerCarry).
func cubicCornerRoundVolume(cubicFirst bool, corner, bend, lead, span float64, lo, hi api.Vec, n int) float64 {
	var first, second pathCurve
	if cubicFirst {
		y := api.V(0, 1, 0)
		b := bend * math.Pi / 180
		up := api.V(0, 0, 1)
		far := up.RotatedAroundAxis(y, b/2).ScaledBy(span)
		out := up.RotatedAroundAxis(y, b)
		first = pathCurve{api.V(0, 0, 0), up.ScaledBy(span / 3), far.Minus(out.ScaledBy(span / 3)), far}
		second = pathLine(far, far.Plus(out.RotatedAroundAxis(y, corner*math.Pi/180).ScaledBy(lead)))
	} else {
		p := cubicCornerPoints(corner, bend, lead, span)
		first = pathLine(p[0], p[1])
		second = pathCurve{p[1], p[2], p[3], p[4]}
	}
	x := first.at(1)
	t1, t2 := first.tangent(1).WithMagnitude(1), second.tangent(0).WithMagnitude(1)
	carry := 2 * (1 + math.Tan(math.Acos(t1.Dot(t2))/2))
	carried := func(p, t api.Vec, a, b float64) bool {
		q := p.Minus(x)
		s := q.Dot(t)
		return s >= a && s <= b && q.Minus(t.ScaledBy(s)).Magnitude() <= 1
	}
	rng := rand.New(rand.NewPCG(1, 2))
	in := 0
	for range n {
		p := api.V(lo.X+rng.Float64()*(hi.X-lo.X), lo.Y+rng.Float64()*(hi.Y-lo.Y), lo.Z+rng.Float64()*(hi.Z-lo.Z))
		if first.sweeps(p, 1) || second.sweeps(p, 1) || carried(p, t1, 0, carry) && carried(p, t2, -carry, 0) {
			in++
		}
	}
	return (hi.X - lo.X) * (hi.Y - lo.Y) * (hi.Z - lo.Z) * float64(in) / float64(n)
}

// A circle swept round a corner of 150° beside a cubic bent 90° the
// same way: the cubic comes back across the line, and the two runs'
// tubes, of one radius with axes crossing, touch tangent where the
// circle's sections there cross the line square to both. The sweep
// hands those points to its Booleans as it does the corner's
// (crossingTouches, srf.SShell.Touch); before 2026-11-09 every such
// corner was refused, 4 of the 60 at lead 10. Made sound, and its
// volume the union's: within 2% of the sampled one (its mesh short of it
// by its chords, 1%, and the sample's error 0.3%), where the runs
// counted twice where they overlap would be 29% over. One case per
// mirror pair, line first and cubic first.
func TestSweepSharpCornerBesideCubicComesBack(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("each sweep joins its runs by Booleans and its volume is sampled: a few seconds")
	}
	for _, cubicFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("cubic first %v", cubicFirst), func(t *testing.T) {
			t.Parallel()
			d := api.New()
			path := cubicCornerPath(d, cubicFirst, 150, 90, 10, 12)
			d.Features.SketchIn(d.Workplane("XY"), "profile")
			d.Sketch.Circle(api.V(0, 0, 0), 1)
			s := d.Features.Sweep(path, "")
			p, r := s.Problem(), s.Analyze().Check()
			if p != "" || !r.Sound() {
				t.Fatalf("%q %s, want made sound", p, r.String())
			}
			pad := api.V(0.05, 0.05, 0.05)
			want := cubicCornerRoundVolume(cubicFirst, 150, 90, 10, 12, r.Min.Minus(pad), r.Max.Plus(pad), 200000)
			if math.Abs(r.Volume-want) > 0.02*want {
				t.Errorf("volume %.4f, want %.4f within 2%%", r.Volume, want)
			}
		})
	}
}
