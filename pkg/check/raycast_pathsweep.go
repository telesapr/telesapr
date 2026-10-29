package check

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// pathSweep is a profile moved along a path of curves of any kind by
// frames of its own: the kernel fits the swept surface between sections
// (topo.MakePathSweep), and the oracle places the profile at every
// point of the path instead, so a fit that strays from the sweep shows.
// The frames turn with the path's tangent and no more (each sample's
// frame is the one before turned by the least rotation that takes its
// tangent onto the next, where the kernel reflects twice), or hold
// their second axis to a direction; a twist turns them about the
// tangent evenly by arc length, and round a closed path the turn they
// come back with is taken out the same way. A point is inside where
// the profile, moved back from one of the planes it is moved to through
// the point, holds it: the solid is the union of the moved profiles.
type pathSweep struct {
	pieces []srf.SBezier
	up     geom.Vector
	// The samples, framesPerPathPiece to a piece: the chain's parameter,
	// the point, its unit tangent, the untwisted second axis, the
	// twisted one and the arc length from the start.
	s      []float64
	p, t   []geom.Vector
	ru, r  []geom.Vector
	arc    []float64
	length float64
	twist  float64 // over the whole length, the closure's turn in it
	pr     profile
	// The path's start, where the profile is drawn, and its frame
	// there, b0 = t0 x r0.
	p0, t0, r0, b0 geom.Vector
	// normals are the profile's normal moved to each sample; cIn is the
	// offset of its plane from p0 along it.
	normals []geom.Vector
	cIn     float64
	// chunks are the samples, grouped by the balls holding them; reach
	// is the furthest point of the profile from p0.
	chunks []pathChunk
	reach  float64
	lo, hi geom.Vector
	// detail is a 64th of the profile's least extent; speedUp how much
	// faster than a point along a ray its image in the profile's plane
	// may move.
	detail, speedUp float64
}

// pathChunk is a run of samples, from lo to hi, inside the ball about c
// of radius rad grown by the profile's reach: a point outside it is on
// none of their moved profiles.
type pathChunk struct {
	c      geom.Vector
	rad    float64
	lo, hi int
	// spread is how far the samples are from c; moved and turned are the
	// lengths of the steps between them and the angles their profile's
	// normals turn by, summed: the height of a point over the moved
	// planes changes by no more than moved plus its distance from the
	// samples times turned across the chunk.
	spread, moved, turned float64
}

// framesPerPathPiece are the frames sampled on each piece of the path;
// between two, a root is refined with frames turned on from the one
// before.
const framesPerPathPiece = 256

// pathChunkSamples are the samples a chunk holds.
const pathChunkSamples = 16

