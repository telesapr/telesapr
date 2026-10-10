package topo

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A sketch drawn on a flat face of the solid (F1) lies in a workplane
// taken from the face at every regeneration. Its origin and axes come
// from an edge of the face chosen by name, so that they stay on the same
// corner and edge when an edit upstream moves or resizes the face: the
// straight edge of the face's outer boundary with the least name, the
// origin at the end of it from which the face lies to the left, the
// normal out of the solid. A face with no straight edge, as a cylinder's
// cap, takes the centre of its least-named circular edge as the origin
// and the direction to that edge's start as u.

// FaceFrame is a workplane in a flat face: an origin, a unit u along the
// face, and the unit normal out of the solid. v is N × U.
type FaceFrame struct {
	Origin geom.Vector
	U, N   geom.Vector
}

// V is the frame's second axis, square to U in the face.
func (f FaceFrame) V() geom.Vector { return f.N.Cross(f.U) }

// Quaternion is the frame's orientation as a workplane's normal holds it.
func (f FaceFrame) Quaternion() geom.Quaternion { return geom.QuaternionFromBasis(f.U, f.V()) }

// frameEdge is an edge of a face's outer boundary as a candidate for its
// frame, and the order candidates are taken in.
type frameEdge struct {
	face, coEdge int
	straight     bool
	named        bool
	name         Name
	start        geom.Vector
}

// before reports whether a is a better edge to take a frame from than b:
// straight first, then named, by name, then by where it starts.
func (a frameEdge) before(b frameEdge) bool {
	if a.straight != b.straight {
		return a.straight
	}
	if a.named != b.named {
		return a.named
	}
	if a.named && a.name != b.name {
		return a.name.Less(b.name)
	}
	for ax := range 3 {
		if d := a.start.Element(ax) - b.start.Element(ax); math.Abs(d) > geom.LengthEps {
			return d < 0
		}
	}
	return false
}

// FaceFrame is the frame of the flat face with identity id: an error
// when the solid has no face of that identity, or when it is not flat.
// A face a Boolean cut in pieces has its pieces' edges to choose from.
func (t *Topology) FaceFrame(id srf.FaceID) (FaceFrame, error) {
	fs := t.FacesWithID(id)
	if len(fs) == 0 {
		return FaceFrame{}, fmt.Errorf("face %x is no longer in the solid", uint32(id))
	}
	n, ok := t.planarNormal(fs[0])
	if !ok {
		return FaceFrame{}, fmt.Errorf("face %x is not flat", uint32(id))
	}

	best, found := frameEdge{}, false
	for _, f := range fs {
		if m, ok := t.planarNormal(f); !ok || m.Dot(n) < 1-geom.LengthEps {
			continue
		}
		for _, l := range t.Faces[f].Loops {
			if !t.Loops[l].Outer {
				continue
			}
			for _, ce := range t.Loops[l].CoEdges {
				c := t.candidate(f, ce)
				if !found || c.before(best) {
					best, found = c, true
				}
			}
		}
	}
	if !found {
		return FaceFrame{}, fmt.Errorf("face %x has no edges", uint32(id))
	}
	return t.frameFrom(best, n), nil
}

// candidate describes co-edge ce of face f's outer loop.
func (t *Topology) candidate(f, ce int) frameEdge {
	e := t.CoEdges[ce].Edge
	c := frameEdge{face: f, coEdge: ce, start: t.Vertices[t.Start(ce)].P}
	c.straight = straight(t.Edges[e].Points)
	if name, err := t.NameOf(e); err == nil {
		c.named, c.name = true, name
	}
	return c
}

// straight reports whether a polyline is a segment of a line, of some
// length.
func straight(pts []geom.Vector) bool {
	if len(pts) < 2 {
		return false
	}
	a, b := pts[0], pts[len(pts)-1]
	d := b.Minus(a)
	if d.Magnitude() < geom.LengthEps {
		return false
	}
	for _, p := range pts[1 : len(pts)-1] {
		if p.DistanceToLine(a, d) > geom.LengthEps {
			return false
		}
	}
	return true
}

// frameFrom is the frame that edge c gives a face of outward normal n.
func (t *Topology) frameFrom(c frameEdge, n geom.Vector) FaceFrame {
	a, b := t.Vertices[t.Start(c.coEdge)].P, t.Vertices[t.End(c.coEdge)].P
	if c.straight {
		u := b.Minus(a).WithMagnitude(1)
		// The origin is the end from which the face lies on the side v
		// points to, so that the face is drawn in the first quadrant
		// near the origin.
		if n.Cross(u).Dot(t.intoFace(c.face, a, b, n)) < 0 {
			a, u = b, u.ScaledBy(-1)
		}
		return FaceFrame{Origin: a, U: inPlane(u, n), N: n}
	}
	e := t.CoEdges[c.coEdge].Edge
	start := t.Vertices[t.Edges[e].V[0]].P
	if sb, ok := t.EdgeCurve(e); ok {
		var centre geom.Vector
		var r float64
		if sb.IsCircle(n, &centre, &r) {
			return FaceFrame{Origin: centre, U: inPlane(start.Minus(centre), n), N: n}
		}
	}
	if b.Minus(a).Magnitude() > geom.LengthEps {
		return FaceFrame{Origin: a, U: inPlane(b.Minus(a), n), N: n}
	}
	// A closed curve that is no circle: the world axis nearest the face.
	axes := []geom.Vector{{X: 1}, {Y: 1}, {Z: 1}}
	u := axes[0]
	for _, ax := range axes[1:] {
		if math.Abs(ax.Dot(n)) < math.Abs(u.Dot(n)) {
			u = ax
		}
	}
	return FaceFrame{Origin: a, U: inPlane(u, n), N: n}
}

// inPlane is d made square to n, of unit length.
func inPlane(d, n geom.Vector) geom.Vector {
	return d.Minus(n.ScaledBy(d.Dot(n))).WithMagnitude(1)
}
