package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// snapPlaneDiv is the chord tolerance over the distance within which a
// point of a Boolean's second operand is moved onto a plane of the first
// (snapToPlanes): a tenth of it, so that a gap or step a user drew wider
// than that stays, and one the mesh could not show is closed.
const snapPlaneDiv = 10

// snapPlaneFloor is how far off a plane a point must be to be moved:
// nearer, it is on the plane but for rounding, which the Boolean takes
// as coincident already, and moving it would change the last bits of
// every result.
const snapPlaneFloor = 1e-9

// snapToPlanes is the first step of roadmap K3's "fuse first" (the spike
// of 2026-11-04, doc/dev-notes/k3-fuse-spike.md), for planes. A planar
// patch of b parallel to one of a's, their boxes overlapping, and off it
// by more than snapPlaneFloor and less than a tenth of the chord
// tolerance, is moved onto a's plane, and with it every point of b
// (patch control points, curves' control and pwl points, trims' ends)
// within twice that of the plane, up to three planes in turn (a corner
// near a top and a side). It returns b if nothing moved, or a moved
// copy; a never moves, so the model before a feature keeps its
// geometry. A block standing a hair off another's top, or sunk a hair
// into it, then stands on it: the Boolean meets coincident faces, which
// it classifies as such, where it met two planes a hair apart, cut
// nothing between them it could mesh, and failed, left the mesh open, or
// made a sliver.
func snapToPlanes(a, b *SShell) *SShell {
	delta := a.Tol.Or(b.Tol).ChordTolMm() / snapPlaneDiv
	if delta <= snapPlaneFloor {
		return b
	}
	grow := geom.Vector{X: delta, Y: delta, Z: delta}
	type plane struct {
		n          geom.Vector
		d          float64
		pmax, pmin geom.Vector
	}
	planesOf := func(sh *SShell) []plane {
		var ps []plane
		for i := range sh.Surface {
			s := &sh.Surface[i]
			if s.DegM != 1 || s.DegN != 1 {
				continue
			}
			var smax, smin geom.Vector
			s.AxisAlignedBounding(&smax, &smin)
			n := s.NormalAt(0, 0).WithMagnitude(1)
			ps = append(ps, plane{n, n.Dot(s.Ctrl[0][0]), smax.Plus(grow), smin.Minus(grow)})
		}
		return ps
	}
	// The planes of a that a plane of b lies along, a hair off: parallel,
	// within delta, their patches' boxes overlapping. Points of b are
	// moved onto these alone, and all of a plane of b that lies along one
	// moves with it, so that it stays a plane.
	bPlanes := planesOf(b)
	var planes []plane
	for _, pa := range planesOf(a) {
		for _, pb := range bPlanes {
			if pa.pmax.BoundingBoxesDisjoint(pa.pmax, pa.pmin, pb.pmax, pb.pmin) {
				continue
			}
			if pa.n.Cross(pb.n).Magnitude() > snapPlaneFloor {
				continue
			}
			off := pb.d - pa.d
			if pa.n.Dot(pb.n) < 0 {
				off = -pb.d - pa.d
			}
			if d := math.Abs(off); d > snapPlaneFloor && d < delta {
				planes = append(planes, pa)
				break
			}
		}
	}
	if len(planes) == 0 {
		return b
	}
	changed := false
	snap := func(p geom.Vector) geom.Vector {
		for range 3 {
			moved := false
			for _, pl := range planes {
				// Twice delta: the plane of b was matched within delta, and
				// all of its points must follow it, each a little off by
				// rounding, the turn of a turned solid among it.
				off := pl.n.Dot(p) - pl.d
				if d := math.Abs(off); d > snapPlaneFloor && d < 2*delta {
					p = p.Minus(pl.n.ScaledBy(off))
					moved, changed = true, true
				}
			}
			if !moved {
				break
			}
		}
		return p
	}
	out := b.Clone()
	for i := range out.Surface {
		s := &out.Surface[i]
		for u := 0; u <= s.DegM; u++ {
			for v := 0; v <= s.DegN; v++ {
				s.Ctrl[u][v] = snap(s.Ctrl[u][v])
			}
		}
		for k := range s.Trim {
			s.Trim[k].StartP = snap(s.Trim[k].StartP)
			s.Trim[k].FinishP = snap(s.Trim[k].FinishP)
		}
	}
	for i := range out.Curve {
		c := &out.Curve[i]
		for k := 0; k <= c.Exact.Degree; k++ {
			c.Exact.Ctrl[k] = snap(c.Exact.Ctrl[k])
		}
		for k := range c.Points {
			c.Points[k].P = snap(c.Points[k].P)
		}
	}
	if !changed {
		return b
	}
	return &out
}

