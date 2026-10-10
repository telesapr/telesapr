package check

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A drafted extrusion, built from its definition rather than from the
// kernel's faces: at a height z along the extrusion its section is the
// profile with each piece moved into the material by |z - neutral|
// times its angle's tangent, a line along its normal, an arc by its
// radius, and the moved pieces meeting where they cross nearest the
// corner they had. A point is inside when its section holds it; the
// sides are planes and cones, which a ray crosses at the roots of a
// linear or a quadratic equation.

// draftPiece is a line or an arc of a profile, in the profile's 2D
// coordinates, with the material on its left.
type draftPiece struct {
	a, b [2]float64
	arc  bool
	// An arc's centre, radius and turn from a to b, positive
	// counter-clockwise.
	c     [2]float64
	r     float64
	angle float64
	// tan is the side's lean: the piece moves into the material by tan
	// for each unit along the extrusion from the neutral plane.
	tan float64
}

// into is the direction into the material at p, a point of the piece.
func (p *draftPiece) into(at [2]float64) [2]float64 {
	if !p.arc {
		d := unit2(sub2(p.b, p.a))
		return [2]float64{-d[1], d[0]}
	}
	rad := unit2(sub2(at, p.c))
	if p.angle > 0 {
		return [2]float64{-rad[0], -rad[1]}
	}
	return rad
}

// tangent is the piece's direction of travel at p.
func (p *draftPiece) tangent(at [2]float64) [2]float64 {
	if !p.arc {
		return unit2(sub2(p.b, p.a))
	}
	rad := unit2(sub2(at, p.c))
	if p.angle > 0 {
		return [2]float64{-rad[1], rad[0]}
	}
	return [2]float64{rad[1], -rad[0]}
}

// radius is an arc's radius moved into the material by d.
func (p *draftPiece) radius(d float64) float64 {
	if p.angle > 0 {
		return p.r - d
	}
	return p.r + d
}

// sectionEdge is a piece of a section's outline: from p to q, along a
// line or round the circle about c of radius r, turning by sweep,
// counter-clockwise when positive.
type sectionEdge struct {
	p, q  [2]float64
	arc   bool
	c     [2]float64
	r     float64
	sweep float64
}

// draftedPrism is a profile's drafted extrusion, z from lo to hi along
// n from the profile's plane, leaning from the neutral plane at s0.
type draftedPrism struct {
	origin, eu, ev, n geom.Vector
	lo, hi, s0        float64
	loops             [][]draftPiece
	cornerLo          geom.Vector
	cornerHi          geom.Vector
}

// newDraftedPrism is the drafted extrusion of sbls along dir from lo to
// hi, curves leaning by angle radians (or upright where the group's
// draft faces leave them out), or false for a profile that isn't lines
// and arcs.
func newDraftedPrism(g *sketch.Group, sbls *srf.SBezierLoopSet, dir geom.Vector, lo, hi, angle float64) (*draftedPrism, bool) {
	n := dir.WithMagnitude(1)
	eu := n.NormalDir(0)
	dp := &draftedPrism{origin: sbls.Point, eu: eu, ev: n.Cross(eu), n: n, lo: lo, hi: hi, s0: g.DraftNeutral()}
	chosen := map[uint32]bool{}
	for _, h := range g.DraftFaces {
		chosen[h.V] = true
	}
	for k, l := range append([]srf.SBezierLoop{sbls.Outer}, sbls.Holes...) {
		var loop []draftPiece
		for i := range l.Curves {
			sb := &l.Curves[i]
			p, ok := dp.piece(sb)
			if !ok {
				return nil, false
			}
			if len(chosen) == 0 || chosen[sb.Sheet] {
				p.tan = math.Tan(angle)
			}
			loop = append(loop, p)
		}
		// The outer loop counter-clockwise, the holes clockwise.
		if (loopArea(loop) > 0) != (k == 0) {
			loop = reversedPieces(loop)
		}
		dp.loops = append(dp.loops, loop)
	}
	dp.findBounds()
	return dp, true
}

