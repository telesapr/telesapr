package topo

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The ends of a cylindrical face, where a thread on it can start: a
// thread made by pointing at a shaft or a hole starts at one end of the
// face and runs along it into the part. An end is named by the face
// across it, so that the place follows the part when it changes
// upstream, as a datum does (AxisEndFrame).

// AxisEnd is one end of a cylindrical face.
type AxisEnd struct {
	// End is the identity of the face across the end, as a shaft's cap
	// or the face a hole is drilled from.
	End srf.FaceID
	// Frame has its origin on the axis in the plane of the end and its
	// normal along the axis into the face.
	Frame FaceFrame
	// Radius is the face's, and Length how far it reaches along the
	// axis.
	Radius, Length float64
	// Free is set where the face across the end faces away from the
	// cylinder, as a shaft's cap or the top of a block about a hole
	// does: the end of the part, where a thread starts; clear at a
	// shoulder or a blind hole's bottom.
	Free bool
	// Hole is set when the material lies outside the cylinder, as about
	// a hole; clear for a shaft.
	Hole bool
}

// axisEndTol is how far apart, along the axis, two circles are taken as
// one end.
const axisEndTol = 1e-6

// AxisEnds are the two ends of the cylindrical face with identity id,
// the one nearer the start of its axis (AxisFrame) first. It is an
// error when the face is not a cylinder, as a plane or a cone, or has
// no edges round its axis at either end.
func (t *Topology) AxisEnds(id srf.FaceID) ([2]AxisEnd, error) {
	ax, err := t.AxisFrame(id)
	if err != nil {
		return [2]AxisEnd{}, err
	}
	fs := t.FacesWithID(id)
	r, hole, err := t.cylinderRadius(fs, ax)
	if err != nil {
		return [2]AxisEnd{}, fmt.Errorf("face %x %w", uint32(id), err)
	}

	// Each end is where circles of the axis bound the face: those at
	// the least and the most distance along it.
	type rim struct {
		at    float64
		other srf.FaceID
		face  int
		edge  int
		start geom.Vector
	}
	var rims []rim
	for _, f := range fs {
		for _, e := range t.FaceEdges(f) {
			ed := &t.Edges[e]
			if ed.Seam {
				continue
			}
			c, ok := polylineCircle(ed.Points)
			if !ok || c.n.Cross(ax.N).Magnitude() > 1e-6 || c.centre.Minus(ax.Origin).Cross(ax.N).Magnitude() > geom.LengthEps*max(1, r) {
				continue
			}
			for _, g := range t.EdgeFaces(e) {
				if g >= 0 && t.Faces[g].ID != id {
					rims = append(rims, rim{at: c.centre.Minus(ax.Origin).Dot(ax.N), other: t.Faces[g].ID,
						face: g, edge: e, start: c.start})
				}
			}
		}
	}
	if len(rims) == 0 {
		return [2]AxisEnd{}, fmt.Errorf("face %x has no circular edges at its ends", uint32(id))
	}
	lo, hi := rims[0].at, rims[0].at
	for _, rm := range rims {
		lo, hi = min(lo, rm.at), max(hi, rm.at)
	}
	if hi-lo < geom.LengthEps {
		return [2]AxisEnd{}, fmt.Errorf("face %x has circular edges at one end only", uint32(id))
	}

	var out [2]AxisEnd
	for i, at := range []float64{lo, hi} {
		n := ax.N
		if i == 1 {
			n = n.ScaledBy(-1)
		}
		origin := ax.Origin.Plus(ax.N.ScaledBy(at))
		// The face across is the one that most of the end's circles meet,
		// the lowest identity among as many.
		count := map[srf.FaceID]int{}
		var first *rim
		for k := range rims {
			if math.Abs(rims[k].at-at) <= axisEndTol*max(1, hi-lo) {
				count[rims[k].other]++
				if first == nil {
					first = &rims[k]
				}
			}
		}
		ids := slices.Sorted(maps.Keys(count))
		end := ids[0]
		for _, k := range ids {
			if count[k] > count[end] {
				end = k
			}
		}
		var across *rim
		for k := range rims {
			if rims[k].other == end && math.Abs(rims[k].at-at) <= axisEndTol*max(1, hi-lo) {
				across = &rims[k]
				break
			}
		}
		out[i] = AxisEnd{
			End:    end,
			Frame:  FaceFrame{Origin: origin, U: frameU(first.start.Minus(origin), n), N: n},
			Radius: r, Length: hi - lo, Hole: hole,
			Free: t.facesAway(across.face, across.edge, n),
		}
	}
	return out, nil
}

