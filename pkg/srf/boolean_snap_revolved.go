package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// cone is a patch's conical side: a ruled surface between two coaxial
// circles of different radii, its axis through c along unit dir, its
// radius r0 at c and r0 + k·h at h along dir, over heights hmin to hmax.
type cone struct {
	c, dir     geom.Vector
	r0, k      float64
	hmin, hmax float64
	pmax, pmin geom.Vector
}

// radiusAt is the cone's radius at height h along its axis.
func (cn *cone) radiusAt(h float64) float64 { return cn.r0 + cn.k*h }

// arcOf is the circular arc of a patch's control points p and weights w
// in order, as a curve.
func arcOf(p [3]geom.Vector, w [3]float64) SBezier {
	var b SBezier
	b.Degree = 2
	for i := range 3 {
		b.Ctrl[i], b.Weight[i] = p[i], w[i]
	}
	return b
}

// conesOf are sh's patches that are cones, their boxes grown by grow.
func conesOf(sh *SShell, grow geom.Vector) []cone {
	var cs []cone
	for i := range sh.Surface {
		s := &sh.Surface[i]
		var e0, e1 SBezier
		switch {
		case s.DegM == 1 && s.DegN == 2:
			e0 = arcOf([3]geom.Vector{s.Ctrl[0][0], s.Ctrl[0][1], s.Ctrl[0][2]}, [3]float64{s.Weight[0][0], s.Weight[0][1], s.Weight[0][2]})
			e1 = arcOf([3]geom.Vector{s.Ctrl[1][0], s.Ctrl[1][1], s.Ctrl[1][2]}, [3]float64{s.Weight[1][0], s.Weight[1][1], s.Weight[1][2]})
		case s.DegM == 2 && s.DegN == 1:
			e0 = arcOf([3]geom.Vector{s.Ctrl[0][0], s.Ctrl[1][0], s.Ctrl[2][0]}, [3]float64{s.Weight[0][0], s.Weight[1][0], s.Weight[2][0]})
			e1 = arcOf([3]geom.Vector{s.Ctrl[0][1], s.Ctrl[1][1], s.Ctrl[2][1]}, [3]float64{s.Weight[0][1], s.Weight[1][1], s.Weight[2][1]})
		default:
			continue
		}
		n := e0.Ctrl[0].Minus(e0.Ctrl[1]).Cross(e0.Ctrl[2].Minus(e0.Ctrl[1]))
		if n.Magnitude() < lengthEps {
			continue
		}
		n = n.WithMagnitude(1)
		var c0, c1 geom.Vector
		var r0, r1 float64
		if !e0.IsCircle(n, &c0, &r0) || !e1.IsCircle(n, &c1, &r1) {
			continue
		}
		h := c1.Minus(c0)
		if h.Magnitude() < lengthEps || h.WithMagnitude(1).Cross(n).Magnitude() > snapPlaneFloor ||
			math.Abs(r1-r0) < lengthEps {
			continue // a flat ring or a cylinder
		}
		dir := h.WithMagnitude(1)
		hh := h.Magnitude()
		var smax, smin geom.Vector
		s.AxisAlignedBounding(&smax, &smin)
		cs = append(cs, cone{c0, dir, r0, (r1 - r0) / hh, 0, hh, smax.Plus(grow), smin.Minus(grow)})
	}
	return cs
}

// snapToCones is snapToCylinders for cones: a conical side of b on the
// axis of one of a's within delta, of its angle, and off it by more than
// snapPlaneFloor and less than delta square to it, is scaled about the
// axis, at each height by as much as takes it onto a's, and with it
// every point of b over that side's heights between a little inside its
// radius there and a little past its arcs' middle control points. A cone
// a hair larger or smaller than a countersink of its angle then fills
// it, as a pin does a bore.
func snapToCones(a, b *SShell) *SShell {
	delta := a.Tol.Or(b.Tol).ChordTolMm() / snapPlaneDiv
	if delta <= snapPlaneFloor {
		return b
	}
	grow := geom.Vector{X: delta, Y: delta, Z: delta}
	type match struct {
		a          cone
		off        float64 // b's radius less a's, at any height
		hmin, hmax float64 // b's side's heights on a's axis
	}
	var ms []match
	bs := conesOf(b, grow)
	for _, ca := range conesOf(a, grow) {
		for _, cb := range bs {
			if ca.pmax.BoundingBoxesDisjoint(ca.pmax, ca.pmin, cb.pmax, cb.pmin) {
				continue
			}
			if ca.dir.Cross(cb.dir).Magnitude() > snapPlaneFloor {
				continue
			}
			if cb.c.DistanceToLine(ca.c, ca.dir) > delta {
				continue
			}
			// b's side in a's frame: its heights, and its slope the same
			// way up.
			h0 := cb.c.Minus(ca.c).Dot(ca.dir)
			h1 := cb.c.Plus(cb.dir.ScaledBy(cb.hmax)).Minus(ca.c).Dot(ca.dir)
			r1 := cb.radiusAt(cb.hmax)
			kb := (r1 - cb.r0) / (h1 - h0)
			if math.Abs(kb-ca.k) > snapPlaneFloor*(1+math.Abs(ca.k)) {
				continue
			}
			off := cb.r0 - ca.radiusAt(h0)
			if d := math.Abs(off) / math.Sqrt(1+ca.k*ca.k); d > snapPlaneFloor && d < delta {
				ms = append(ms, match{ca, off, min(h0, h1), max(h0, h1)})
			}
		}
	}
	if len(ms) == 0 {
		return b
	}
	changed := false
	snap := func(p geom.Vector) geom.Vector {
		for _, m := range ms {
			h := p.Minus(m.a.c).Dot(m.a.dir)
			if h < m.hmin-2*delta || h > m.hmax+2*delta {
				continue
			}
			ra := m.a.radiusAt(h)
			rb := ra + m.off
			if rb <= lengthEps || ra <= lengthEps {
				continue
			}
			foot := m.a.c.Plus(m.a.dir.ScaledBy(h))
			rad := p.Minus(foot)
			d := rad.Magnitude()
			if d < rb-2*delta || d > rb*math.Sqrt2+2*delta {
				continue
			}
			if q := foot.Plus(rad.ScaledBy(ra / rb)); q != p {
				p, changed = q, true
			}
			break
		}
		return p
	}
	out := b.Clone()
	movePoints(&out, snap)
	if !changed {
		return b
	}
	return &out
}

