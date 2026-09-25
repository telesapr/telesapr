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

// TestExprConstantArithmetic verifies basic constant arithmetic.
func TestExprConstantArithmetic(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	// (2 + 3) * 4 = 20
	e := NewConstant(2).Plus(NewConstant(3)).Times(NewConstant(4))
	if v := e.Eval(ctx, nil); v != 20 {
		t.Errorf("2+3*4 = %v, want 20", v)
	}
	// 10 / 2 = 5
	e = NewConstant(10).Div(NewConstant(2))
	if v := e.Eval(ctx, nil); v != 5 {
		t.Errorf("10/2 = %v, want 5", v)
	}
	// 7 - 3 = 4
	e = NewConstant(7).Minus(NewConstant(3))
	if v := e.Eval(ctx, nil); v != 4 {
		t.Errorf("7-3 = %v, want 4", v)
	}
}

// TestExprNegate verifies negation.
func TestExprNegate(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(5).Negate().Eval(ctx, nil); v != -5 {
		t.Errorf("-(5) = %v, want -5", v)
	}
	if v := NewConstant(-3).Negate().Eval(ctx, nil); v != 3 {
		t.Errorf("-(-3) = %v, want 3", v)
	}
}

// TestExprSqrtAndSquare verifies sqrt and square.
func TestExprSqrtAndSquare(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(16).Sqrt().Eval(ctx, nil); v != 4 {
		t.Errorf("sqrt(16) = %v, want 4", v)
	}
	if v := NewConstant(7).Square().Eval(ctx, nil); v != 49 {
		t.Errorf("7^2 = %v, want 49", v)
	}
}

// TestExprAbsFloorCeilRound verifies abs, floor, ceil, round.
func TestExprAbsFloorCeilRound(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(-5).Abs().Eval(ctx, nil); v != 5 {
		t.Errorf("abs(-5) = %v, want 5", v)
	}
	if v := NewConstant(3.7).Floor().Eval(ctx, nil); v != 3 {
		t.Errorf("floor(3.7) = %v, want 3", v)
	}
	if v := NewConstant(3.2).Ceil().Eval(ctx, nil); v != 4 {
		t.Errorf("ceil(3.2) = %v, want 4", v)
	}
	if v := NewConstant(3.5).Round().Eval(ctx, nil); v != 4 {
		t.Errorf("round(3.5) = %v, want 4", v)
	}
}


// TestExprLnExp verifies natural log and exponential.
func TestExprLnExp(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(1).Ln().Eval(ctx, nil); v != 0 {
		t.Errorf("ln(1) = %v, want 0", v)
	}
	if v := NewConstant(0).Exp().Eval(ctx, nil); v != 1 {
		t.Errorf("exp(0) = %v, want 1", v)
	}
}

// TestExprPower verifies power function.
func TestExprPower(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(2).Pow(NewConstant(10)).Eval(ctx, nil); v != 1024 {
		t.Errorf("2^10 = %v, want 1024", v)
	}
}

// TestExprHypot verifies hypot function.
func TestExprHypot(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(3).Hypot(NewConstant(4)).Eval(ctx, nil); v != 5 {
		t.Errorf("hypot(3,4) = %v, want 5", v)
	}
}

// TestExprArctrig verifies inverse trig functions.
func TestExprArctrig(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(1).ASin().Eval(ctx, nil); math.Abs(v-math.Pi/2) > 1e-9 {
		t.Errorf("asin(1) = %v, want pi/2", v)
	}
	if v := NewConstant(0).ACos().Eval(ctx, nil); math.Abs(v-math.Pi/2) > 1e-9 {
		t.Errorf("acos(0) = %v, want pi/2", v)
	}
}

// TestExprVectorMagnitude verifies vector magnitude.
func TestExprVectorMagnitude(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	tests := []struct{ x, y, z, want float64 }{
		{3, 0, 0, 3},
		{0, 4, 0, 4},
		{2, 3, 6, 7}, // 4+9+36=49, sqrt=7
	}
	for _, tc := range tests {
		v := ExprVector{
			X: NewConstant(tc.x),
			Y: NewConstant(tc.y),
			Z: NewConstant(tc.z),
		}.Magnitude().Eval(ctx, nil)
		if math.Abs(v-tc.want) > 1e-9 {
			t.Errorf("|(%g,%g,%g)| = %v, want %g", tc.x, tc.y, tc.z, v, tc.want)
		}
	}
}

