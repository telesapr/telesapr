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
	t.Parallel()
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
	t.Parallel()
	vec := func(x, y, z float64) ExprVector {
		return ExprVector{X: NewConstant(x), Y: NewConstant(y), Z: NewConstant(z)}
	}
	quat := func(w, x, y, z float64) ExprQuaternion {
		return ExprQuaternion{W: NewConstant(w), X: NewConstant(x), Y: NewConstant(y), Z: NewConstant(z)}
	}
	h := math.Sqrt2 / 2
	cases := []struct {
		name    string
		q       ExprQuaternion
		v       ExprVector
		x, y, z float64
		tol     float64 // 0: the rotation is exact
	}{
		{"90 about y", quat(math.Cos(math.Pi/4), 0, math.Sin(math.Pi/4), 0), vec(1, 0, 0), 0, 0, -1, 1e-9},
		{"identity", quat(1, 0, 0, 0), vec(1, 2, 3), 1, 2, 3, 0},
		{"180 about z", quat(0, 0, 0, 1), vec(1, 0, 0), -1, 0, 0, 0},
		{"90 about z", quat(h, 0, 0, h), vec(1, 0, 0), 0, 1, 0, 1e-9},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := c.q.Rotate(c.v)
			x, y, z := r.X.Eval(nil, nil), r.Y.Eval(nil, nil), r.Z.Eval(nil, nil)
			if math.Abs(x-c.x) > c.tol || math.Abs(y-c.y) > c.tol || math.Abs(z-c.z) > c.tol {
				t.Errorf("got (%v,%v,%v), want (%v,%v,%v)", x, y, z, c.x, c.y, c.z)
			}
		})
	}
}

// TestExprConstantArithmetic checks the operators on constants, which need
// no parameter provider.
func TestExprConstantArithmetic(t *testing.T) {
	t.Parallel()
	c := NewConstant
	cases := []struct {
		name string
		e    *Expr
		want float64
	}{
		{"(2+3)*4", c(2).Plus(c(3)).Times(c(4)), 20},
		{"3+4", c(3).Plus(c(4)), 7},
		{"10-3", c(10).Minus(c(3)), 7},
		{"7-3", c(7).Minus(c(3)), 4},
		{"6*7", c(6).Times(c(7)), 42},
		{"10/2", c(10).Div(c(2)), 5},
		{"20/4", c(20).Div(c(4)), 5},
		{"-(5)", c(5).Negate(), -5},
		{"-(-3)", c(-3).Negate(), 3},
		{"abs(-5)", c(-5).Abs(), 5},
		{"abs(-7)", c(-7).Abs(), 7},
		{"sqrt(16)", c(16).Sqrt(), 4},
		{"7^2", c(7).Square(), 49},
		{"5^2", c(5).Square(), 25},
		{"floor(3.7)", c(3.7).Floor(), 3},
		{"ceil(3.2)", c(3.2).Ceil(), 4},
		{"round(3.5)", c(3.5).Round(), 4},
	}
	for _, tc := range cases {
		if v := tc.e.Eval(nil, nil); v != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, v, tc.want)
		}
	}
}

// TestExprLnExp verifies natural log and exponential.
func TestExprLnExp(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(2).Pow(NewConstant(10)).Eval(ctx, nil); v != 1024 {
		t.Errorf("2^10 = %v, want 1024", v)
	}
}

// TestExprHypot verifies hypot function.
func TestExprHypot(t *testing.T) {
	t.Parallel()
	ctx := stubParamProvider{values: map[HParam]float64{}}
	if v := NewConstant(3).Hypot(NewConstant(4)).Eval(ctx, nil); v != 5 {
		t.Errorf("hypot(3,4) = %v, want 5", v)
	}
}

// TestExprArctrig verifies inverse trig functions.
func TestExprArctrig(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestExprSinCosSumToOne checks sin^2 + cos^2 = 1.
func TestExprSinCosSumToOne(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	ctx := stubParamProvider{values: map[HParam]float64{}}
	v := NewConstant(1).Div(NewConstant(0)).Eval(ctx, nil)
	if !isInf(v) {
		t.Errorf("1/0 = %v, expected inf", v)
	}
}

func isInf(f float64) bool {
	return f > 1e308 || f < -1e308
}