// AxisEndFrame is the frame of the end of the cylindrical face face
// that the face end lies across (AxisEnds): on the axis in the end's
// plane, its normal along the axis into the face.
func (t *Topology) AxisEndFrame(face, end srf.FaceID) (FaceFrame, error) {
	ends, err := t.AxisEnds(face)
	if err != nil {
		return FaceFrame{}, err
	}
	for _, e := range ends {
		if e.End == end {
			return e.Frame, nil
		}
	}
	return FaceFrame{}, fmt.Errorf("face %x no longer ends at face %x", uint32(face), uint32(end))
}

// AxisEndAt is the end of a cylindrical face at the circular edge e,
// and that face's identity: of the edge's two faces, the cylinder, the
// other being the face across the end.
func (t *Topology) AxisEndAt(e int) (srf.FaceID, AxisEnd, error) {
	for _, f := range t.EdgeFaces(e) {
		if f < 0 {
			continue
		}
		id := t.Faces[f].ID
		ends, err := t.AxisEnds(id)
		if err != nil {
			continue
		}
		for _, g := range t.EdgeFaces(e) {
			if g < 0 || g == f {
				continue
			}
			for _, end := range ends {
				if end.End == t.Faces[g].ID {
					return id, end, nil
				}
			}
		}
	}
	return 0, AxisEnd{}, fmt.Errorf("edge %d is not the end of a cylindrical face", e)
}

// cylinderRadius is the radius of the cylinder the faces fs lie on
// about the axis of ax, and whether its material is outside it, as
// about a hole; an error when they are not on one cylinder.
func (t *Topology) cylinderRadius(fs []int, ax FaceFrame) (float64, bool, error) {
	const steps = 4
	r := -1.0
	var out, in int
	for _, f := range fs {
		s := t.Surface(f)
		if s == nil {
			return 0, false, fmt.Errorf("has no surface")
		}
		for i := range steps + 1 {
			for j := range steps + 1 {
				u, v := float64(i)/steps, float64(j)/steps
				p := s.PointAt(u, v)
				d := p.Minus(ax.Origin)
				radial := d.Minus(ax.N.ScaledBy(d.Dot(ax.N)))
				dist := radial.Magnitude()
				if r < 0 {
					r = dist
				}
				if math.Abs(dist-r) > geom.LengthEps*max(1, r) {
					return 0, false, fmt.Errorf("is not a cylinder")
				}
				switch m := s.NormalAt(u, v).Dot(radial); {
				case m > 0:
					out++
				case m < 0:
					in++
				}
			}
		}
	}
	if !(r > geom.LengthEps) {
		return 0, false, fmt.Errorf("is not a cylinder")
	}
	return r, in > out, nil
}

// facesAway reports whether face f, across edge e from a cylinder,
// faces away from it: its outward normal near the edge points against
// n, the direction along the axis into the cylinder.
func (t *Topology) facesAway(f, e int, n geom.Vector) bool {
	if m, ok := t.planarNormal(f); ok {
		return m.Dot(n) < -1e-6
	}
	s := t.Surface(f)
	pts := t.Edges[e].Points
	if s == nil || len(pts) == 0 {
		return false
	}
	var u, v float64
	s.ClosestPointToUV(pts[len(pts)/2], &u, &v, false)
	m := s.NormalAt(u, v)
	if m.Magnitude() < geom.LengthEps {
		return false
	}
	return m.WithMagnitude(1).Dot(n) < -1e-6
}