// sphere is a patch that lies on a sphere about c of radius r.
type sphere struct {
	c          geom.Vector
	r          float64
	pmax, pmin geom.Vector
}

// spheresOf are sh's patches that lie on spheres, their boxes grown by
// grow: curved both ways, and every point of a grid over them as far
// from the centre that four of them give.
func spheresOf(sh *SShell, grow geom.Vector) []sphere {
	var ss []sphere
	for i := range sh.Surface {
		s := &sh.Surface[i]
		if s.DegM < 2 || s.DegN < 2 {
			continue
		}
		p0, p1 := s.PointAt(0.2, 0.2), s.PointAt(0.8, 0.2)
		p2, p3 := s.PointAt(0.2, 0.8), s.PointAt(0.7, 0.6)
		plane := func(q geom.Vector) (geom.Vector, float64) {
			n := q.Minus(p0).ScaledBy(2)
			return n, q.Dot(q) - p0.Dot(p0)
		}
		n1, d1 := plane(p1)
		n2, d2 := plane(p2)
		n3, d3 := plane(p3)
		c, parallel := geom.AtIntersectionOfThreePlanes(n1, d1, n2, d2, n3, d3)
		if parallel {
			continue
		}
		r := p0.Minus(c).Magnitude()
		round := r > lengthEps
		for u := 0.0; round && u <= 1; u += 0.25 {
			for v := 0.0; v <= 1; v += 0.25 {
				if math.Abs(s.PointAt(u, v).Minus(c).Magnitude()-r) > lengthEps {
					round = false
					break
				}
			}
		}
		if !round {
			continue
		}
		var smax, smin geom.Vector
		s.AxisAlignedBounding(&smax, &smin)
		ss = append(ss, sphere{c, r, smax.Plus(grow), smin.Minus(grow)})
	}
	return ss
}

// snapToSpheres is snapToCylinders for spheres: a patch of b on a sphere
// about a centre within delta of one of a's, of a radius off a's by more
// than snapPlaneFloor and less than delta, is scaled about a's centre
// onto a's sphere, and with it every point of b within that patch's box
// between a little inside its radius and its control net's reach. A ball
// a hair larger or smaller than a seat of its radius then fills it.
func snapToSpheres(a, b *SShell) *SShell {
	delta := a.Tol.Or(b.Tol).ChordTolMm() / snapPlaneDiv
	if delta <= snapPlaneFloor {
		return b
	}
	grow := geom.Vector{X: 2 * delta, Y: 2 * delta, Z: 2 * delta}
	type match struct {
		a, b sphere
	}
	var ms []match
	bs := spheresOf(b, grow)
	for _, sa := range spheresOf(a, grow) {
		for _, sb := range bs {
			if sa.pmax.BoundingBoxesDisjoint(sa.pmax, sa.pmin, sb.pmax, sb.pmin) {
				continue
			}
			if sb.c.Minus(sa.c).Magnitude() > delta {
				continue
			}
			if d := math.Abs(sa.r - sb.r); d > snapPlaneFloor && d < delta {
				ms = append(ms, match{sa, sb})
			}
		}
	}
	if len(ms) == 0 {
		return b
	}
	changed := false
	snap := func(p geom.Vector) geom.Vector {
		for _, m := range ms {
			if !p.OutsideAndNotOn(m.b.pmax, m.b.pmin) {
				d := p.Minus(m.b.c).Magnitude()
				if d < m.b.r-2*delta || d > 2*m.b.r+2*delta {
					continue
				}
				if q := m.a.c.Plus(p.Minus(m.b.c).ScaledBy(m.a.r / m.b.r)); q != p {
					p, changed = q, true
				}
				break
			}
		}
		return p
	}
	out := b.Clone()
	movePoints(&out, snap)
	if !changed {
		return b
	}
	return &out
}