// newPathSweep is the loop set swept along sp's curves, as sp's options
// say, by the oracle's own frames.
func newPathSweep(sbls *srf.SBezierLoopSet, sp *topo.SweepPath) *pathSweep {
	opts := sp.Options()
	ps := &pathSweep{pieces: sp.Pieces(), up: opts.Up}
	ps.p0 = ps.pieces[0].Start()
	ps.t0 = ps.tangent(0)
	r0 := ps.t0.NormalDir(0)
	if ps.up.Magnitude() > 0 {
		r0 = ps.up
	}
	ps.r0 = r0.Minus(ps.t0.ScaledBy(r0.Dot(ps.t0))).WithMagnitude(1)
	ps.b0 = ps.t0.Cross(ps.r0)

	n := len(ps.pieces) * framesPerPathPiece
	ps.s = append(ps.s, 0)
	ps.p = append(ps.p, ps.p0)
	ps.t = append(ps.t, ps.t0)
	ps.ru = append(ps.ru, ps.r0)
	ps.arc = append(ps.arc, 0)
	for k := 1; k <= n; k++ {
		s := float64(k) / framesPerPathPiece
		p, t, ru, arc := ps.turnOn(k-1, s)
		ps.s = append(ps.s, s)
		ps.p = append(ps.p, p)
		ps.t = append(ps.t, t)
		ps.ru = append(ps.ru, ru)
		ps.arc = append(ps.arc, arc)
	}
	ps.length = ps.arc[n]
	ps.twist = opts.Twist
	if opts.Closed {
		// The turn from the first frame's second axis to the last's,
		// about the tangent, taken back.
		r1 := ps.ru[n].Minus(ps.t0.ScaledBy(ps.ru[n].Dot(ps.t0)))
		ps.twist -= math.Atan2(ps.r0.Cross(r1).Dot(ps.t0), ps.r0.Dot(r1))
	}
	ps.r = make([]geom.Vector, len(ps.ru))
	for k := range ps.ru {
		ps.r[k] = ps.twisted(ps.t[k], ps.ru[k], ps.arc[k])
	}

	nrm := sbls.Normal.WithMagnitude(1)
	eu := nrm.NormalDir(0)
	ps.pr = newProfile(sbls, sbls.Point, eu, nrm.Cross(eu))
	ps.cIn = sbls.Point.Minus(ps.p0).Dot(nrm)
	ps.normals = make([]geom.Vector, len(ps.p))
	for k := range ps.p {
		ps.normals[k] = ps.moveDir(k, nrm)
	}

	// The profile's extent and its reach from p0.
	uLo, vLo, uHi, vHi := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, poly := range ps.pr.polys {
		for _, v := range poly {
			uLo, uHi = math.Min(uLo, v[0]), math.Max(uHi, v[0])
			vLo, vHi = math.Min(vLo, v[1]), math.Max(vHi, v[1])
			q := ps.pr.origin.Plus(eu.ScaledBy(v[0])).Plus(ps.pr.ev.ScaledBy(v[1]))
			ps.reach = math.Max(ps.reach, q.Minus(ps.p0).Magnitude())
		}
	}
	ps.detail = math.Min(uHi-uLo, vHi-vLo) / 64
	// The box holds the profile's points moved to every sample, and
	// between samples, where they run along curves, their sag: a quarter
	// of the most a point's second difference between samples comes to.
	ps.lo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	ps.hi = ps.lo.ScaledBy(-1)
	var verts []geom.Vector
	for _, poly := range ps.pr.polys {
		for _, v := range poly {
			verts = append(verts, ps.pr.origin.Plus(eu.ScaledBy(v[0])).Plus(ps.pr.ev.ScaledBy(v[1])))
		}
	}
	sag, margin, bend := 0.0, 0.0, 0.0
	for _, v := range verts {
		var m0, m1 geom.Vector
		for k := range ps.p {
			m := ps.moveTo(k, v)
			m.MakeMaxMin(&ps.hi, &ps.lo)
			if k > 1 {
				sag = math.Max(sag, m.Plus(m0).Minus(m1.ScaledBy(2)).Magnitude()/4)
			}
			m0, m1 = m1, m
		}
	}
	for k := 1; k < len(ps.p); k++ {
		step := ps.p[k].Minus(ps.p[k-1]).Magnitude()
		margin = math.Max(margin, step*(1+ps.reach))
		if step > 0 {
			bend = math.Max(bend, math.Acos(math.Max(-1, math.Min(1, ps.t[k].Dot(ps.t[k-1]))))/step)
		}
	}
	ps.lo = ps.lo.Minus(geom.Vector{X: sag, Y: sag, Z: sag})
	ps.hi = ps.hi.Plus(geom.Vector{X: sag, Y: sag, Z: sag})
	// A point moving along the ray moves its image in the profile's
	// plane faster where the planes turn about it: by 1/(1 - kappa rho)
	// inside a bend, and by the twist's turn times rho on top.
	turn := 0.0
	if ps.length > 0 {
		turn = math.Abs(ps.twist) / ps.length
	}
	ps.speedUp = (1 + ps.reach*turn) / math.Max(1.0/8, 1-ps.reach*bend)

	for lo := 0; lo < len(ps.p)-1; lo += pathChunkSamples {
		hi := min(lo+pathChunkSamples, len(ps.p)-1)
		c := ps.p[lo].Plus(ps.p[hi]).ScaledBy(0.5)
		rad := 0.0
		for k := lo; k <= hi; k++ {
			rad = math.Max(rad, ps.p[k].Minus(c).Magnitude())
		}
		ch := pathChunk{c: c, rad: rad + margin + ps.reach, lo: lo, hi: hi, spread: rad}
		for k := lo; k < hi; k++ {
			ch.moved += ps.p[k+1].Minus(ps.p[k]).Magnitude()
			ch.turned += ps.normals[k+1].Minus(ps.normals[k]).Magnitude()
		}
		ps.chunks = append(ps.chunks, ch)
	}
	return ps
}

