package topo

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The frame of a datum on a solid: a workplane a part offers by a face
// or an edge of its solid rather than by an entity, so that a face no
// entity makes, as a fillet's, or an edge of the solid, can be named
// and mated (sketch.NewDatumWorkplane). A flat face gives the frame a
// sketch on it has (FaceFrame). A face round about an axis, as a
// cylinder's, a cone's or a fillet's, and a circular edge give a frame
// whose normal is the axis and whose origin is on it; a straight edge
// one whose normal runs along it from its start. Any other face, as a
// loft's, a blend's or a sphere's, gives the plane touching it at its
// middle (FacePointFrame), and any other edge, as a cubic's or a
// helix's, the plane square to it at its middle (EdgePointFrame). Each
// is found again from the solid at every regeneration, so it follows
// the face or edge.

// DatumKind is what the frame of a datum on a solid stands for, and so
// which mates take it: a plane, an axis, or both.
type DatumKind int

const (
	// DatumUnknown is a datum whose kind isn't known, as one named
	// before kinds were kept: every mate takes it.
	DatumUnknown DatumKind = iota
	// DatumPlane is a flat face's: the plane of the face.
	DatumPlane
	// DatumAxis is a round face's or a circular edge's: the axis, and
	// the plane square to it at its origin, as a circle's.
	DatumAxis
	// DatumLine is a straight edge's: the line of the edge.
	DatumLine
	// DatumFacePoint is any other face's: the plane touching it at a
	// point, and the line square to the face there.
	DatumFacePoint
	// DatumEdgePoint is any other edge's: the line touching it at a
	// point.
	DatumEdgePoint
)

// datumKindWords are the kinds' names in a file and on screen.
var datumKindWords = map[DatumKind]string{
	DatumPlane:     "plane",
	DatumAxis:      "axis",
	DatumLine:      "line",
	DatumFacePoint: "face-point",
	DatumEdgePoint: "edge-point",
}

// String is the kind's word, as a file holds it; "" for DatumUnknown.
func (k DatumKind) String() string { return datumKindWords[k] }

// ParseDatumKind is the kind of word w, as String gives it.
func ParseDatumKind(w string) (DatumKind, bool) {
	for k, kw := range datumKindWords {
		if kw == w {
			return k, true
		}
	}
	return DatumUnknown, false
}

// IsAxis reports whether a concentric mate takes the datum as an axis:
// a round face's or a circle's axis, an edge's line, or the line square
// to a face at its point. A flat face's normal runs through a corner of
// the face, where no axis is.
func (k DatumKind) IsAxis() bool { return k != DatumPlane }

// IsPlane reports whether a flush mate takes the datum as a plane: a
// flat face's, the plane square to a round face's axis, or the plane
// touching a face. An edge's line has no one plane.
func (k DatumKind) IsPlane() bool { return k != DatumLine && k != DatumEdgePoint }

// DatumFrame is the frame of the face with identity id as a datum, and
// its kind: its FaceFrame where it is flat, its axis where it is round
// (AxisFrame), and otherwise the plane touching it (FacePointFrame).
func (t *Topology) DatumFrame(id srf.FaceID) (FaceFrame, DatumKind, error) {
	fs := t.FacesWithID(id)
	if len(fs) == 0 {
		return FaceFrame{}, DatumUnknown, fmt.Errorf("face %x is no longer in the solid", uint32(id))
	}
	if _, flat := t.planarNormal(fs[0]); flat {
		fr, err := t.FaceFrame(id)
		return fr, DatumPlane, err
	}
	if fr, err := t.AxisFrame(id); err == nil {
		return fr, DatumAxis, nil
	}
	fr, err := t.FacePointFrame(id)
	return fr, DatumFacePoint, err
}

// datumCircle is a circle of the solid: its centre, its unit normal and its
// radius.
type datumCircle struct {
	centre, n geom.Vector
	r         float64
	// start is a point on it.
	start geom.Vector
}

// polylineCircle is the circle a polyline lies on, if it lies on one.
func polylineCircle(pts []geom.Vector) (datumCircle, bool) {
	if len(pts) < 3 {
		return datumCircle{}, false
	}
	a, m, b := pts[0], pts[len(pts)/3], pts[2*len(pts)/3]
	u, v := m.Minus(a), b.Minus(a)
	w := u.Cross(v)
	if w.Magnitude() < geom.LengthEps*geom.LengthEps {
		return datumCircle{}, false
	}
	// The centre of the circle through a, m and b.
	c := a.Plus(v.ScaledBy(u.Dot(u)).Minus(u.ScaledBy(v.Dot(v))).Cross(w).ScaledBy(1 / (2 * w.Dot(w))))
	r := a.Minus(c).Magnitude()
	n := w.WithMagnitude(1)
	tol := geom.LengthEps * max(1, r)
	for _, p := range pts {
		if math.Abs(p.Minus(c).Magnitude()-r) > tol || math.Abs(p.Minus(c).Dot(n)) > tol {
			return datumCircle{}, false
		}
	}
	return datumCircle{centre: c, n: n, r: r, start: a}, true
}

