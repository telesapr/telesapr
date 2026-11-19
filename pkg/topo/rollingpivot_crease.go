package topo

import (
	"math"
	"sort"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The crease a corner patch swings about (rollingpivot_corner.go). Where
// it is a line the ball's centre swings in planes square to it, each a
// radius from its point at a depth. Where it is curved, as where a
// pocket's wall of a cylinder lying on its side meets a flat one, a line
// through the vertex along its tangent leaves it by the sag, x²/2R at a
// depth x on a crease of radius R: a hundredth of a millimetre at a
// depth of 1 on a radius of 50, ten times the fillets' tolerance. So the
// swing is re-centred along the crease: at a depth h, measured along the
// crease's tangent at the vertex, its centre is the crease's own point
// there, and the frame the swing turns in lies square to the crease's
// tangent there. The ball then stands a radius off the crease, touching
// it, as at either end it touches both walls on the crease.

// pivotCrease is the crease of a corner: the vertex v and its tangent
// there, e, pointing down along it; and the two walls it is the edge
// of, nil where it is a line.
type pivotCrease struct {
	v, e   geom.Vector
	sa, sb *srf.SSurface
}

// pivotCreaseSteps is how many Newton steps a crease's point takes at
// most.
const pivotCreaseSteps = 40

// point is the crease's point at depth h, where the plane square to e
// at v + e·h meets it, and the crease's tangent there, pointing down
// along it.
func (pc *pivotCrease) point(h float64) (geom.Vector, geom.Vector) {
	p := pc.v.Plus(pc.e.ScaledBy(h))
	if pc.sa == nil {
		return p, pc.e
	}
	// Newton's steps onto the walls' two surfaces and the plane at once:
	// each wall taken as its tangent plane at the point's foot on it.
	var na, nb geom.Vector
	for range pivotCreaseSteps {
		ua, va, qa := nearestOnWithin(pc.sa, p, 0.5)
		ub, vb, qb := nearestOnWithin(pc.sb, p, 0.5)
		na, nb = pc.sa.NormalAt(ua, va), pc.sb.NormalAt(ub, vb)
		if na.Magnitude() == 0 || nb.Magnitude() == 0 {
			break
		}
		na, nb = na.WithMagnitude(1), nb.WithMagnitude(1)
		g1, g2, g3 := p.Minus(qa).Dot(na), p.Minus(qb).Dot(nb), p.Minus(pc.v).Dot(pc.e)-h
		det := na.Dot(nb.Cross(pc.e))
		if math.Abs(det) < 1e-12 {
			break
		}
		dp := nb.Cross(pc.e).ScaledBy(g1).Plus(pc.e.Cross(na).ScaledBy(g2)).Plus(na.Cross(nb).ScaledBy(g3)).ScaledBy(-1 / det)
		p = p.Plus(dp)
		if dp.Magnitude() < 1e-13 {
			break
		}
	}
	tan := na.Cross(nb)
	if tan.Magnitude() == 0 {
		return p, pc.e
	}
	if tan.Dot(pc.e) < 0 {
		tan = tan.ScaledBy(-1)
	}
	return p, tan.WithMagnitude(1)
}

// depth is p's depth along the crease, the parameter point takes.
func (pc *pivotCrease) depth(p geom.Vector) float64 {
	return p.Minus(pc.v).Dot(pc.e)
}

// on reports whether p lies on the crease, between depths lo and hi.
func (pc *pivotCrease) on(p geom.Vector, lo, hi float64) bool {
	h := pc.depth(p)
	if h < lo-10*geom.LengthEps || h > hi+10*geom.LengthEps {
		return false
	}
	q, _ := pc.point(h)
	return q.Minus(p).Magnitude() < 10*geom.LengthEps
}

// curved reports whether the corner's crease is curved.
func (pf *pivotFrame) curved() bool { return pf.crease.sa != nil }

// frameAt is the crease's point at depth h, its tangent there and the
// swing's frame: u0 turned square to the tangent, and w square to both.
func (pf *pivotFrame) frameAt(h float64) (q, tan, u, w geom.Vector) {
	q, tan = pf.crease.point(h)
	u = pf.u0.Minus(tan.ScaledBy(pf.u0.Dot(tan))).WithMagnitude(1)
	return q, tan, u, tan.Cross(u).ScaledBy(pf.wSign)
}

// wayAt is the crease's point at depth h and the direction phi of the
// way round there, square to the crease.
func (pf *pivotFrame) wayAt(phi, h float64) (geom.Vector, geom.Vector) {
	q, _, u, w := pf.frameAt(h)
	return q, u.ScaledBy(math.Cos(phi)).Plus(w.ScaledBy(math.Sin(phi)))
}

// curvedFrame sets the swing up about a curved crease: u0 square to the
// crease at a's foot, u1 at b's, and the turn theta between them
// measured in the frame at b's foot, where u0 has been carried along the
// crease.
func (pf *pivotFrame) curvedFrame() error {
	c := pf.c
	from, to := pf.secA.c, pf.secB.c
	if pf.flat {
		// A chamfer's frame turns from wall a's normal to wall b's, into
		// the solid, as a fillet's turns between its balls' centres. Its
		// sections from the crease to the shared face don't do: where the
		// crease leans far from square to that face, as where two walls
		// that both lean meet, their parts square to the crease may turn
		// the short way round through the pocket, and the corner's foot
		// on the shared face with them (seen at 167° on the dish's pocket
		// between two walls of cylinders lying on their sides).
		into := func(s *srf.SSurface, q, d geom.Vector) geom.Vector {
			u, v, _ := nearestOnWithin(s, q, 0.5)
			n := s.NormalAt(u, v)
			if n.Magnitude() == 0 {
				return n
			}
			n = n.WithMagnitude(pf.r)
			if n.Dot(d.Minus(q)) < 0 {
				n = n.ScaledBy(-1)
			}
			return n
		}
		from = c.qa.Plus(into(pf.crease.sa, c.qa, c.da))
		to = c.qb.Plus(into(pf.crease.sb, c.qb, c.db))
	}
	_, ta := pf.crease.point(pf.ha)
	_, tb := pf.crease.point(pf.hb)
	sq := func(x, tan geom.Vector) geom.Vector { return x.Minus(tan.ScaledBy(x.Dot(tan))) }
	u0, u1 := sq(from.Minus(c.qa), ta), sq(to.Minus(c.qb), tb)
	if u0.Magnitude() < 1e-9*pf.r || u1.Magnitude() < 1e-9*pf.r {
		return filletErr(pf.v, "the fillets don't stand off the crease at the inside corner")
	}
	pf.u0, pf.u1 = u0.WithMagnitude(1), u1.WithMagnitude(1)
	pf.wSign = 1
	_, _, ub, wb := pf.frameAt(pf.hb)
	if wb.Dot(pf.u1) < 0 {
		pf.wSign = -1
		wb = wb.ScaledBy(-1)
	}
	pf.theta = math.Atan2(pf.u1.Dot(wb), pf.u1.Dot(ub))
	if pf.theta < 1e-6 {
		return filletErr(pf.v, "the fillets meet at the inside corner without turning")
	}
	pf.w = pf.e.Cross(pf.u0).ScaledBy(pf.wSign)
	return nil
}

// phiAt is the turn at share t of the swing's path's length, about a
// curved crease: the samples' turns against their lengths, through a
// cubic between each two, its slopes at each sample those of the chords
// either side. Taken as straight between the samples, as about a line,
// the turn's rate jumps at each where the swing's speed changes along
// it, as a chamfer's does whose feet lie at two distances from the
// crease: its sections came out a few tenths of a micrometre off smooth
// there, and the corner wasn't fitted through 64 of them. Taken evenly
// in the turn instead, the fillet's corner where two leaning walls meet
// in the dish crossed the shallower fillet's mesh near the crease.
func (pf *pivotFrame) phiAt(t float64) float64 {
	path := pf.path
	x := t * pf.total
	j := min(max(sort.Search(len(path), func(i int) bool { return path[i].sigma >= x })-1, 0), len(path)-2)
	slope := func(i int) float64 {
		lo, hi := max(i-1, 0), min(i+1, len(path)-1)
		return (path[hi].phi - path[lo].phi) / (path[hi].sigma - path[lo].sigma)
	}
	a, b := path[j], path[j+1]
	h := b.sigma - a.sigma
	s := (x - a.sigma) / h
	h00, h10 := 2*s*s*s-3*s*s+1, s*s*s-2*s*s+s
	h01, h11 := -2*s*s*s+3*s*s, s*s*s-s*s
	return h00*a.phi + h10*h*slope(j) + h01*b.phi + h11*h*slope(j+1)
}

// pivotPolar is a point in the crease's frame at its depth h: its angle
// round from u, its distance from the crease and its offset along the
// crease's tangent.
type pivotPolar struct{ h, ang, dist, along float64 }

// polarOf is p in the crease's frame at its depth.
func (pf *pivotFrame) polarOf(p geom.Vector) pivotPolar {
	h := pf.crease.depth(p)
	q, tan, u, w := pf.frameAt(h)
	x := p.Minus(q)
	return pivotPolar{h, math.Atan2(x.Dot(w), x.Dot(u)), math.Hypot(x.Dot(u), x.Dot(w)), x.Dot(tan)}
}

// curvedChamferAt is swingAt for a chamfer about a curved crease: the
// shared face's point under the crease's, its depth, its angle in the
// crease's frame, its distance from the crease and its offset along the
// crease's tangent each moved evenly from a's foot to b's, so that at
// either end it is that chamfer's foot. The swing's path is measured on
// that point before it is put on the face (pivotSwing.c): the foot on
// the face can stand nearly still as the turn starts, where the point
// moves at its distance from the crease, and the turn against the foot's
// path, steep there, left the corner unfitted through 64 sections.
func (pf *pivotFrame) curvedChamferAt(phi float64) pivotSwing {
	rb, side := pf.near(phi)
	f := phi / pf.theta
	a, b := pf.polarOf(pf.c.da), pf.polarOf(pf.c.db)
	if b.ang < a.ang {
		// Round the way the frame turns, through the solid.
		b.ang += 2 * math.Pi
	}
	lerp := func(x, y float64) float64 { return x + (y-x)*f }
	h := lerp(a.h, b.h)
	ang := lerp(a.ang, b.ang)
	q, tan, u, w := pf.frameAt(h)
	d := u.ScaledBy(math.Cos(ang)).Plus(w.ScaledBy(math.Sin(ang)))
	x := q.Plus(d.ScaledBy(lerp(a.dist, b.dist))).Plus(tan.ScaledBy(lerp(a.along, b.along)))
	ft := rb.foot(side, x)
	return pivotSwing{phi: phi, c: x, d: ft.p, n: ft.n}
}