// TestExprVectorDotAndCross verifies dot and cross products.
func TestExprVectorDotAndCross(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	a := ExprVector{X: NewConstant(1), Y: NewConstant(2), Z: NewConstant(3)}
	b := ExprVector{X: NewConstant(4), Y: NewConstant(5), Z: NewConstant(6)}
	if v := a.Dot(b).Eval(ctx, nil); v != 32 {
		t.Errorf("dot = %v, want 32", v)
	}
	x := ExprVector{X: NewConstant(1)}.Cross(ExprVector{Y: NewConstant(1)})
	if x.Z.Eval(ctx, nil) != 1 {
		t.Errorf("X cross Y = (0,0,%v), want (0,0,1)", x.Z.Eval(ctx, nil))
	}
}

// TestExprVectorPlusMinus verifies vector arithmetic.
func TestExprVectorPlusMinus(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	a := ExprVector{X: NewConstant(5), Y: NewConstant(3), Z: NewConstant(0)}
	b := ExprVector{X: NewConstant(2), Y: NewConstant(1), Z: NewConstant(0)}
	d := a.Minus(b)
	if v := d.X.Eval(ctx, nil); v != 3 {
		t.Errorf("a-b x = %v, want 3", v)
	}
	if v := d.Y.Eval(ctx, nil); v != 2 {
		t.Errorf("a-b y = %v, want 2", v)
	}
	s := a.Plus(b)
	if v := s.X.Eval(ctx, nil); v != 7 {
		t.Errorf("a+b x = %v, want 7", v)
	}
}

// TestExprQuaternionRotateIdentity checks identity rotation.
func TestExprQuaternionRotateIdentity(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	q := ExprQuaternion{W: NewConstant(1), X: NewConstant(0), Y: NewConstant(0), Z: NewConstant(0)}
	v := q.Rotate(ExprVector{X: NewConstant(1), Y: NewConstant(2), Z: NewConstant(3)})
	if v.X.Eval(ctx, nil) != 1 || v.Y.Eval(ctx, nil) != 2 || v.Z.Eval(ctx, nil) != 3 {
		t.Errorf("identity rotation changed the vector")
	}
}

// TestExprQuaternionRotate180AroundZ checks 180-degree rotation around Z.
func TestExprQuaternionRotate180AroundZ(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	q := ExprQuaternion{W: NewConstant(0), X: NewConstant(0), Y: NewConstant(0), Z: NewConstant(1)}
	v := q.Rotate(ExprVector{X: NewConstant(1), Y: NewConstant(0), Z: NewConstant(0)})
	if v.X.Eval(ctx, nil) != -1 {
		t.Errorf("rotate x 180 around z: got x=%v, want -1", v.X.Eval(ctx, nil))
	}
}

// TestExprQuaternionRotate90AroundZ checks 90-degree rotation around Z.
func TestExprQuaternionRotate90AroundZ(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	// Quaternion for 90-degree rotation around Z axis: (cos45, 0, 0, sin45).
	q := ExprQuaternion{
		W: NewConstant(0.7071067811865476),
		X: NewConstant(0),
		Y: NewConstant(0),
		Z: NewConstant(0.7071067811865476),
	}
	v := q.Rotate(ExprVector{X: NewConstant(1), Y: NewConstant(0), Z: NewConstant(0)})
	if v.X.Eval(ctx, nil) > 1e-9 {
		t.Errorf("rotate x 90 around z: got x=%v, want 0", v.X.Eval(ctx, nil))
	}
	if v.Y.Eval(ctx, nil) < 1-1e-9 {
		t.Errorf("rotate x 90 around z: got y=%v, want 1", v.Y.Eval(ctx, nil))
	}
}

// TestExprSinCosSumToOne checks sin^2 + cos^2 = 1.
func TestExprSinCosSumToOne(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	s := NewConstant(0.7).Sin().Square()
	c := NewConstant(0.7).Cos().Square()
	if v := s.Plus(c).Eval(ctx, nil); math.Abs(v-1) > 1e-9 {
		t.Errorf("sin^2 + cos^2 = %v, want 1", v)
	}
}

// TestExprDivByZero verifies that division by zero produces
// infinity (not panic). The behavior depends on Go's float semantics.
func TestExprDivByZero(t *testing.T) {
	ctx := stubParamProvider{values: map[HParam]float64{}}
	v := NewConstant(1).Div(NewConstant(0)).Eval(ctx, nil)
	if !isInf(v) {
		t.Errorf("1/0 = %v, expected inf", v)
	}
}

func isInf(f float64) bool {
	return f > 1e308 || f < -1e308
}