// AxisFrame is the frame of the face with identity id about its axis:
// the axis of the circles its edges run along, all of which must share
// it, or of the cylinder it lies on. The normal runs along the axis from
// the centre of the first of those circles to the last, or with one
// circle towards the face; the origin is that first centre, and the
// first axis points to where the first circle's edge starts.
func (t *Topology) AxisFrame(id srf.FaceID) (FaceFrame, error) {
	fs := t.FacesWithID(id)
	if len(fs) == 0 {
		return FaceFrame{}, fmt.Errorf("face %x is no longer in the solid", uint32(id))
	}
	var cs []datumCircle
	var sum geom.Vector
	count := 0
	seen := map[int]bool{}
	for _, f := range fs {
		for _, l := range t.Faces[f].Loops {
			for _, ce := range t.Loops[l].CoEdges {
				e := t.CoEdges[ce].Edge
				if seen[e] {
					continue
				}
				seen[e] = true
				for _, p := range t.Edges[e].Points {
					sum = sum.Plus(p)
					count++
				}
				if c, ok := polylineCircle(t.Edges[e].Points); ok {
					cs = append(cs, c)
				}
			}
		}
	}
	if len(cs) == 0 {
		for _, f := range fs {
			var axis, centre, start, finish geom.Vector
			var r float64
			if s := t.Surface(f); s != nil && s.IsCylinder(&axis, &centre, &r, &start, &finish) {
				cs = append(cs, datumCircle{centre: centre, n: axis.WithMagnitude(1), r: r, start: start})
				break
			}
		}
	}
	if len(cs) == 0 {
		return FaceFrame{}, fmt.Errorf("face %x is neither flat nor round about an axis", uint32(id))
	}
	n := cs[0].n
	first, last := cs[0], cs[0]
	for _, c := range cs[1:] {
		if c.n.Cross(n).Magnitude() > 1e-6 || c.centre.Minus(cs[0].centre).Cross(n).Magnitude() > geom.LengthEps*max(1, c.r) {
			return FaceFrame{}, fmt.Errorf("face %x is not round about one axis", uint32(id))
		}
		if c.centre.Dot(n) < first.centre.Dot(n) {
			first = c
		}
		if c.centre.Dot(n) > last.centre.Dot(n) {
			last = c
		}
	}
	// Edges along circles of one axis don't make a face round about it,
	// as a loft's side from a circle to a square has one: the surface
	// must turn about the axis, its normals in planes through the axis.
	for _, f := range fs {
		if !t.turnsAbout(f, first.centre, n) {
			return FaceFrame{}, fmt.Errorf("face %x is not round about one axis", uint32(id))
		}
	}
	switch along := last.centre.Minus(first.centre).Dot(n); {
	case along > geom.LengthEps:
	case count > 0 && sum.ScaledBy(1/float64(count)).Minus(first.centre).Dot(n) < -geom.LengthEps:
		n = n.ScaledBy(-1)
	}
	return FaceFrame{Origin: first.centre, U: frameU(first.start.Minus(first.centre), n), N: n}, nil
}

// turnsAbout reports whether face f's surface is one turned about the
// axis through c along unit n: at each point of a grid over its
// parameters, the normal lies in the plane through the axis and the
// point.
func (t *Topology) turnsAbout(f int, c, n geom.Vector) bool {
	s := t.Surface(f)
	if s == nil {
		return false
	}
	const steps = 4
	for i := range steps + 1 {
		for j := range steps + 1 {
			u, v := float64(i)/steps, float64(j)/steps
			p, m := s.PointAt(u, v), s.NormalAt(u, v)
			around := n.Cross(p.Minus(c))
			if around.Magnitude() < geom.LengthEps || m.Magnitude() < geom.LengthEps {
				continue
			}
			if math.Abs(m.WithMagnitude(1).Dot(around.WithMagnitude(1))) > 1e-6 {
				return false
			}
		}
	}
	return true
}

// EdgeFrame is the frame of the edge named n, and its kind: along it
// from its start where it is straight, its first axis out of a flat
// face beside it; about its axis where it is a circle or an arc, its
// first axis to its start; and otherwise square to it at its middle
// (EdgePointFrame).
func (t *Topology) EdgeFrame(n Name) (FaceFrame, DatumKind, error) {
	e, err := t.Resolve(n)
	if err != nil {
		return FaceFrame{}, DatumUnknown, err
	}
	ed := &t.Edges[e]
	a, b := t.Vertices[ed.V[0]].P, t.Vertices[ed.V[1]].P
	if straight(ed.Points) {
		dir := b.Minus(a).WithMagnitude(1)
		u := dir.NormalDir(0)
		for _, f := range t.EdgeFaces(e) {
			if f < 0 {
				continue
			}
			if m, ok := t.planarNormal(f); ok {
				u = m
				break
			}
		}
		return FaceFrame{Origin: a, U: frameU(u, dir), N: dir}, DatumLine, nil
	}
	if c, ok := polylineCircle(ed.Points); ok {
		return FaceFrame{Origin: c.centre, U: frameU(c.start.Minus(c.centre), c.n), N: c.n}, DatumAxis, nil
	}
	fr, err := t.edgePointFrame(e)
	if err != nil {
		return FaceFrame{}, DatumUnknown, fmt.Errorf("%s: %w", n, err)
	}
	return fr, DatumEdgePoint, nil
}

// frameU is d square to the unit normal n, or a direction square to n
// where d runs along it.
func frameU(d, n geom.Vector) geom.Vector {
	u := d.Minus(n.ScaledBy(d.Dot(n)))
	if u.Magnitude() < geom.LengthEps {
		return n.NormalDir(0)
	}
	return u.WithMagnitude(1)
}