// to2 is x in the profile's plane's coordinates.
func (dp *draftedPrism) to2(x geom.Vector) [2]float64 {
	q := x.Minus(dp.origin)
	return [2]float64{q.Dot(dp.eu), q.Dot(dp.ev)}
}

// piece is curve sb as a line or an arc.
func (dp *draftedPrism) piece(sb *srf.SBezier) (draftPiece, bool) {
	a, m, b := dp.to2(sb.Start()), dp.to2(sb.PointAt(0.5)), dp.to2(sb.Finish())
	p := draftPiece{a: a, b: b}
	ab := sub2(b, a)
	if sb.Degree == 1 || math.Abs(cross2(ab, sub2(m, a))) <= 1e-9*dot2(ab, ab) {
		return p, true
	}
	// The circle through a, m and b.
	ma := sub2(m, a)
	den := 2 * cross2(ab, ma)
	if sb.Degree != 2 || den == 0 {
		return p, false
	}
	l1, l2 := dot2(ab, ab), dot2(ma, ma)
	p.c = [2]float64{a[0] + (ma[1]*l1-ab[1]*l2)/den, a[1] + (ab[0]*l2-ma[0]*l1)/den}
	p.arc, p.r = true, math.Hypot(a[0]-p.c[0], a[1]-p.c[1])
	if q := dp.to2(sb.PointAt(0.25)); math.Abs(math.Hypot(q[0]-p.c[0], q[1]-p.c[1])-p.r) > 1e-6*(1+p.r) {
		return p, false
	}
	ra, rb, rm := sub2(a, p.c), sub2(b, p.c), sub2(m, p.c)
	p.angle = math.Atan2(cross2(ra, rb), dot2(ra, rb))
	// The long way round when the middle lies the other side.
	if math.Signbit(cross2(ra, rm)) != math.Signbit(p.angle) {
		p.angle -= math.Copysign(2*math.Pi, p.angle)
	}
	return p, true
}

// loopArea is the loop's signed area, positive counter-clockwise: its
// pieces' chords, and each arc's segment beyond its chord.
func loopArea(loop []draftPiece) float64 {
	area := 0.0
	for _, p := range loop {
		area += cross2(p.a, p.b) / 2
		if p.arc {
			area += p.r * p.r * (p.angle - math.Sin(p.angle)) / 2
		}
	}
	return area
}

// reversedPieces is the loop run the other way.
func reversedPieces(loop []draftPiece) []draftPiece {
	out := make([]draftPiece, len(loop))
	for i, p := range loop {
		p.a, p.b, p.angle = p.b, p.a, -p.angle
		out[len(loop)-1-i] = p
	}
	return out
}

// section is the outline at height z, loop by loop.
func (dp *draftedPrism) section(z float64) [][]sectionEdge {
	w := math.Abs(z - dp.s0)
	out := make([][]sectionEdge, len(dp.loops))
	for k, loop := range dp.loops {
		corners := make([][2]float64, len(loop))
		for i := range loop {
			corners[i] = meetMovedPieces(&loop[(i+len(loop)-1)%len(loop)], &loop[i], w)
		}
		for i := range loop {
			p := &loop[i]
			e := sectionEdge{p: corners[i], q: corners[(i+1)%len(loop)]}
			if p.arc {
				e.arc, e.c, e.r = true, p.c, p.radius(w*p.tan)
				sweep := math.Atan2(e.q[1]-e.c[1], e.q[0]-e.c[0]) - math.Atan2(e.p[1]-e.c[1], e.p[0]-e.c[0])
				// The turn nearest the arc's own.
				for sweep < p.angle-math.Pi {
					sweep += 2 * math.Pi
				}
				for sweep > p.angle+math.Pi {
					sweep -= 2 * math.Pi
				}
				e.sweep = sweep
			}
			out[k] = append(out[k], e)
		}
	}
	return out
}