// at is the piece and its own parameter at the chain's parameter s.
func (ps *pathSweep) at(s float64) (*srf.SBezier, float64) {
	i := max(min(int(math.Floor(s)), len(ps.pieces)-1), 0)
	return &ps.pieces[i], s - float64(i)
}

// tangent is the path's unit tangent at s, the limit inside where a
// control point lies on the piece's end.
func (ps *pathSweep) tangent(s float64) geom.Vector {
	b, u := ps.at(s)
	d := b.TangentAt(u)
	if d.Magnitude() < 1e-12 {
		const e = 1e-6
		if u < 0.5 {
			d = b.PointAt(u + e).Minus(b.PointAt(u))
		} else {
			d = b.PointAt(u).Minus(b.PointAt(u - e))
		}
	}
	return d.WithMagnitude(1)
}

// arcBetween is the path's length from sa to sb on one piece, by
// three-point Gauss-Legendre quadrature of the speed.
func (ps *pathSweep) arcBetween(sa, sb float64) float64 {
	b, ua := ps.at(sa)
	ub := ua + (sb - sa)
	h := (ub - ua) / 2
	m := (ua + ub) / 2
	g := math.Sqrt(0.6)
	return h * (5*b.TangentAt(m-h*g).Magnitude() + 8*b.TangentAt(m).Magnitude() + 5*b.TangentAt(m+h*g).Magnitude()) / 9
}

// turnOn is the point, unit tangent, untwisted second axis and arc
// length at s, turned on from sample k at or before it.
func (ps *pathSweep) turnOn(k int, s float64) (p, t, ru geom.Vector, arc float64) {
	if s == ps.s[k] {
		return ps.p[k], ps.t[k], ps.ru[k], ps.arc[k]
	}
	b, u := ps.at(s)
	p, t = b.PointAt(u), ps.tangent(s)
	if k+1 < len(ps.arc) {
		// Between two samples the speed hardly changes: the arc length
		// is the samples' in proportion.
		arc = ps.arc[k] + (ps.arc[k+1]-ps.arc[k])*(s-ps.s[k])/(ps.s[k+1]-ps.s[k])
	} else {
		// s is on the sample's piece: pieces start at samples.
		arc = ps.arc[k] + ps.arcBetween(ps.s[k], s)
	}
	if ps.up.Magnitude() > 0 {
		return p, t, ps.up.Minus(t.ScaledBy(ps.up.Dot(t))).WithMagnitude(1), arc
	}
	// The least rotation taking the sample's tangent onto t.
	ta, r := ps.t[k], ps.ru[k]
	axis, c := ta.Cross(t), ta.Dot(t)
	if c > -1+1e-12 {
		r = r.ScaledBy(c).Plus(axis.Cross(r)).Plus(axis.ScaledBy(axis.Dot(r) / (1 + c)))
	}
	return p, t, r.Minus(t.ScaledBy(r.Dot(t))).WithMagnitude(1), arc
}

// twisted is the second axis ru turned about t by the twist at arc.
func (ps *pathSweep) twisted(t, ru geom.Vector, arc float64) geom.Vector {
	if ps.twist == 0 || ps.length == 0 {
		return ru
	}
	a := ps.twist * arc / ps.length
	return ru.ScaledBy(math.Cos(a)).Plus(t.Cross(ru).ScaledBy(math.Sin(a)))
}

// moveTo is q moved from the path's start to sample k.
func (ps *pathSweep) moveTo(k int, q geom.Vector) geom.Vector {
	d := q.Minus(ps.p0)
	b := ps.t[k].Cross(ps.r[k])
	return ps.p[k].Plus(ps.t[k].ScaledBy(d.Dot(ps.t0))).Plus(ps.r[k].ScaledBy(d.Dot(ps.r0))).Plus(b.ScaledBy(d.Dot(ps.b0)))
}

// moveDir is the direction v turned from the path's start to sample k.
func (ps *pathSweep) moveDir(k int, v geom.Vector) geom.Vector {
	b := ps.t[k].Cross(ps.r[k])
	return ps.t[k].ScaledBy(v.Dot(ps.t0)).Plus(ps.r[k].ScaledBy(v.Dot(ps.r0))).Plus(b.ScaledBy(v.Dot(ps.b0)))
}

