package core

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

type testSketch struct {
	entities map[HEntity]*Entity
	params   map[HParam]*Param
}

func newTestSketch() *testSketch {
	return &testSketch{entities: map[HEntity]*Entity{}, params: map[HParam]*Param{}}
}

func (s *testSketch) Entity(h HEntity) *Entity { return s.entities[h] }
func (s *testSketch) Param(h HParam) *Param    { return s.params[h] }

func (s *testSketch) ParamValue(h HParam) (float64, bool) {
	p, ok := s.params[h]
	if !ok {
		return 0, false
	}
	return p.Val, true
}

func (s *testSketch) add(e Entity) *Entity {
	s.entities[e.H] = &e
	return &e
}

func (s *testSketch) param(v uint32, val float64) HParam {
	h := HParam{V: v}
	s.params[h] = &Param{H: h, Val: val}
	return h
}

func evalVector(t *testing.T, sk *testSketch, v ExprVector) geom.Vector {
	t.Helper()
	return geom.Vector{X: v.X.Eval(sk, nil), Y: v.Y.Eval(sk, nil), Z: v.Z.Eval(sk, nil)}
}

// TestEntityNumAndExprsAgree checks that every point, normal and face
// type's numeric accessor matches its symbolic one evaluated at the
// same parameters, and spot-checks the numeric values.
func TestEntityNumAndExprsAgree(t *testing.T) {
	t.Parallel()
	sk := newTestSketch()

	// A workplane whose normal is rotated about z by 90 degrees, with
	// its origin at (1, 2, 3).
	c := math.Sqrt(0.5)
	nq := geom.Quaternion{W: c, X: 0, Y: 0, Z: c}
	sk.add(Entity{H: HEntity{V: 0x10}, Type: EntityPointIn3D,
		Param: [8]HParam{sk.param(1, 1), sk.param(2, 2), sk.param(3, 3)}})
	sk.add(Entity{H: HEntity{V: 0x11}, Type: EntityNormalIn3D,
		Param: [8]HParam{sk.param(4, nq.W), sk.param(5, nq.X), sk.param(6, nq.Y), sk.param(7, nq.Z)}})
	sk.add(Entity{H: HEntity{V: 0x12}, Type: EntityWorkplane,
		Point: [MaxPointsInEntity]HEntity{{V: 0x10}}, Normal: HEntity{V: 0x11}})

	// Transform params: translation, then axis-angle (theta, axis), then
	// helix distance.
	tx, ty, tz := sk.param(10, 0.5), sk.param(11, -1), sk.param(12, 2)
	th, ax, ay, az := sk.param(13, 0.3), sk.param(14, 0), sk.param(15, 0.6), sk.param(16, 0.8)
	dist := sk.param(17, 1.5)
	// A unit quaternion: FACE_N_ROT_TRANS's symbolic normal assumes one.
	m := math.Sqrt(0.95)
	qw, qx, qy, qz := sk.param(18, 0.9/m), sk.param(19, 0.1/m), sk.param(20, -0.3/m), sk.param(21, 0.2/m)
	num := geom.Vector{X: 4, Y: -2, Z: 1}

	points := []Entity{
		{H: HEntity{V: 0x20}, Type: EntityPointIn2D, Workplane: HEntity{V: 0x12},
			Param: [8]HParam{sk.param(30, 5), sk.param(31, 7)}},
		{H: HEntity{V: 0x21}, Type: EntityPointNTrans, NumPoint: num, TimesApplied: 2,
			Param: [8]HParam{tx, ty, tz}},
		{H: HEntity{V: 0x22}, Type: EntityPointNRotTrans, NumPoint: num,
			Param: [8]HParam{tx, ty, tz, qw, qx, qy, qz}},
		{H: HEntity{V: 0x23}, Type: EntityPointNRotAA, NumPoint: num, TimesApplied: -1,
			Param: [8]HParam{tx, ty, tz, th, ax, ay, az}},
		{H: HEntity{V: 0x24}, Type: EntityPointNRotAxisTrans, NumPoint: num, TimesApplied: 2,
			Param: [8]HParam{tx, ty, tz, th, ax, ay, az, dist}},
		{H: HEntity{V: 0x25}, Type: EntityPointNCopy, NumPoint: num},
	}
	for i := range points {
		e := sk.add(points[i])
		got := e.PointGetNum(sk)
		want := evalVector(t, sk, e.PointGetExprs(sk))
		if !got.EqualsTol(want, 1e-12) {
			t.Errorf("type %d: PointGetNum %v, PointGetExprs %v", e.Type, got, want)
		}
	}

	// POINT_IN_2D: origin + u*5 + v*7, with u = +y and v = -x.
	if got := sk.Entity(HEntity{V: 0x20}).PointGetNum(sk); !got.EqualsTol(geom.Vector{X: 1 - 7, Y: 2 + 5, Z: 3}, 1e-12) {
		t.Errorf("POINT_IN_2D = %v", got)
	}
	// POINT_N_TRANS: numPoint + 2*trans.
	if got := sk.Entity(HEntity{V: 0x21}).PointGetNum(sk); !got.EqualsTol(geom.Vector{X: 5, Y: -4, Z: 5}, 1e-12) {
		t.Errorf("POINT_N_TRANS = %v", got)
	}

	normals := []Entity{
		{H: HEntity{V: 0x30}, Type: EntityNormalIn2D, Workplane: HEntity{V: 0x12}},
		{H: HEntity{V: 0x31}, Type: EntityNormalNCopy, NumNormal: nq},
		{H: HEntity{V: 0x32}, Type: EntityNormalNRot, NumNormal: nq,
			Param: [8]HParam{qw, qx, qy, qz}},
		{H: HEntity{V: 0x33}, Type: EntityNormalNRotAA, NumNormal: nq, TimesApplied: 2,
			Param: [8]HParam{th, ax, ay, az}},
	}
	for i := range normals {
		e := sk.add(normals[i])
		q := e.NormalGetExprs(sk)
		got := e.NormalGetNum(sk)
		want := geom.Quaternion{W: q.W.Eval(sk, nil), X: q.X.Eval(sk, nil), Y: q.Y.Eval(sk, nil), Z: q.Z.Eval(sk, nil)}
		if !got.Equals(want) {
			t.Errorf("type %d: NormalGetNum %v, NormalGetExprs %v", e.Type, got, want)
		}
	}

	fn := geom.Quaternion{W: 0, X: 0, Y: 0, Z: 1}
	faces := []Entity{
		{H: HEntity{V: 0x40}, Type: EntityFaceNormalPt, NumNormal: fn,
			Point: [MaxPointsInEntity]HEntity{{V: 0x21}}},
		{H: HEntity{V: 0x41}, Type: EntityFaceXProd, NumPoint: num,
			NumNormal: geom.Quaternion{X: 1, Y: 2, Z: 0}, Param: [8]HParam{tx, ty, tz}},
		{H: HEntity{V: 0x42}, Type: EntityFaceNRotTrans, NumPoint: num, NumNormal: fn,
			Param: [8]HParam{tx, ty, tz, qw, qx, qy, qz}},
		{H: HEntity{V: 0x43}, Type: EntityFaceNTrans, NumPoint: num, NumNormal: fn, TimesApplied: 2,
			Param: [8]HParam{tx, ty, tz}},
		{H: HEntity{V: 0x44}, Type: EntityFaceNRotAA, NumPoint: num, NumNormal: fn, TimesApplied: -1,
			Param: [8]HParam{tx, ty, tz, th, ax, ay, az}},
		{H: HEntity{V: 0x45}, Type: EntityFaceRotNormalPt, NumNormal: fn, TimesApplied: 2,
			Point: [MaxPointsInEntity]HEntity{{V: 0x23}}, Param: [8]HParam{tx, ty, tz, th, ax, ay, az}},
		{H: HEntity{V: 0x46}, Type: EntityFaceNRotAxisTrans, NumPoint: num, NumNormal: fn, TimesApplied: 2,
			Param: [8]HParam{tx, ty, tz, th, ax, ay, az, dist}},
	}
	for i := range faces {
		e := sk.add(faces[i])
		if got, want := e.FaceGetPointNum(sk), evalVector(t, sk, e.FaceGetPointExprs(sk)); !got.EqualsTol(want, 1e-12) {
			t.Errorf("type %d: FaceGetPointNum %v, FaceGetPointExprs %v", e.Type, got, want)
		}
		if e.Type == EntityFaceNRotAxisTrans {
			// SolveSpace has no FaceGetNormalExprs case for this type.
			continue
		}
		if got, want := e.FaceGetNormalNum(sk), evalVector(t, sk, e.FaceGetNormalExprs()); !got.EqualsTol(want, 1e-12) {
			t.Errorf("type %d: FaceGetNormalNum %v, FaceGetNormalExprs %v", e.Type, got, want)
		}
	}
}