// meetMovedPieces is where p, then q, moved into the material by w
// times their leans, meet: where they cross nearest their corner, or,
// where they run on into one another, the corner moved along q's
// normal.
func meetMovedPieces(p, q *draftPiece, w float64) [2]float64 {
	v := q.a
	dp, dq := w*p.tan, w*q.tan
	tp, tq := p.tangent(v), q.tangent(v)
	moved := func(x *draftPiece, at [2]float64, d float64) [2]float64 {
		in := x.into(at)
		return [2]float64{at[0] + d*in[0], at[1] + d*in[1]}
	}
	if math.Abs(cross2(tp, tq)) < 1e-9 && dot2(tp, tq) > 0 {
		return moved(q, v, dq)
	}
	var cands [][2]float64
	switch {
	case !p.arc && !q.arc:
		pa, qa := moved(p, p.a, dp), moved(q, q.a, dq)
		dpv, dqv := sub2(p.b, p.a), sub2(q.b, q.a)
		if den := cross2(dpv, dqv); den != 0 {
			s := cross2(sub2(qa, pa), dqv) / den
			cands = append(cands, [2]float64{pa[0] + s*dpv[0], pa[1] + s*dpv[1]})
		}
	case p.arc && q.arc:
		cands = circleCircle2(p.c, p.radius(dp), q.c, q.radius(dq))
	case p.arc:
		cands = lineCircle2(moved(q, q.a, dq), sub2(q.b, q.a), p.c, p.radius(dp))
	default:
		cands = lineCircle2(moved(p, p.a, dp), sub2(p.b, p.a), q.c, q.radius(dq))
	}
	if len(cands) == 0 {
		// The kernel refuses such a draft; any point will do.
		return moved(q, v, dq)
	}
	best := cands[0]
	for _, c := range cands[1:] {
		if dist2(c, v) < dist2(best, v) {
			best = c
		}
	}
	return best
}

// lineCircle2 is where the line through p along d meets the circle.
func lineCircle2(p, d, c [2]float64, r float64) [][2]float64 {
	f := sub2(p, c)
	a, b, cc := dot2(d, d), 2*dot2(f, d), dot2(f, f)-r*r
	var out [][2]float64
	for _, t := range appendRoots(nil, a, b, cc) {
		out = append(out, [2]float64{p[0] + t*d[0], p[1] + t*d[1]})
	}
	return out
}

// circleCircle2 is where two circles meet.
func circleCircle2(c0 [2]float64, r0 float64, c1 [2]float64, r1 float64) [][2]float64 {
	d := sub2(c1, c0)
	l := math.Hypot(d[0], d[1])
	if l == 0 {
		return nil
	}
	a := (r0*r0 - r1*r1 + l*l) / (2 * l)
	h2 := r0*r0 - a*a
	if h2 < 0 {
		if h2 < -1e-12*r0*r0 {
			return nil
		}
		h2 = 0
	}
	h := math.Sqrt(h2)
	u := [2]float64{d[0] / l, d[1] / l}
	m := [2]float64{c0[0] + a*u[0], c0[1] + a*u[1]}
	return [][2]float64{{m[0] - h*u[1], m[1] + h*u[0]}, {m[0] + h*u[1], m[1] - h*u[0]}}
}

// sectionContains is the even-odd rule over a section's edges: a ray
// from (a, b) along +u crosses them an odd number of times. An arc
// crosses it as often as its chord does, give or take one when (a, b)
// lies between the two, inside the circle on the arc's side of the
// chord: that way the corners, chords' ends, count once each, however
// the ray meets them.
func sectionContains(sec [][]sectionEdge, a, b float64) bool {
	in := false
	x := [2]float64{a, b}
	for _, loop := range sec {
		for _, e := range loop {
			p, q := e.p, e.q
			if (p[1] > b) != (q[1] > b) && a < p[0]+(b-p[1])*(q[0]-p[0])/(q[1]-p[1]) {
				in = !in
			}
			if !e.arc || dist2(x, e.c) >= math.Abs(e.r) {
				continue
			}
			// A counter-clockwise arc bulges to the right of its chord.
			side := cross2(sub2(q, p), sub2(x, p))
			if side != 0 && (side < 0) == (e.sweep > 0) {
				in = !in
			}
		}
	}
	return in
}

