package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A loft drawn from a face of a solid can carry on the solid's sides
// smoothly: at each point of its profile, which lies along edges of
// that face, it leaves along the side face that meets the edge there,
// in the side's tangent plane and square to the edge, rather than square
// to the profile's plane. Where the sides are square to the face the two
// are the same; where they lean, as a drafted or turned body's do, or
// curve, as a loft's do, the loft follows them.

// sideTol is how far from a side's surface a profile's point may lie
// and still be on it.
const sideTol = 1e-3

// SideTangents finds, for points of a profile drawn on a face of the
// shell whose plane has normal n, pointing away from the solid, the
// direction across which the loft leaves each point, tangent to the
// side faces meeting the face's edge there: n made square to the side's
// normal, or at a corner between two sides the line they meet in. A point on no edge of
// the shell, or on an edge with no side below the face, is an error.
func SideTangents(sh *srf.SShell, n geom.Vector) (func(p geom.Vector) (geom.Vector, error), error) {
	t, err := Build(sh)
	if err != nil {
		return nil, fmt.Errorf("the solid's topology: %w", err)
	}
	n = n.WithMagnitude(1)
	return func(p geom.Vector) (geom.Vector, error) {
		var ds, ns []geom.Vector
		seen := map[int]bool{}
		for e := range t.Edges {
			ed := &t.Edges[e]
			if distanceToPolyline(p, ed.Points) > edgeTol(ed.Points) {
				continue
			}
			for _, ce := range ed.CoEdges {
				// A face met along two of its edges, at a corner, counts
				// once.
				face := t.Loops[t.CoEdges[ce].Loop].Face
				if seen[face] {
					continue
				}
				seen[face] = true
				d, nrm, ok := t.sideAlong(t.CoEdges[ce].Surface, face, p, n)
				if ok {
					ds, ns = append(ds, d), append(ns, nrm)
				}
			}
		}
		if len(ds) == 0 {
			return geom.Vector{}, fmt.Errorf("the profile at (%g, %g, %g) is on no edge of the solid with a side "+
				"below its face: there is no face to be tangent to", p.X, p.Y, p.Z)
		}
		// At a corner between two sides that meet at an angle the loft
		// leaves along the line they meet in, which lies in both: a
		// profile's straight edge between two corners then leaves in its
		// side's plane all along.
		if len(ds) == 2 {
			if e := ns[0].Cross(ns[1]); e.Magnitude() > 1e-6 {
				if e.Dot(n) < 0 {
					e = e.ScaledBy(-1)
				}
				return e.WithMagnitude(1), nil
			}
		}
		var sum geom.Vector
		for _, d := range ds {
			sum = sum.Plus(d)
		}
		if sum.Magnitude() < 1e-9 {
			return geom.Vector{}, fmt.Errorf("the solid's sides at (%g, %g, %g) lead apart", p.X, p.Y, p.Z)
		}
		return sum.WithMagnitude(1), nil
	}, nil
}

// sideAlong is the direction a face, of surface hs, carries on past p
// away from the solid, n made square to its normal at p, if p lies on
// it, it is not the face the profile is drawn on, and it lies below p.
func (t *Topology) sideAlong(hs core.HSSurface, face int, p, n geom.Vector) (d, normal geom.Vector, ok bool) {
	s := t.shell.FindSurface(hs)
	if s == nil {
		return d, normal, false
	}
	var u, v float64
	s.ClosestPointToUV(p, &u, &v, false)
	if s.PointAt(u, v).Minus(p).Magnitude() > sideTol {
		return d, normal, false
	}
	ns := s.NormalAt(u, v)
	if ns.Magnitude() == 0 {
		return d, normal, false
	}
	ns = ns.WithMagnitude(1)
	if math.Abs(ns.Dot(n)) > 1-1e-6 {
		// The face the profile lies on, or one level with it.
		return d, normal, false
	}
	// The side must lie below the profile's face, on the solid's side of
	// it, to be carried on past the edge.
	for _, l := range t.Faces[face].Loops {
		for _, ce := range t.Loops[l].CoEdges {
			for _, q := range t.Edges[t.CoEdges[ce].Edge].Points {
				if q.Minus(p).Dot(n) > sideTol {
					return d, normal, false
				}
			}
		}
	}
	return n.Minus(ns.ScaledBy(n.Dot(ns))).WithMagnitude(1), ns, true
}

// errSidesAgainst is a loft whose sides would turn back on themselves.
var errSidesAgainst = errors.New("the solid's sides lead back against the loft")

// LoopNormal is the unit normal of a closed loop of curves, by the
// right-hand rule round it.
func LoopNormal(loop []srf.SBezier) geom.Vector { return newell(loop).WithMagnitude(1) }

// LoopCentroid is the middle of a closed loop of curves.
func LoopCentroid(loop []srf.SBezier) geom.Vector { return centroid(loop) }
