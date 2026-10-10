package check

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// helix is a profile in a plane through an axis, turned about it while
// moved along it: at the angle th, from th0 to th1, its points are the
// profile's turned by th about w through c and moved d0 + slope (th -
// th0) along w. In (z, r), along the axis and from it, the profile is
// pr. A point at the angle phi about w from e, the profile's side of the
// axis, is a point of the profile at each th = phi + 2 pi k in the
// range, moved back along the axis.
type helix struct {
	pr                 profile // eu is w, ev is e
	c, w, e, f         geom.Vector
	th0, th1           float64
	d0, slope          float64
	zLo, zHi, rLo, rHi float64
	lo, hi             geom.Vector
	// detail is how far the ray's image in the profile's plane moves
	// between two samples of inside: a 64th of the profile's least
	// extent.
	detail float64
}

// helixStepsMost bounds the samples along one ray.
const helixStepsMost = 1 << 16

// newHelix is the helix of the loop set about the axis through c along
// w, from the angle th0, moved d0 along the axis, to th1, moved d1. It
// is false where the profile is not in a plane through the axis or
// crosses it.
func newHelix(sbls *srf.SBezierLoopSet, c, w geom.Vector, th0, th1, d0, d1 float64) (*helix, bool) {
	n := sbls.Normal.WithMagnitude(1)
	if math.Abs(n.Dot(w)) > 1e-9 || math.Abs(sbls.Point.Minus(c).Dot(n)) > 1e-6 {
		return nil, false
	}
	if th1 < th0 {
		th0, th1, d0, d1 = th1, th0, d1, d0
	}
	if th1 == th0 {
		return nil, false
	}
	e := n.Cross(w)
	// The profile's side of the axis.
	side := 0.0
	for _, l := range append([]srf.SBezierLoop{sbls.Outer}, sbls.Holes...) {
		for _, cv := range l.Curves {
			for k := 0; k <= cv.Degree; k++ {
				if r := cv.Ctrl[k].Minus(c).Dot(e); math.Abs(r) > math.Abs(side) {
					side = r
				}
			}
		}
	}
	if side < 0 {
		e = e.ScaledBy(-1)
	}
	hx := &helix{pr: newProfile(sbls, c, w, e), c: c, w: w, e: e, f: w.Cross(e),
		th0: th0, th1: th1, d0: d0, slope: (d1 - d0) / (th1 - th0)}
	hx.zLo, hx.rLo = math.Inf(1), math.Inf(1)
	hx.zHi, hx.rHi = math.Inf(-1), math.Inf(-1)
	for _, poly := range hx.pr.polys {
		for _, v := range poly {
			hx.zLo, hx.zHi = math.Min(hx.zLo, v[0]), math.Max(hx.zHi, v[0])
			hx.rLo, hx.rHi = math.Min(hx.rLo, v[1]), math.Max(hx.rHi, v[1])
		}
	}
	if hx.rLo < -1e-9 {
		return nil, false
	}
	hx.detail = math.Min(hx.zHi-hx.zLo, hx.rHi-hx.rLo) / 64
	hx.lo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hx.hi = hx.lo.ScaledBy(-1)
	for _, z := range []float64{hx.zLo + math.Min(d0, d1), hx.zHi + math.Max(d0, d1)} {
		for _, s := range []float64{-1, 1} {
			for _, t := range []float64{-1, 1} {
				p := c.Plus(w.ScaledBy(z)).Plus(e.ScaledBy(s * hx.rHi)).Plus(hx.f.ScaledBy(t * hx.rHi))
				p.MakeMaxMin(&hx.hi, &hx.lo)
			}
		}
	}
	return hx, true
}

