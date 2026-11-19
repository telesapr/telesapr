package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// poleSides finds the sides of a patch's uv square that the surface
// folds to one point, as a cone's apex or a sphere's pole: a point of
// the surface there has any parameter along that side.
type poleSides struct {
	s *SSurface
	// folds is, by side (0 to 3: u = 0, u = 1, v = 0, v = 1), 1 where
	// the side folds, 0 where it doesn't, -1 not yet known.
	folds [4]int
}

func newPoleSides(s *SSurface) *poleSides {
	return &poleSides{s: s, folds: [4]int{-1, -1, -1, -1}}
}

// side is the side of the uv square that p lies on and that the
// surface folds to a point, or -1.
func (ps *poleSides) side(p geom.Vector) int {
	const eps = 1e-9
	k := -1
	switch {
	case math.Abs(p.X) < eps:
		k = 0
	case math.Abs(p.X-1) < eps:
		k = 1
	case math.Abs(p.Y) < eps:
		k = 2
	case math.Abs(p.Y-1) < eps:
		k = 3
	}
	if k < 0 {
		return -1
	}
	if ps.folds[k] < 0 {
		ps.folds[k] = 0
		if ps.s.foldsSide(k) {
			ps.folds[k] = 1
		}
	}
	if ps.folds[k] == 1 {
		return k
	}
	// A corner: the other side through it may fold.
	if k < 2 && (math.Abs(p.Y) < eps || math.Abs(p.Y-1) < eps) {
		return ps.side(geom.Vector{X: 0.5, Y: p.Y})
	}
	return -1
}

// alongSide sets p's parameter along side k to q's.
func alongSide(p *geom.Vector, q geom.Vector, k int) {
	if k < 2 {
		p.Y = q.Y
	} else {
		p.X = q.X
	}
}

// uvOffPoles mends a closed outline in s's parameters, pts (its last
// point its first again, or not), where it passes through a pole: the
// closest point there is one end of the folded side whichever way the
// outline comes in, so that a straight line up to a cone's apex and the
// one back down cross the uv square on a diagonal, and the outline
// cuts off a corner of the face. A point at a pole becomes two along
// the folded side, the first at the parameter of the point before it
// and the second at the one after, as the outline reaches the pole
// and leaves it.
func (s *SSurface) uvOffPoles(pts []geom.Vector) []geom.Vector {
	if s.DegM < 1 || s.DegN < 1 || len(pts) < 3 {
		return pts
	}
	closed := pts[0].Equals(pts[len(pts)-1])
	ring := pts
	if closed {
		ring = pts[:len(pts)-1]
	}
	n := len(ring)
	ps := newPoleSides(s)
	sides := make([]int, n)
	atPole := false
	for i, p := range ring {
		sides[i] = ps.side(p)
		atPole = atPole || sides[i] >= 0
	}
	if !atPole {
		return pts
	}
	// off is the nearest point from i on, stepping by d, not at a pole.
	off := func(i, d int) (geom.Vector, bool) {
		for range n {
			i = (i + d + n) % n
			if sides[i] < 0 {
				return ring[i], true
			}
		}
		return geom.Vector{}, false
	}
	var out []geom.Vector
	for i, p := range ring {
		k := sides[i]
		if k < 0 {
			out = append(out, p)
			continue
		}
		prev, okP := off(i, -1)
		next, okN := off(i, 1)
		if !okP || !okN {
			return pts
		}
		in, leave := p, p
		alongSide(&in, prev, k)
		alongSide(&leave, next, k)
		out = append(out, in)
		if !leave.Equals(in) {
			out = append(out, leave)
		}
	}
	if closed {
		out = append(out, out[0])
	}
	return out
}
