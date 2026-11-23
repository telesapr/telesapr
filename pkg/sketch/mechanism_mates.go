package sketch

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// A mechanism's plan (mechanism_plan.go) reads its constraints as mates
// between rigid bodies: the ground, which is everything before the
// mechanism and holds still, and its parts, each placed by its link
// group's move and turn. A mate is a constraint of a kind the plan
// knows, between features of at most two bodies: points, and the
// directions of lines, normals and faces. What the mates of the
// assembly menu make is all of these kinds: Hinge is a point on a
// point and a point on a line, Concentric a parallel and two distances
// along directions, Flush a parallel and a point on a face, a driven
// hinge adds an axis angle, a driven concentric or a screw a distance
// along the axis; a slide held in a workplane, as the engine's piston,
// is a point on a line and a parallel seen in it. Any other constraint
// in the mechanism leaves the mechanism to the solve alone.

// mateKind is what a mate holds.
type mateKind int

const (
	// mateCoincident puts two points on each other, in 3d.
	mateCoincident mateKind = iota
	// matePointOnLine puts a point on a line, in 3d.
	matePointOnLine
	// matePointOnPlane puts a point on the plane of a face or a
	// workplane.
	matePointOnPlane
	// mateParallel makes two directions parallel, in 3d.
	mateParallel
	// mateAlong holds two points a distance apart along a direction.
	mateAlong
	// mateAxisAngle holds a point at an angle about an axis.
	mateAxisAngle
	// mateFlatCoincident puts two points on each other, seen in a
	// workplane.
	mateFlatCoincident
	// mateFlatOnLine puts a point on a line, seen in a workplane.
	mateFlatOnLine
	// mateFlatParallel makes two directions parallel, seen in a
	// workplane.
	mateFlatParallel
)

// planMate is a mate as compiled: its constraint, what it holds, the
// entities it holds and the bodies they are on (0 the ground, i+1 the
// i-th group of the mechanism): one or two parts, and the ground.
type planMate struct {
	kind mateKind
	c    core.HConstraint
	// pts are its points; for an axis angle the axis's point, then the
	// point turned.
	pts []core.HEntity
	// vecs are its directions: a line, a normal or a face; for a point
	// on a line or a plane, the line or the plane (a face or a
	// workplane); for an axis angle, its axis.
	vecs []core.HEntity
	// ptBody and vecBody are the bodies of pts and vecs; bodies the two
	// the mate joins.
	ptBody, vecBody []int
	bodies          []int
}

// joins reports whether the mate joins body b to one placed before it
// in order, or to the ground: the mates that place b.
func (m *planMate) joins(b int, placed func(int) bool) bool {
	if !slices.Contains(m.bodies, b) {
		return false
	}
	for _, o := range m.bodies {
		if o != b && !placed(o) {
			return false
		}
	}
	return true
}

// bodyOf is the body entity he moves with in the mechanism hgs: 0 for
// the ground, i+1 for a rigid copy in the i-th group's part, placed by
// the group's move and turn; ok false for an entity of a group of the
// mechanism that its placement does not move rigidly.
func (sk *Sketch) bodyOf(hgs []core.HGroup, he core.HEntity) (b int, ok bool) {
	e := sk.Entity(he)
	if e == nil {
		return 0, false
	}
	i := slices.Index(hgs, e.Group)
	if i < 0 {
		return 0, true
	}
	hg := hgs[i]
	moves := func(from, n int) bool {
		for k := range n {
			if e.Param[from+k] != hg.Param(from+k) {
				return false
			}
		}
		return true
	}
	switch e.Type {
	case core.EntityPointNRotTrans, core.EntityFaceNRotTrans:
		ok = moves(0, 7)
	case core.EntityNormalNRot:
		for k := range 4 {
			if e.Param[k] != hg.Param(3+k) {
				return 0, false
			}
		}
		ok = true
	case core.EntityLineSegment:
		b0, ok0 := sk.bodyOf(hgs, e.Point[0])
		b1, ok1 := sk.bodyOf(hgs, e.Point[1])
		ok = ok0 && ok1 && b0 == i+1 && b1 == i+1
	}
	return i + 1, ok
}

// hasDirection reports whether entity he has a direction a mate can
// hold: a line, a normal or a face.
func (sk *Sketch) hasDirection(he core.HEntity) bool {
	e := sk.Entity(he)
	if e == nil {
		return false
	}
	switch e.Type {
	case core.EntityLineSegment, core.EntityNormalIn3D, core.EntityNormalIn2D, core.EntityNormalNCopy,
		core.EntityNormalNRot:
		return true
	}
	return e.IsFace()
}

