package core

import (
	"math"
	"testing"
)

type stubParamProvider struct {
	values map[HParam]float64
}

func (s stubParamProvider) ParamValue(h HParam) (float64, bool) {
	v, ok := s.values[h]
	return v, ok
}

func TestExprEvalAndDerivative(t *testing.T) {
	p1 := HParam{V: 0x00010001}
	p2 := HParam{V: 0x00010002}

	expr := NewParam(p1).Plus(NewParam(p2)).Div(NewConstant(2))

	ctx := stubParamProvider{
		values: map[HParam]float64{
			p1: 6,
			p2: 2,
		},
	}

	got := expr.Eval(ctx, nil)
	if got != 4 {
		t.Fatalf("eval: got %f, want 4", got)
	}

	deriv := expr.Derivative(p1).FoldConstants()
	derivVal := deriv.Eval(ctx, nil)
	if math.Abs(derivVal-0.5) > 1e-9 {
		t.Fatalf("derivative: got %f want 0.5", derivVal)
	}
}

func TestExprQuaternionRotate(t *testing.T) {
	angle := math.Pi / 2
	q := ExprQuaternion{
		W: NewConstant(math.Cos(angle / 2)),
		X: NewConstant(0),
		Y: NewConstant(math.Sin(angle / 2)),
		Z: NewConstant(0),
	}
	vec := ExprVector{
		X: NewConstant(1),
		Y: NewConstant(0),
		Z: NewConstant(0),
	}
	res := q.Rotate(vec)

	ctx := stubParamProvider{values: map[HParam]float64{}}
	x := res.X.Eval(ctx, nil)
	y := res.Y.Eval(ctx, nil)
	z := res.Z.Eval(ctx, nil)
	if math.Abs(x) > 1e-9 || math.Abs(y) > 1e-9 || math.Abs(z+1) > 1e-9 {
		t.Fatalf("rotation: got (%f,%f,%f) want (0,0,-1)", x, y, z)
	}
}
