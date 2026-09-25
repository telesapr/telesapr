package solver

import (
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

