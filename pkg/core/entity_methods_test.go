package core

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestEntityPointGetExprs(t *testing.T) {
	e := &Entity{
		EntityBase: EntityBase{
			Type:  EntityPointIn3D,
			Param: [8]HParam{{V: 1}, {V: 2}, {V: 3}, {V: 4}, {V: 5}, {V: 6}, {V: 7}, {V: 8}},
		},
	}
	res := newTestResolver(map[HParam]float64{
		{1}: 1, {2}: 2, {3}: 3,
	})
	got := e.PointGetExprs(res)
	vx := got.X.Eval(res, nil)
	vy := got.Y.Eval(res, nil)
	vz := got.Z.Eval(res, nil)
	if vx != 1 || vy != 2 || vz != 3 {
		t.Fatalf("PointGetExprs = (%v,%v,%v), want (1,2,3)", vx, vy, vz)
	}
}

func TestEntityPointForceParamTo(t *testing.T) {
	counts := make(map[HParam]float64)
	res := &countingResolver{counts: counts}
	e := &Entity{
		EntityBase: EntityBase{
			Type:  EntityPointIn3D,
			Param: [8]HParam{{V: 10}, {V: 20}, {V: 30}},
		},
	}
	e.PointForceParamTo(res, geom.Vector{X: 1.5, Y: 2.5, Z: 3.5})
	if e.ActPoint.DistanceTo(geom.Vector{X: 1.5, Y: 2.5, Z: 3.5}) > 1e-9 {
		t.Fatalf("PointForceParamTo did not update ActPoint: %v", e.ActPoint)
	}
	if counts[HParam{V: 10}] != 1.5 || counts[HParam{V: 20}] != 2.5 || counts[HParam{V: 30}] != 3.5 {
		t.Fatalf("resolver did not receive writes: %v", counts)
	}
}

func TestEntityVectorGetNum(t *testing.T) {
	res := newTestResolver(nil)
	e := &Entity{
		EntityBase: EntityBase{
			Type:  EntityLineSegment,
			Point: [12]HEntity{{V: 1}, {V: 2}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}},
		},
	}
	res.points[HEntity{V: 1}] = geom.Vector{X: 1, Y: 0, Z: 0}
	res.points[HEntity{V: 2}] = geom.Vector{X: 4, Y: 3, Z: 0}
	v := e.VectorGetNum(res)
	if v.DistanceTo(geom.Vector{X: 3, Y: 3, Z: 0}) > 1e-9 {
		t.Fatalf("VectorGetNum = %v, want (3,3,0)", v)
	}
}

func TestEntityCircleGetRadiusNum(t *testing.T) {
	e := &Entity{
		EntityBase: EntityBase{Type: EntityCircle},
		ActDistance: 5.0,
	}
	if e.CircleGetRadiusNum() != 5.0 {
		t.Fatalf("CircleGetRadiusNum = %v, want 5", e.CircleGetRadiusNum())
	}
}

func TestEntityGetAxisAngleQuaternion(t *testing.T) {
	// Quaternion representing a 90° rotation around +Z.
	e := &Entity{
		ActNormal: geom.NewQuaternion(
			math.Cos(math.Pi/4), 0, 0, math.Sin(math.Pi/4),
		),
	}
	axis, angle := e.GetAxisAngleQuaternion(nil)
	if math.Abs(angle-math.Pi/2) > 1e-6 {
		t.Fatalf("angle = %f, want pi/2", angle)
	}
	if axis.DistanceTo(geom.Vector{Z: 1}) > 1e-6 {
		t.Fatalf("axis = %v, want (0,0,1)", axis)
	}
}

func TestEntityArcGetAngles(t *testing.T) {
	e := &Entity{EntityBase: EntityBase{Type: EntityArcOfCircle}}
	thetaa, thetaf, dtheta := e.ArcGetAngles()
	if thetaa != 0 || math.Abs(thetaf-2*math.Pi) > 1e-9 || math.Abs(dtheta-2*math.Pi) > 1e-9 {
		t.Fatalf("ArcGetAngles = (%v, %v, %v)", thetaa, thetaf, dtheta)
	}
}

func TestEntityDistanceForceTo(t *testing.T) {
	e := &Entity{EntityBase: EntityBase{Type: EntityDistance}}
	e.DistanceForceTo(3.14)
	if e.ActDistance != 3.14 {
		t.Fatalf("DistanceForceTo: ActDistance = %v, want 3.14", e.ActDistance)
	}
}

// newTestResolver creates a ParamResolver with the supplied param
// values and a default empty map.
func newTestResolver(params map[HParam]float64) *testResolver {
	if params == nil {
		params = make(map[HParam]float64)
	}
	return &testResolver{
		params: params,
		points: make(map[HEntity]geom.Vector),
		vecs:   make(map[HEntity]geom.Vector),
	}
}

type testResolver struct {
	params map[HParam]float64
	points map[HEntity]geom.Vector
	vecs   map[HEntity]geom.Vector
}

func (r *testResolver) ParamValue(h HParam) (float64, bool) {
	v, ok := r.params[h]
	return v, ok
}

func (r *testResolver) SetParam(h HParam, v float64) {
	r.params[h] = v
}

func (r *testResolver) PointValue(h HEntity) (geom.Vector, bool) {
	v, ok := r.points[h]
	return v, ok
}

func (r *testResolver) VectorValue(h HEntity) (geom.Vector, bool) {
	v, ok := r.vecs[h]
	return v, ok
}

type countingResolver struct {
	counts map[HParam]float64
	points map[HEntity]geom.Vector
}

func (r *countingResolver) ParamValue(h HParam) (float64, bool) { return 0, false }
func (r *countingResolver) SetParam(h HParam, v float64)        { r.counts[h] = v }
func (r *countingResolver) PointValue(h HEntity) (geom.Vector, bool) {
	v, ok := r.points[h]
	return v, ok
}
func (r *countingResolver) VectorValue(h HEntity) (geom.Vector, bool) { return geom.Vector{}, false }