// cylinder is a patch's circular side: an extrusion of an arc, its axis
// through c along unit dir, of radius r.
type cylinder struct {
	c, dir     geom.Vector
	r          float64
	pmax, pmin geom.Vector
}

// cylindersOf are sh's patches that are extrusions of circular arcs, their
// boxes grown by grow.
func cylindersOf(sh *SShell, grow geom.Vector) []cylinder {
	var cs []cylinder
	for i := range sh.Surface {
		s := &sh.Surface[i]
		var of SBezier
		var along geom.Vector
		if !s.IsExtrusion(&of, &along) || along.Magnitude() < lengthEps {
			continue
		}
		dir := along.WithMagnitude(1)
		var c geom.Vector
		var r float64
		if !of.IsCircle(dir, &c, &r) {
			continue
		}
		var smax, smin geom.Vector
		s.AxisAlignedBounding(&smax, &smin)
		cs = append(cs, cylinder{c, dir, r, smax.Plus(grow), smin.Minus(grow)})
	}
	return cs
}

// snapToCylinders is snapToPlanes for the sides of cylinders: a circular
// side of b coaxial with one of a's (its axis on a's within delta) whose
// radius is off a's by more than snapPlaneFloor and less than delta is
// scaled about the axis onto a's radius, and with it every point of b
// about that axis between a little inside its radius and a little past
// its arcs' middle control points, so that b's caps and arcs stay what
// they were, a hair larger or smaller. A pin a hair under or over a
// bore's size then fills it; the Boolean meets coincident sides, which
// classifyEdgeOn takes as such.
func snapToCylinders(a, b *SShell) *SShell {
	delta := a.Tol.Or(b.Tol).ChordTolMm() / snapPlaneDiv
	if delta <= snapPlaneFloor {
		return b
	}
	grow := geom.Vector{X: delta, Y: delta, Z: delta}
	type match struct {
		c, dir geom.Vector
		rb, ra float64
	}
	var ms []match
	bs := cylindersOf(b, grow)
	for _, ca := range cylindersOf(a, grow) {
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
			if d := math.Abs(ca.r - cb.r); d > snapPlaneFloor && d < delta {
				ms = append(ms, match{ca.c, ca.dir, cb.r, ca.r})
			}
		}
	}
	if len(ms) == 0 {
		return b
	}
	changed := false
	snap := func(p geom.Vector) geom.Vector {
		for _, m := range ms {
			along := p.Minus(m.c).Dot(m.dir)
			foot := m.c.Plus(m.dir.ScaledBy(along))
			rad := p.Minus(foot)
			d := rad.Magnitude()
			if d < m.rb-2*delta || d > m.rb*math.Sqrt2+2*delta {
				continue
			}
			// Moved by as little as the radii differ, past what Equals
			// tells from no move.
			q := foot.Plus(rad.ScaledBy(m.ra / m.rb))
			if q != p {
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

// movePoints moves every point of sh by f: its patches' control points,
// its curves' control and pwl points, and its trims' ends.
func movePoints(sh *SShell, f func(geom.Vector) geom.Vector) {
	for i := range sh.Surface {
		s := &sh.Surface[i]
		for u := 0; u <= s.DegM; u++ {
			for v := 0; v <= s.DegN; v++ {
				s.Ctrl[u][v] = f(s.Ctrl[u][v])
			}
		}
		for k := range s.Trim {
			s.Trim[k].StartP = f(s.Trim[k].StartP)
			s.Trim[k].FinishP = f(s.Trim[k].FinishP)
		}
	}
	for i := range sh.Curve {
		c := &sh.Curve[i]
		for k := 0; k <= c.Exact.Degree; k++ {
			c.Exact.Ctrl[k] = f(c.Exact.Ctrl[k])
		}
		for k := range c.Points {
			c.Points[k].P = f(c.Points[k].P)
		}
	}
}
