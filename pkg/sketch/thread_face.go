package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A thread made on a face starts where a user points at a shaft or a
// hole: a cylindrical face of the solid, or the circular edge at one of
// its ends. The face gives the thread its axis, its end, which way it
// runs into the part, and whether it is a screw's or a nut's. It is
// drawn on a circle as any thread is, a construction circle in a datum
// at the face's end (GroupWorkplaneByDatum with two faces), which
// follows the face when the part changes upstream.

// ThreadPlace is where a thread made on a round face of the solid
// starts: the face, the end it starts at and the end it runs to.
type ThreadPlace struct {
	// Face is the cylindrical face.
	Face srf.FaceID
	// At is the end the thread starts at, Far the other.
	At, Far topo.AxisEnd
}

// ErrNotCylinder is returned for a face or an edge a thread can't be
// made on.
var ErrNotCylinder = errors.New("a thread is made on a cylindrical face or a circular edge at its end")

// threadTopology is the topology of the solid at the active group,
// which a thread made on it starts from.
func (sk *Sketch) threadTopology() (*topo.Topology, error) {
	return solidTopology(sk, sk.solidAt(sk.Active))
}

// ThreadPlaceOnFace is where a thread on the cylindrical face face of
// the solid at the active group starts: at the end of the part where
// the face has one such end (a shaft's tip, the mouth of a blind hole),
// otherwise at its upper end, the axis read with its largest component
// up. other starts it at the end that rule doesn't pick.
func (sk *Sketch) ThreadPlaceOnFace(face srf.FaceID, other bool) (ThreadPlace, error) {
	t, err := sk.threadTopology()
	if err != nil {
		return ThreadPlace{}, err
	}
	ends, err := t.AxisEnds(face)
	if err != nil {
		return ThreadPlace{}, fmt.Errorf("%w: %w", ErrNotCylinder, err)
	}
	first := 0
	switch {
	case ends[0].Free && !ends[1].Free:
	case ends[1].Free && !ends[0].Free:
		first = 1
	case upwards(ends[0]):
		// The first end is the lower; the second looks down from above.
		first = 1
	}
	if other {
		first = 1 - first
	}
	return ThreadPlace{Face: face, At: ends[first], Far: ends[1-first]}, nil
}

// upwards tells whether the end's frame points up its axis: along the
// axis's largest component when that is positive.
func upwards(e topo.AxisEnd) bool {
	n := e.Frame.N
	big := n.X
	if math.Abs(n.Y) > math.Abs(big) {
		big = n.Y
	}
	if math.Abs(n.Z) > math.Abs(big) {
		big = n.Z
	}
	return big > 0
}

// ThreadPlaceAtEdge is where a thread starts at the circular edge named
// n of the solid at the active group: the end of the cylindrical face
// the edge bounds.
func (sk *Sketch) ThreadPlaceAtEdge(n topo.Name) (ThreadPlace, error) {
	t, err := sk.threadTopology()
	if err != nil {
		return ThreadPlace{}, err
	}
	e, err := t.Resolve(n)
	if err != nil {
		return ThreadPlace{}, err
	}
	face, at, err := t.AxisEndAt(e)
	if err != nil {
		return ThreadPlace{}, fmt.Errorf("%w: %w", ErrNotCylinder, err)
	}
	ends, err := t.AxisEnds(face)
	if err != nil {
		return ThreadPlace{}, fmt.Errorf("%w: %w", ErrNotCylinder, err)
	}
	far := ends[0]
	if far.End == at.End && far.Frame.Origin.Equals(at.Frame.Origin) {
		far = ends[1]
	}
	return ThreadPlace{Face: face, At: at, Far: far}, nil
}

// Diameter is the face's diameter.
func (p ThreadPlace) Diameter() float64 { return 2 * p.At.Radius }

// Options are o as the place sets them: a nut's thread in a hole and a
// screw's on a shaft, run along the face from its end, and with
// FullLength as long as the face, through where the face reaches the end
// of the part.
func (p ThreadPlace) Options(o ThreadOptions) ThreadOptions {
	o.Internal = p.At.Hole
	o.Reverse = false
	if o.FullLength {
		// Measured from the mesh of the face's edges, the length is
		// kept to a nanometre, as a typed one would be.
		o.Length, o.LengthExpr = math.Round(p.At.Length*1e6)/1e6, ""
		o.Through = p.Far.Free
		if o.Through {
			o.RunOut = 0
		}
	}
	return o
}