// mateOf reads constraint c of the mechanism hgs as a mate; ok false
// for a constraint of a kind, or between entities, the plan does not
// know.
func (sk *Sketch) mateOf(hgs []core.HGroup, c *core.Constraint) (m planMate, ok bool) {
	m.c = c.H
	free := core.IsFreeIn3D(c.Workplane)
	if !free {
		// A workplane the mechanism moves would move what is seen in it.
		if w := sk.Entity(c.Workplane); w == nil || !w.IsWorkplane() || slices.Contains(hgs, w.Group) {
			return m, false
		}
	}
	isPoint := func(h core.HEntity) bool { e := sk.Entity(h); return e != nil && e.IsPoint() }
	isLine := func(h core.HEntity) bool { e := sk.Entity(h); return e != nil && e.Type == core.EntityLineSegment }
	switch c.Type {
	case core.ConstraintPointsCoincident:
		m.kind = mateCoincident
		if !free {
			m.kind = mateFlatCoincident
		}
		m.pts = []core.HEntity{c.PtA, c.PtB}
	case core.ConstraintPtOnLine:
		if !isLine(c.EntityA) {
			return m, false
		}
		m.kind = matePointOnLine
		if !free {
			m.kind = mateFlatOnLine
		}
		m.pts, m.vecs = []core.HEntity{c.PtA}, []core.HEntity{c.EntityA}
	case core.ConstraintPtOnFace, core.ConstraintPtInPlane:
		e := sk.Entity(c.EntityA)
		if e == nil || !(e.IsFace() && c.Type == core.ConstraintPtOnFace || e.IsWorkplane() && c.Type == core.ConstraintPtInPlane) {
			return m, false
		}
		if e.IsWorkplane() && slices.Contains(hgs, e.Group) {
			return m, false
		}
		m.kind = matePointOnPlane
		m.pts, m.vecs = []core.HEntity{c.PtA}, []core.HEntity{c.EntityA}
	case core.ConstraintParallel:
		m.kind = mateParallel
		if !free {
			m.kind = mateFlatParallel
		}
		m.vecs = []core.HEntity{c.EntityA, c.EntityB}
	case core.ConstraintProjPtDistance:
		m.kind = mateAlong
		m.pts, m.vecs = []core.HEntity{c.PtA, c.PtB}, []core.HEntity{c.EntityA}
	case core.ConstraintAxisAngle:
		m.kind = mateAxisAngle
		m.pts, m.vecs = []core.HEntity{c.PtA, c.PtB}, []core.HEntity{c.EntityA}
	default:
		return m, false
	}
	for _, h := range m.pts {
		if !isPoint(h) {
			return m, false
		}
	}
	for _, h := range m.vecs {
		if m.kind == matePointOnPlane {
			continue
		}
		if !sk.hasDirection(h) {
			return m, false
		}
	}
	for i, h := range slices.Concat(m.pts, m.vecs) {
		b, rigid := sk.bodyOf(hgs, h)
		if !rigid {
			return m, false
		}
		if i < len(m.pts) {
			m.ptBody = append(m.ptBody, b)
		} else {
			m.vecBody = append(m.vecBody, b)
		}
		if !slices.Contains(m.bodies, b) {
			m.bodies = append(m.bodies, b)
		}
	}
	// Between bodies, one or two of them parts: a constraint within one
	// part, or on the ground alone, no placement changes. A direction
	// may be the ground's between two parts, as the reference axes a
	// concentric mate's distances are along.
	parts := len(m.bodies)
	if slices.Contains(m.bodies, 0) {
		parts--
	}
	return m, len(m.bodies) >= 2 && parts <= 2
}

// feat is a point or a direction of a body, in the body's own frame:
// where it is when the body is placed at its start, the move and the
// turn it had when the plan's run began. The ground's are where they
// are.
type feat struct {
	body  int
	local geom.Vector
}

// place is a body's placement: its move and its turn, a unit
// quaternion. The ground's is none.
type place struct {
	t geom.Vector
	q geom.Quaternion
}

// point is f, a point, where placements pl put it.
func (f feat) point(pl []place) geom.Vector {
	if f.body == 0 {
		return f.local
	}
	p := pl[f.body]
	return p.t.Plus(p.q.Rotate(f.local))
}

// dir is f, a direction, where placements pl turn it.
func (f feat) dir(pl []place) geom.Vector {
	if f.body == 0 {
		return f.local
	}
	return pl[f.body].q.Rotate(f.local)
}

// liveMate is a mate read where the mechanism is when a run of its
// plan begins: its features in their bodies' frames, and its value.
type liveMate struct {
	kind   mateKind
	bodies []int
	// p are its points, o a line's or a plane's point, d its
	// directions: a line's, a plane's normal, the two parallels, the
	// direction a distance is along; u and v an axis angle's frame.
	p    []feat
	o    feat
	d    []feat
	u, v feat
	// wu and wv are a workplane's directions, for a mate seen in it.
	wu, wv geom.Vector
	// val is a distance's value; ca and sa an axis angle's cosine and
	// sine.
	val, ca, sa float64
}