func TestEntityPointForceToRoundTrips(t *testing.T) {
	t.Parallel()
	sk := newTestSketch()
	sk.add(Entity{H: HEntity{V: 0x10}, Type: EntityPointIn3D,
		Param: [8]HParam{sk.param(1, 1), sk.param(2, 2), sk.param(3, 3)}})
	sk.add(Entity{H: HEntity{V: 0x11}, Type: EntityNormalIn3D,
		Param: [8]HParam{sk.param(4, 1), sk.param(5, 0), sk.param(6, 0), sk.param(7, 0)}})
	sk.add(Entity{H: HEntity{V: 0x12}, Type: EntityWorkplane,
		Point: [MaxPointsInEntity]HEntity{{V: 0x10}}, Normal: HEntity{V: 0x11}})

	target := geom.Vector{X: -3, Y: 4, Z: 3}
	for _, e := range []Entity{
		{H: HEntity{V: 0x20}, Type: EntityPointIn2D, Workplane: HEntity{V: 0x12},
			Param: [8]HParam{sk.param(30, 0), sk.param(31, 0)}},
		{H: HEntity{V: 0x21}, Type: EntityPointNTrans, NumPoint: geom.Vector{X: 1}, TimesApplied: 2,
			Param: [8]HParam{sk.param(40, 0), sk.param(41, 0), sk.param(42, 0)}},
	} {
		p := sk.add(e)
		p.PointForceTo(sk, target)
		if got := p.PointGetNum(sk); !got.EqualsTol(target, 1e-12) {
			t.Errorf("type %d: PointForceTo(%v) then PointGetNum = %v", p.Type, target, got)
		}
	}
}