// CheckThreadOnFace reports why AddThreadOnFace would refuse to make
// the thread, without changing anything.
func (sk *Sketch) CheckThreadOnFace(p ThreadPlace, o ThreadOptions) error {
	o = p.Options(o)
	if o.LengthExpr != "" {
		if _, _, err := sk.parseNamed(o.LengthExpr); err != nil {
			return fmt.Errorf("thread length: %w", err)
		}
	}
	l, err := o.layout(p.At.Radius)
	if err != nil {
		return err
	}
	if err := sk.checkRoundRoot(l); err != nil {
		return err
	}
	// The ridges reach past the root into the part; a hole wider, or a
	// shaft thinner, than half way there leaves them barely touching it.
	root, reach := l.profile.RootRadius, math.Abs(l.baseR-l.profile.RootRadius)/2
	switch {
	case p.At.Hole && p.At.Radius > root+reach:
		return fmt.Errorf("the hole, %.4g mm across, is wider than the thread's root, %.4g mm", p.Diameter(), 2*root)
	case !p.At.Hole && p.At.Radius < root-reach:
		return fmt.Errorf("the shaft, %.4g mm across, is thinner than the thread's root, %.4g mm", p.Diameter(), 2*root)
	}
	return nil
}

// AddThreadOnFace makes a screw thread on the round face of p, from its
// end p.At along the face, with o as p sets it (Options): a datum at the
// face's end, with a construction circle of the face's diameter on its
// axis, then the thread's groups on that circle, as AddThread makes
// them.
func (sk *Sketch) AddThreadOnFace(p ThreadPlace, o ThreadOptions) (ThreadResult, error) {
	if err := sk.CheckThreadOnFace(p, o); err != nil {
		return ThreadResult{}, err
	}
	o = p.Options(o)

	g := NewGroup()
	g.Type = GroupDrawingWorkplane
	g.Subtype = GroupWorkplaneByDatum
	g.Name = o.Spec.String() + "-end"
	g.SolidFaces = []srf.FaceID{p.Face, p.At.End}
	g.Predef.Q, g.Predef.At = p.At.Frame.Quaternion(), p.At.Frame.Origin
	g.datumKind = topo.DatumAxis
	wg := sk.addGroupAfterActive(g)
	wg.ActiveWorkplane = wg.H.Entity(0)

	hr := sk.AddRequestOfType(RequestCircle, false)
	sk.Request(hr).Construction = true
	sk.MustEntity(hr.Entity(1)).PointForceTo(sk, p.At.Frame.Origin)
	sk.MustEntity(hr.Entity(64)).DistanceForceTo(sk, p.At.Radius)
	sk.ConstrainCoincident(hr.Entity(1), wg.H.Entity(2))
	c := core.Constraint{Group: wg.H, Workplane: wg.H.Entity(0), Type: core.ConstraintDiameter,
		EntityA: hr.Entity(0), ValA: p.Diameter()}
	sk.AddConstraintRemember(&c, false)
	sk.GenerateAllTyped(GenerateRegen)

	return sk.AddThread(hr.Entity(0), o)
}

// threadPlaceOf is where the thread on the circle he was made, when it
// was made on a face: the face and the end its datum names, found again
// in the solid before it.
func (sk *Sketch) threadPlaceOf(he core.HEntity) (ThreadPlace, bool) {
	e := sk.Entity(he)
	if e == nil {
		return ThreadPlace{}, false
	}
	g := sk.Group(e.Group)
	if g == nil || g.Subtype != GroupWorkplaneByDatum || len(g.SolidFaces) != 2 {
		return ThreadPlace{}, false
	}
	prev := g.PreviousGroup(sk)
	if prev == nil {
		return ThreadPlace{}, false
	}
	t, err := solidTopology(sk, sk.solidAt(prev.H))
	if err != nil {
		return ThreadPlace{}, false
	}
	ends, err := t.AxisEnds(g.SolidFaces[0])
	if err != nil {
		return ThreadPlace{}, false
	}
	for i, end := range ends {
		if end.End == g.SolidFaces[1] {
			return ThreadPlace{Face: g.SolidFaces[0], At: end, Far: ends[1-i]}, true
		}
	}
	return ThreadPlace{}, false
}

// ThreadPlaceOf is where the thread whose helix group is hg was made,
// and whether it was made on a face (AddThreadOnFace) and that face is
// still there.
func (sk *Sketch) ThreadPlaceOf(hg core.HGroup) (ThreadPlace, bool) {
	g := sk.Group(hg)
	if g == nil || !g.Thread.IsThread() {
		return ThreadPlace{}, false
	}
	return sk.threadPlaceOf(g.Thread.Circle)
}

// followFace is o for a thread on the circle he: as its face sets it
// (Options) when it was made on one, its length measured again when it
// is FullLength; as it is otherwise.
func (sk *Sketch) followFace(he core.HEntity, o ThreadOptions) ThreadOptions {
	if p, ok := sk.threadPlaceOf(he); ok {
		return p.Options(o)
	}
	return o
}