// residual appends to out what mate m misses by with the bodies at pl:
// zero where it holds. Each is a length, or the sine of an angle; and
// each is affine in a body's move while its turn is held, which the
// placement of its move (settleMove) counts on.
func (m *liveMate) residual(pl []place, out []float64) []float64 {
	flat := func(v geom.Vector) (float64, float64) { return v.Dot(m.wu), v.Dot(m.wv) }
	switch m.kind {
	case mateCoincident:
		d := m.p[0].point(pl).Minus(m.p[1].point(pl))
		return append(out, d.X, d.Y, d.Z)
	case mateFlatCoincident:
		u, v := flat(m.p[0].point(pl).Minus(m.p[1].point(pl)))
		return append(out, u, v)
	case matePointOnLine:
		d := m.d[0].dir(pl).WithMagnitude(1)
		off := m.p[0].point(pl).Minus(m.o.point(pl))
		off = off.Minus(d.ScaledBy(off.Dot(d)))
		return append(out, off.X, off.Y, off.Z)
	case mateFlatOnLine:
		du, dv := flat(m.d[0].dir(pl))
		pu, pv := flat(m.p[0].point(pl).Minus(m.o.point(pl)))
		return append(out, (du*pv-dv*pu)/math.Hypot(du, dv))
	case matePointOnPlane:
		n := m.d[0].dir(pl).WithMagnitude(1)
		return append(out, m.p[0].point(pl).Minus(m.o.point(pl)).Dot(n))
	case mateParallel:
		a, b := m.d[0].dir(pl).WithMagnitude(1), m.d[1].dir(pl).WithMagnitude(1)
		x := a.Cross(b)
		return append(out, x.X, x.Y, x.Z)
	case mateFlatParallel:
		au, av := flat(m.d[0].dir(pl))
		bu, bv := flat(m.d[1].dir(pl))
		return append(out, (au*bv-av*bu)/(math.Hypot(au, av)*math.Hypot(bu, bv)))
	case mateAlong:
		d := m.d[0].dir(pl).WithMagnitude(1)
		return append(out, m.p[1].point(pl).Minus(m.p[0].point(pl)).Dot(d)-m.val)
	case mateAxisAngle:
		across := m.v.dir(pl).ScaledBy(m.ca).Minus(m.u.dir(pl).ScaledBy(m.sa))
		return append(out, m.p[1].point(pl).Minus(m.p[0].point(pl)).Dot(across))
	}
	panic("unknown mate kind")
}

// liveMateOf reads mate m where the mechanism is now, its features in
// the frames of their bodies at start (each body's move and turn when
// the run began).
func (sk *Sketch) liveMateOf(m *planMate, start []place) liveMate {
	c := sk.Constraint(m.c)
	lm := liveMate{kind: m.kind, bodies: m.bodies, val: c.ValA}
	toPoint := func(b int, w geom.Vector) feat {
		if b == 0 {
			return feat{0, w}
		}
		return feat{b, start[b].q.Inverse().Rotate(w.Minus(start[b].t))}
	}
	toDir := func(b int, w geom.Vector) feat {
		if b == 0 {
			return feat{0, w}
		}
		return feat{b, start[b].q.Inverse().Rotate(w)}
	}
	for i, h := range m.pts {
		lm.p = append(lm.p, toPoint(m.ptBody[i], sk.MustEntity(h).PointGetNum(sk)))
	}
	switch m.kind {
	case matePointOnLine, mateFlatOnLine:
		ln, b := sk.MustEntity(m.vecs[0]), m.vecBody[0]
		lm.o = toPoint(b, sk.MustEntity(ln.Point[0]).PointGetNum(sk))
		lm.d = []feat{toDir(b, ln.VectorGetNum(sk))}
	case matePointOnPlane:
		e, b := sk.MustEntity(m.vecs[0]), m.vecBody[0]
		if e.IsWorkplane() {
			lm.o, lm.d = toPoint(b, e.WorkplaneGetOffset(sk)), []feat{toDir(b, e.NormalEntity(sk).NormalN(sk))}
		} else {
			lm.o, lm.d = toPoint(b, e.FaceGetPointNum(sk)), []feat{toDir(b, e.FaceGetNormalNum(sk))}
		}
	case mateAxisAngle:
		_, u, v := c.AxisFrameNum(sk)
		b := m.vecBody[0]
		lm.u, lm.v = toDir(b, u), toDir(b, v)
		a := c.ValA * math.Pi / 180
		lm.ca, lm.sa = math.Cos(a), math.Sin(a)
	default:
		for i, h := range m.vecs {
			lm.d = append(lm.d, toDir(m.vecBody[i], sk.MustEntity(h).VectorGetNum(sk)))
		}
	}
	if !core.IsFreeIn3D(c.Workplane) {
		w := sk.MustEntity(c.Workplane).NormalEntity(sk)
		lm.wu, lm.wv = w.NormalU(sk), w.NormalV(sk)
	}
	return lm
}