func TestEntityArcGetAngles(t *testing.T) {
	t.Parallel()
	sk := newTestSketch()
	pt := func(v uint32, x, y float64) HEntity {
		h := HEntity{V: v}
		sk.add(Entity{H: h, Type: EntityPointIn3D,
			Param: [8]HParam{sk.param(v*4, x), sk.param(v*4+1, y), sk.param(v*4+2, 0)}})
		return h
	}
	c, a, b := pt(1, 0, 0), pt(2, 1, 0), pt(3, 0, 1)
	sk.add(Entity{H: HEntity{V: 9}, Type: EntityNormalNCopy, NumNormal: geom.QuaternionIdentity})
	arc := sk.add(Entity{H: HEntity{V: 10}, Type: EntityArcOfCircle,
		Point: [MaxPointsInEntity]HEntity{c, a, b}, Normal: HEntity{V: 9}})

	ta, tb, dt := arc.ArcGetAngles(sk)
	if ta != 0 || math.Abs(tb-math.Pi/2) > 1e-12 || math.Abs(dt-math.Pi/2) > 1e-12 {
		t.Errorf("ArcGetAngles = %g %g %g", ta, tb, dt)
	}
	if r := arc.CircleGetRadiusNum(sk); r != 1 {
		t.Errorf("CircleGetRadiusNum = %g", r)
	}
	// Coincident endpoints make a full circle, not a zero arc.
	arc.Point[2] = a
	if _, _, dt := arc.ArcGetAngles(sk); math.Abs(dt-2*math.Pi) > 1e-12 {
		t.Errorf("full-circle dtheta = %g", dt)
	}
}