// coords are x's height along the extrusion and place in the profile.
func (dp *draftedPrism) coords(x geom.Vector) (z, a, b float64) {
	q := x.Minus(dp.origin)
	return q.Dot(dp.n), q.Dot(dp.eu), q.Dot(dp.ev)
}

func (dp *draftedPrism) inside(x geom.Vector) bool {
	z, a, b := dp.coords(x)
	return z >= dp.lo && z <= dp.hi && sectionContains(dp.section(z), a, b)
}

func (dp *draftedPrism) crossings(o, d geom.Vector, out []float64) []float64 {
	z0, u0, v0 := dp.coords(o)
	dz, du, dv := d.Dot(dp.n), d.Dot(dp.eu), d.Dot(dp.ev)
	if dz != 0 {
		out = append(out, (dp.lo-z0)/dz, (dp.hi-z0)/dz)
	}
	// Each side leans one way above the neutral plane and the other
	// below: both, a superset of the crossings.
	for _, loop := range dp.loops {
		for i := range loop {
			p := &loop[i]
			for _, sign := range []float64{1, -1} {
				k := sign * p.tan
				if !p.arc {
					// (x - a) · into = k (z - s0)
					in := p.into(p.a)
					g0 := (u0-p.a[0])*in[0] + (v0-p.a[1])*in[1] - k*(z0-dp.s0)
					g1 := du*in[0] + dv*in[1] - k*dz
					if g1 != 0 {
						out = append(out, -g0/g1)
					}
				} else {
					// |x - c| = radius(k (z - s0)), its radius linear in s.
					uc, vc := u0-p.c[0], v0-p.c[1]
					r0 := p.radius(k * (z0 - dp.s0))
					rd := p.radius(k*dz) - p.r
					out = appendRoots(out, du*du+dv*dv-rd*rd, 2*(uc*du+vc*dv-r0*rd), uc*uc+vc*vc-r0*r0)
				}
				if p.tan == 0 {
					break
				}
			}
		}
	}
	return out
}

// findBounds boxes the sections at both ends and at the neutral plane:
// a corner where two lines meet moves straight between them, and every
// other lies on an arc's circle, whose box grows or shrinks straight.
func (dp *draftedPrism) findBounds() {
	dp.cornerLo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	dp.cornerHi = dp.cornerLo.ScaledBy(-1)
	add := func(z, a, b float64) {
		q := dp.origin.Plus(dp.n.ScaledBy(z)).Plus(dp.eu.ScaledBy(a)).Plus(dp.ev.ScaledBy(b))
		q.MakeMaxMin(&dp.cornerHi, &dp.cornerLo)
	}
	for _, z := range []float64{dp.lo, dp.hi, max(dp.lo, min(dp.hi, dp.s0))} {
		for _, loop := range dp.section(z) {
			for _, e := range loop {
				add(z, e.p[0], e.p[1])
				if e.arc {
					r := math.Abs(e.r)
					add(z, e.c[0]-r, e.c[1]-r)
					add(z, e.c[0]+r, e.c[1]+r)
				}
			}
		}
	}
}

func (dp *draftedPrism) bounds(lo, hi *geom.Vector) {
	dp.cornerLo.MakeMaxMin(hi, lo)
	dp.cornerHi.MakeMaxMin(hi, lo)
}

func sub2(a, b [2]float64) [2]float64 { return [2]float64{a[0] - b[0], a[1] - b[1]} }
func dot2(a, b [2]float64) float64    { return a[0]*b[0] + a[1]*b[1] }
func cross2(a, b [2]float64) float64  { return a[0]*b[1] - a[1]*b[0] }
func dist2(a, b [2]float64) float64   { return math.Hypot(a[0]-b[0], a[1]-b[1]) }
func unit2(a [2]float64) [2]float64 {
	l := math.Hypot(a[0], a[1])
	return [2]float64{a[0] / l, a[1] / l}
}
