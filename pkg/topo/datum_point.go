package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The frames of datums on faces neither flat nor round, and on edges
// neither straight nor round, are taken at a point of the face or edge
// found again the same way at every regeneration: the middle of the
// face in its surface's parameters, and the point half way along the
// edge. An edit that moves or bends the face moves the point with it,
// as it moves the face's parameters.

// datumGrid is how many points a side the search for a point inside a
// face takes, when the middle of its outer loop is not in the face.
const datumGrid = 32

// FacePointFrame is the frame of the face with identity id at a point
// of it: the plane touching it there, the normal out of the solid, the
// first axis along the surface's first parameter. The point is the
// middle of the face's outer loop in its surface's parameters, or where
// that lies outside the face, as in a hole or beyond a notch, the point
// of the face nearest it on a grid over the loop. A face a Boolean cut
// in pieces is taken by its first piece.
func (t *Topology) FacePointFrame(id srf.FaceID) (FaceFrame, error) {
	return t.facePointFrameAt(id, nil)
}

// facePointFrameAt is FacePointFrame at place (FacePlace) rather than at
// the face's middle, where place isn't nil; a place the face no longer
// covers, as after an edit opened a hole there, gives the point of the
// face nearest it.
func (t *Topology) facePointFrameAt(id srf.FaceID, place []float64) (FaceFrame, error) {
	fs := t.FacesWithID(id)
	if len(fs) == 0 {
		return FaceFrame{}, fmt.Errorf("face %x is no longer in the solid", uint32(id))
	}
	f := fs[0]
	s := uncached(t.Surface(f))
	if s == nil {
		return FaceFrame{}, fmt.Errorf("face %x has no surface", uint32(id))
	}
	var outer []geom.Vector
	var holes [][]geom.Vector
	for _, l := range t.Faces[f].Loops {
		uv := t.loopUV(s, t.Loops[l].CoEdges)
		if t.Loops[l].Outer {
			outer = uv
		} else {
			holes = append(holes, uv)
		}
	}
	if len(outer) < 3 {
		return FaceFrame{}, fmt.Errorf("face %x has no outer loop", uint32(id))
	}
	want := polygonMiddle(outer)
	if len(place) == 2 {
		lo, hi := uvBounds(outer)
		want = geom.Vector{X: lo.X + place[0]*(hi.X-lo.X), Y: lo.Y + place[1]*(hi.Y-lo.Y)}
	}
	uv, ok := insideNear(want, outer, holes)
	if !ok {
		return FaceFrame{}, fmt.Errorf("face %x has no point inside its loops", uint32(id))
	}
	n := s.NormalAt(uv.X, uv.Y)
	if n.Magnitude() < geom.LengthEps {
		return FaceFrame{}, fmt.Errorf("face %x has no normal at its middle", uint32(id))
	}
	n = n.WithMagnitude(1)
	var tu, tv geom.Vector
	s.TangentsAt(uv.X, uv.Y, &tu, &tv, false)
	return FaceFrame{Origin: s.PointAt(uv.X, uv.Y), U: frameU(tu, n), N: n}, nil
}

// uncached is a copy of s whose search for the closest point starts
// afresh: the surface's own starts from where its last search ended,
// so a datum found through it would move in its last bits with what
// was looked for before, and the search would move the shell's.
func uncached(s *srf.SSurface) *srf.SSurface {
	if s == nil {
		return nil
	}
	c := *s
	c.Cached = geom.Point2d{}
	return &c
}

// polygonMiddle is the centroid of a polygon's area, or of its corners
// where it has none.
func polygonMiddle(poly []geom.Vector) geom.Vector {
	var a2 float64
	var c geom.Vector
	for i := range poly {
		p, q := poly[i], poly[(i+1)%len(poly)]
		w := p.X*q.Y - q.X*p.Y
		a2 += w
		c.X += (p.X + q.X) * w
		c.Y += (p.Y + q.Y) * w
	}
	if math.Abs(a2) > geom.LengthEps*geom.LengthEps {
		return geom.Vector{X: c.X / (3 * a2), Y: c.Y / (3 * a2)}
	}
	var sum geom.Vector
	for _, p := range poly {
		sum = sum.Plus(p)
	}
	return sum.ScaledBy(1 / float64(len(poly)))
}