func (hx *helix) inside(x geom.Vector) bool {
	q := x.Minus(hx.c)
	z := q.Dot(hx.w)
	rad := q.Minus(hx.w.ScaledBy(z))
	r := rad.Magnitude()
	if r < hx.rLo || r > hx.rHi {
		return false
	}
	phi := math.Atan2(rad.Dot(hx.f), rad.Dot(hx.e))
	for k := math.Ceil((hx.th0 - phi) / (2 * math.Pi)); ; k++ {
		th := phi + 2*math.Pi*k
		if th > hx.th1 {
			return false
		}
		zz := z - hx.d0 - hx.slope*(th-hx.th0)
		if zz >= hx.zLo && zz <= hx.zHi && hx.pr.contains(zz, r) {
			return true
		}
	}
}

// crossings samples inside along the ray across the helix's box and
// finds each change between two samples by bisection: no closed form
// gives where a line meets a helicoid. The ray's image in the profile's
// plane, (z, r) moved back to the profile for each turn, moves at the
// same speed on every turn; the samples are as far apart as moves it
// detail of the profile, so a stretch the ray runs inside is missed only
// where its image cuts across a corner of the profile less than that
// wide. The samples are crossings too, as far as Probe is concerned:
// what one misses then costs a step, not the stretch to the next
// crossing.
func (hx *helix) crossings(o, d geom.Vector, out []float64) []float64 {
	s0, s1, ok := boxSpan(hx.lo, hx.hi, o, d)
	if !ok {
		return out
	}
	// In the plane square to the axis the ray's offset from it is
	// rho0 + s rhod; along the axis z0 + s zd.
	q := o.Minus(hx.c)
	zd := d.Dot(hx.w)
	rho0 := q.Minus(hx.w.ScaledBy(q.Dot(hx.w)))
	rhod := d.Minus(hx.w.ScaledBy(zd))
	dm := d.Magnitude()
	step := func(s float64) float64 {
		rho := rho0.Plus(rhod.ScaledBy(s))
		rr := rho.Dot(rho)
		speed := dm
		if rr > 0 {
			dr := rho.Dot(rhod) / math.Sqrt(rr)
			dphi := rho.Cross(rhod).Dot(hx.w) / rr
			dz := zd - hx.slope*dphi
			speed = math.Max(speed, math.Hypot(dr, dz))
		}
		return hx.detail / speed
	}
	return sampledCrossings(hx.inside, o, d, s0, s1, step, out)
}

func (hx *helix) bounds(lo, hi *geom.Vector) {
	hx.lo.MakeMaxMin(hi, lo)
	hx.hi.MakeMaxMin(hi, lo)
}

// boxSpan is the stretch of the ray o + s d inside the box, a little
// grown.
func boxSpan(lo, hi, o, d geom.Vector) (s0, s1 float64, ok bool) {
	s0, s1 = math.Inf(-1), math.Inf(1)
	for k := range 3 {
		a, b := o.Element(k), d.Element(k)
		mn, mx := lo.Element(k)-1e-6, hi.Element(k)+1e-6
		if b == 0 {
			if a < mn || a > mx {
				return 0, 0, false
			}
			continue
		}
		t0, t1 := (mn-a)/b, (mx-a)/b
		s0, s1 = math.Max(s0, math.Min(t0, t1)), math.Min(s1, math.Max(t0, t1))
	}
	return s0, s1, s0 < s1
}

// sampledCrossings appends where inside changes along o + s d, s from
// s0 to s1, sampled every step(s) and bisected to the last bits, and
// the samples between.
func sampledCrossings(inside func(geom.Vector) bool, o, d geom.Vector, s0, s1 float64, step func(float64) float64, out []float64) []float64 {
	at := func(s float64) bool { return inside(o.Plus(d.ScaledBy(s))) }
	least := (s1 - s0) / helixStepsMost
	prevS, prevIn := s0, at(s0)
	for prevS < s1 {
		s := math.Min(s1, prevS+math.Max(step(prevS), least))
		in := at(s)
		if in != prevIn {
			a, b := prevS, s
			for range 64 {
				m := (a + b) / 2
				if m <= a || m >= b {
					break
				}
				if at(m) == prevIn {
					a = m
				} else {
					b = m
				}
			}
			out = append(out, (a+b)/2)
		}
		out = append(out, s)
		prevS, prevIn = s, in
	}
	return out
}