// back is x moved back from the frame (p, t, r) to the path's start.
func (ps *pathSweep) back(x, p, t, r geom.Vector) geom.Vector {
	d := x.Minus(p)
	return ps.p0.Plus(ps.t0.ScaledBy(d.Dot(t))).Plus(ps.r0.ScaledBy(d.Dot(r))).Plus(ps.b0.ScaledBy(d.Dot(t.Cross(r))))
}

// height is x's height over the profile's plane moved to sample k.
func (ps *pathSweep) height(k int, x geom.Vector) float64 {
	return x.Minus(ps.p[k]).Dot(ps.normals[k]) - ps.cIn
}

// heightAt is x moved back from the frame at s, between samples k and
// k+1, and its height over the profile's plane.
func (ps *pathSweep) heightAt(k int, s float64, x geom.Vector) (geom.Vector, float64) {
	p, t, ru, arc := ps.turnOn(k, s)
	q := ps.back(x, p, t, ps.twisted(t, ru, arc))
	return q, q.Minus(ps.pr.origin).Dot(ps.pr.n)
}

func (ps *pathSweep) inside(x geom.Vector) bool {
	for _, ch := range ps.chunks {
		far := x.Minus(ch.c).Magnitude()
		if far > ch.rad {
			continue
		}
		hb := ps.height(ch.lo, x)
		if math.Abs(hb) > ch.moved+(far+ch.spread)*ch.turned {
			// Every sample's plane is on the same side.
			continue
		}
		for k := ch.lo; k < ch.hi; k++ {
			ha := hb
			hb = ps.height(k+1, x)
			if ha != 0 && (ha > 0) == (hb > 0) {
				continue
			}
			// A root of the height between the samples, by regula
			// falsi on frames turned on from sample k.
			sa, sb := ps.s[k], ps.s[k+1]
			fa, fb := ha, hb
			var q geom.Vector
			for range 4 {
				s := sa
				if fb != fa {
					s = sa - fa*(sb-sa)/(fb-fa)
				}
				var h float64
				q, h = ps.heightAt(k, s, x)
				if math.Abs(h) < 1e-10 {
					break
				}
				if (h > 0) == (fa > 0) {
					sa, fa = s, h
				} else {
					sb, fb = s, h
				}
			}
			d := q.Minus(ps.pr.origin)
			if ps.pr.contains(d.Dot(ps.pr.eu), d.Dot(ps.pr.ev)) {
				return true
			}
		}
	}
	return false
}

// crossings samples inside along the ray where it passes through the
// chunks' balls, as a helix's are: no closed form gives where a line
// meets a swept surface, and outside every ball the ray is outside. The
// samples are as far apart as moves the ray's image in the profile's
// plane a detail of the profile.
func (ps *pathSweep) crossings(o, d geom.Vector, out []float64) []float64 {
	s0, s1, ok := boxSpan(ps.lo, ps.hi, o, d)
	if !ok {
		return out
	}
	// Where the ray is within each ball, merged.
	var spans [][2]float64
	dd := d.Dot(d)
	for _, ch := range ps.chunks {
		q := o.Minus(ch.c)
		b := q.Dot(d) / dd
		disc := b*b - (q.Dot(q)-ch.rad*ch.rad)/dd
		if disc <= 0 {
			continue
		}
		r := math.Sqrt(disc)
		if a, z := math.Max(s0, -b-r), math.Min(s1, -b+r); a < z {
			spans = append(spans, [2]float64{a, z})
		}
	}
	slices.SortFunc(spans, func(x, y [2]float64) int { return cmp.Compare(x[0], y[0]) })
	step := ps.detail / (math.Sqrt(dd) * ps.speedUp)
	for i := 0; i < len(spans); {
		a, z := spans[i][0], spans[i][1]
		for i++; i < len(spans) && spans[i][0] <= z; i++ {
			z = math.Max(z, spans[i][1])
		}
		out = append(out, a)
		out = sampledCrossings(ps.inside, o, d, a, z, func(float64) float64 { return step }, out)
	}
	return out
}

func (ps *pathSweep) bounds(lo, hi *geom.Vector) {
	ps.lo.MakeMaxMin(hi, lo)
	ps.hi.MakeMaxMin(hi, lo)
}