// insideNear is uv when it lies inside outer and outside every hole,
// and otherwise the point of a grid over outer's bounds that does and
// lies nearest it; false when no point of the grid does.
func insideNear(uv geom.Vector, outer []geom.Vector, holes [][]geom.Vector) (geom.Vector, bool) {
	inside := func(p geom.Vector) bool {
		if !pointInPolygon(p, outer) {
			return false
		}
		for _, h := range holes {
			if pointInPolygon(p, h) {
				return false
			}
		}
		return true
	}
	if inside(uv) {
		return uv, true
	}
	lo, hi := uvBounds(outer)
	best, found := geom.Vector{}, false
	for i := range datumGrid {
		for j := range datumGrid {
			p := geom.Vector{
				X: lo.X + (hi.X-lo.X)*(float64(i)+0.5)/datumGrid,
				Y: lo.Y + (hi.Y-lo.Y)*(float64(j)+0.5)/datumGrid,
			}
			if inside(p) && (!found || p.Minus(uv).Magnitude() < best.Minus(uv).Magnitude()) {
				best, found = p, true
			}
		}
	}
	return best, found
}

// errNoLength is edgePointFrame's error for an edge of no length.
var errNoLength = errors.New("the edge has no length")

// edgePointFrame is the frame of edge e at the point half way along it,
// or with place the fraction of its length from its start (EdgePlace):
// the normal along the edge from its start, the first axis out of the
// first face beside it, square to the edge.
func (t *Topology) edgePointFrame(e int, place []float64) (FaceFrame, error) {
	pts := t.Edges[e].Points
	var total float64
	for i := 1; i < len(pts); i++ {
		total += pts[i].Minus(pts[i-1]).Magnitude()
	}
	if len(pts) < 2 || total < geom.LengthEps {
		return FaceFrame{}, errNoLength
	}
	reach := total / 2
	if len(place) == 1 {
		reach = total * min(max(place[0], 0), 1)
	}
	var at, dir geom.Vector
	walked := 0.0
	for i := 1; i < len(pts); i++ {
		d := pts[i].Minus(pts[i-1])
		l := d.Magnitude()
		if l < geom.LengthEps*geom.LengthEps {
			continue
		}
		if walked+l >= reach || i == len(pts)-1 {
			at, dir = pts[i-1].Plus(d.ScaledBy(min((reach-walked)/l, 1))), d.WithMagnitude(1)
			break
		}
		walked += l
	}
	// The polyline's point and direction are a chord's; the faces beside
	// the edge put the point on the curve and, where they meet at an
	// angle, the direction along it, square to both normals.
	var normals []geom.Vector
	for _, f := range t.EdgeFaces(e) {
		s := uncached(t.Surface(max(f, 0)))
		if f < 0 || s == nil {
			continue
		}
		var su, sv float64
		s.ClosestPointToUV(at, &su, &sv, false)
		if len(normals) == 0 {
			at = s.PointAt(su, sv)
		}
		if m := s.NormalAt(su, sv); m.Magnitude() > geom.LengthEps {
			normals = append(normals, m.WithMagnitude(1))
		}
	}
	u := dir.NormalDir(0)
	if len(normals) > 0 {
		u = normals[0]
	}
	if len(normals) == 2 {
		if along := normals[0].Cross(normals[1]); along.Magnitude() > 1e-3 {
			along = along.WithMagnitude(1)
			if along.Dot(dir) < 0 {
				along = along.ScaledBy(-1)
			}
			dir = along
		}
	}
	return FaceFrame{Origin: at, U: frameU(u, dir), N: dir}, nil
}
