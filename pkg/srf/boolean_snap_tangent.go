package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// flatPlane is a planar patch: points x with n·x = d, n of unit length,
// and the patch's box grown by the snap's reach.
type flatPlane struct {
	n          geom.Vector
	d          float64
	pmax, pmin geom.Vector
}

// flatPlanesOf are sh's planar patches, their boxes grown by grow.
func flatPlanesOf(sh *SShell, grow geom.Vector) []flatPlane {
	var ps []flatPlane
	for i := range sh.Surface {
		s := &sh.Surface[i]
		if s.DegM != 1 || s.DegN != 1 {
			continue
		}
		var smax, smin geom.Vector
		s.AxisAlignedBounding(&smax, &smin)
		n := s.NormalAt(0, 0)
		if n.Magnitude() < lengthEps {
			continue
		}
		n = n.WithMagnitude(1)
		ps = append(ps, flatPlane{n, n.Dot(s.Ctrl[0][0]), smax.Plus(grow), smin.Minus(grow)})
	}
	return ps
}

// snapTangent is the fuse of roadmap K3 for a round side resting on a
// plane: a cylinder's side parallel to a plane of the other operand, or a
// sphere, whose distance from the plane differs from its radius by more
// than snapPlaneFloor and less than a tenth of the chord tolerance. Sunk
// a hair, the two cut each other in two lines (or a circle) closer than
// the mesh can tell apart, the trims between them close on nothing, and
// the Boolean failed; standing a hair off, nothing met where the mesh
// shows them touching. b is moved, whole and rigidly, along the plane's
// normal until the two touch: the Boolean then meets a round face
// tangent to a plane, which it takes as touching (the union is then no
// manifold along the line, as the true union of solids that touch, and
// Check Solid says so; the difference and the intersection are sound).
// Every match must ask the same move, or nothing moves; a never moves.
// It returns b if nothing moved, or a moved copy.
func snapTangent(a, b *SShell) *SShell {
	delta := a.Tol.Or(b.Tol).ChordTolMm() / snapPlaneDiv
	if delta <= snapPlaneFloor {
		return b
	}
	grow := geom.Vector{X: 2 * delta, Y: 2 * delta, Z: 2 * delta}
	var move geom.Vector
	found, conflict := false, false
	// ask takes the move of b that would bring a round of radius r,
	// whose centre or axis lies s from a plane of normal n, to touch it;
	// bMoves is -1 when the plane is b's, as b's plane moves the other
	// way to touch a's round.
	ask := func(n geom.Vector, s, r, bMoves float64) {
		gap := math.Abs(s) - r
		if math.Abs(gap) <= snapPlaneFloor || math.Abs(gap) >= delta {
			return
		}
		sign := 1.0
		if s < 0 {
			sign = -1
		}
		v := n.ScaledBy(-sign * gap * bMoves)
		if !found {
			move, found = v, true
			return
		}
		if v.Minus(move).Magnitude() > snapPlaneFloor {
			conflict = true
		}
	}
	pair := func(planes []flatPlane, cyls []cylinder, balls []sphere, bMoves float64) {
		for _, p := range planes {
			for _, c := range cyls {
				if p.pmax.BoundingBoxesDisjoint(p.pmax, p.pmin, c.pmax, c.pmin) {
					continue
				}
				if math.Abs(c.dir.Dot(p.n)) > snapPlaneFloor {
					continue
				}
				ask(p.n, p.n.Dot(c.c)-p.d, c.r, bMoves)
			}
			for _, sp := range balls {
				if p.pmax.BoundingBoxesDisjoint(p.pmax, p.pmin, sp.pmax, sp.pmin) {
					continue
				}
				ask(p.n, p.n.Dot(sp.c)-p.d, sp.r, bMoves)
			}
		}
	}
	pair(flatPlanesOf(a, grow), cylindersOf(b, grow), spheresOf(b, grow), 1)
	pair(flatPlanesOf(b, grow), cylindersOf(a, grow), spheresOf(a, grow), -1)
	if !found || conflict {
		return b
	}
	out := b.Clone()
	movePoints(&out, func(p geom.Vector) geom.Vector { return p.Plus(move) })
	return &out
}
