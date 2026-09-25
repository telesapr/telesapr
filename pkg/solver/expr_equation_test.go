package solver

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
)

// TestAddExprEquationLinkedMagnitude verifies that AddExprEquation
// adds a quaternion-magnitude=1 equation to the system.
func TestAddExprEquationLinkedMagnitude(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{3}: 1, {4}: 0, {5}: 0, {6}: 0,
	}}
	s := NewSystem(res1)
	// LINKED: |q|² - 1 = 0 where q is on params 3..6.
	qw := core.NewParam(core.HParam{V: 3})
	qvx := core.NewParam(core.HParam{V: 4})
	qvy := core.NewParam(core.HParam{V: 5})
	qvz := core.NewParam(core.HParam{V: 6})
	magSq := qw.Times(qw).Plus(qvx.Times(qvx)).Plus(qvy.Times(qvy)).Plus(qvz.Times(qvz))
	if err := s.AddExprEquation(magSq, core.NewConstant(1)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	if len(s.Equations) == 0 {
		t.Fatalf("expected AddExprEquation to register an equation")
	}
	// Evaluate at q = (1, 0, 0, 0) (identity quaternion). Result should be 0.
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("magnitude equation evaluated to %f, want 0", res)
	}
	// Evaluate at q = (2, 0, 0, 0). Result should be 3 (|q|² - 1 = 3).
	res2 := &stubResolver{params: map[core.HParam]float64{
		{3}: 2, {4}: 0, {5}: 0, {6}: 0,
	}}
	res = s.Equations[0].Expr.Eval(res2, nil)
	if res != 3 {
		t.Fatalf("magnitude equation evaluated to %f, want 3", res)
	}
}

// stubResolver is a minimal solver Resolver for tests.
type stubResolver struct {
	params map[core.HParam]float64
}

func (r *stubResolver) Entity(h core.HEntity) (*core.Entity, error) {
	return nil, nil
}
func (r *stubResolver) Param(h core.HParam) (*core.Param, error) {
	if r.params != nil {
		if v, ok := r.params[h]; ok {
			return &core.Param{H: h, Val: v}, nil
		}
	}
	return nil, nil
}

// ParamValue satisfies core.EvalParamProvider.
func (r *stubResolver) ParamValue(h core.HParam) (float64, bool) {
	if r.params != nil {
		v, ok := r.params[h]
		return v, ok
	}
	return 0, false
}


// TestAddExprEquationZeroEqualsConstant verifies the trivial constant equation.
func TestAddExprEquationZeroEqualsConstant(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{}}
	s := NewSystem(res1)
	// 5 - 5 = 0 (always true).
	lhs := core.NewConstant(5)
	rhs := core.NewConstant(5)
	if err := s.AddExprEquation(lhs, rhs); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationParamDifference verifies a simple param-vs-param equation.
func TestAddExprEquationParamDifference(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 5, {2}: 3,
	}}
	s := NewSystem(res1)
	// param1 - param2 - 2 = 0 (5 - 3 - 2 = 0).
	pa := core.NewParam(core.HParam{V: 1})
	pb := core.NewParam(core.HParam{V: 2})
	expr := pa.Minus(pb).Minus(core.NewConstant(2))
	if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationScaledDifference verifies a constant-scaled param eq.
func TestAddExprEquationScaledDifference(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 10,
	}}
	s := NewSystem(res1)
	// param1 * 2 - 20 = 0.
	pa := core.NewParam(core.HParam{V: 1})
	two := core.NewConstant(2)
	twenty := core.NewConstant(20)
	expr := pa.Times(two).Minus(twenty)
	if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationSumEqualsConstant verifies a sum equation.
func TestAddExprEquationSumEqualsConstant(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 3, {2}: 7,
	}}
	s := NewSystem(res1)
	// param1 + param2 - 10 = 0 (3 + 7 - 10 = 0).
	pa := core.NewParam(core.HParam{V: 1})
	pb := core.NewParam(core.HParam{V: 2})
	expr := pa.Plus(pb).Minus(core.NewConstant(10))
	if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationDivision verifies a division equation.
func TestAddExprEquationDivision(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 20,
	}}
	s := NewSystem(res1)
	// param1 / 4 - 5 = 0 (20 / 4 - 5 = 0).
	pa := core.NewParam(core.HParam{V: 1})
	four := core.NewConstant(4)
	five := core.NewConstant(5)
	expr := pa.Div(four).Minus(five)
	if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationNegation verifies a negated param equation.
func TestAddExprEquationNegation(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 7,
	}}
	s := NewSystem(res1)
	// -param1 + 7 = 0.
	pa := core.NewParam(core.HParam{V: 1})
	expr := pa.Negate().Plus(core.NewConstant(7))
	if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationSqrt verifies a square-root equation.
func TestAddExprEquationSqrt(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 16,
	}}
	s := NewSystem(res1)
	// sqrt(param1) - 4 = 0.
	pa := core.NewParam(core.HParam{V: 1})
	expr := pa.Sqrt().Minus(core.NewConstant(4))
	if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationSin verifies a sin equation.
func TestAddExprEquationSin(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 0,
	}}
	s := NewSystem(res1)
	// sin(param1) = 0.
	pa := core.NewParam(core.HParam{V: 1})
	expr := pa.Sin()
	if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if res != 0 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationCos verifies a cos equation.
func TestAddExprEquationCos(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 0,
	}}
	s := NewSystem(res1)
	// cos(param1) - 1 = 0.
	pa := core.NewParam(core.HParam{V: 1})
	expr := pa.Cos().Minus(core.NewConstant(1))
	if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
		t.Fatalf("AddExprEquation: %v", err)
	}
	res := s.Equations[0].Expr.Eval(res1, nil)
	if math.Abs(res) > 1e-9 {
		t.Fatalf("evaluated to %f, want 0", res)
	}
}

// TestAddExprEquationMultipleEquations verifies multiple equations can be added.
func TestAddExprEquationMultipleEquations(t *testing.T) {
	res1 := &stubResolver{params: map[core.HParam]float64{
		{1}: 2, {2}: 4, {3}: 8,
	}}
	s := NewSystem(res1)
	// 3 equations: param1 = 2, param2 = 4, param3 = 8.
	for i, v := range []float64{2, 4, 8} {
		pa := core.NewParam(core.HParam{V: uint32(i + 1)})
		expr := pa.Minus(core.NewConstant(v))
		if err := s.AddExprEquation(expr, core.NewConstant(0)); err != nil {
			t.Fatalf("AddExprEquation %d: %v", i, err)
		}
	}
	if len(s.Equations) != 3 {
		t.Fatalf("expected 3 equations, got %d", len(s.Equations))
	}
}
