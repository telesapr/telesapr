package solver

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
)

func TestSubstitutionInternal(t *testing.T) {
	h1 := core.HParam{V: 0x1}
	h2 := core.HParam{V: 0x2}
	p1 := &core.Param{H: h1, Val: 0}
	p2 := &core.Param{H: h2, Val: 0}
	expr1 := core.NewParam(h1).Minus(core.NewParam(h2))
	expr2 := core.NewParam(h2).Minus(core.NewConstant(5))
	s := &System{
		Params:    map[core.HParam]*core.Param{h1: p1, h2: p2},
		Equations: []*Equation{{Expr: expr1}, {Expr: expr2}},
	}
	current := map[core.HParam]float64{h1: 1, h2: 2}
	remap, err := s.substituteEqualParameters(current)
	if err != nil {
		t.Fatalf("substitute: %v", err)
	}
	if len(remap) == 0 {
		t.Fatalf("expected substitution map")
	}
	opts := SolveOptions{}
	opts = opts.withDefaults()
	_, err = s.solveSingleParameterEquations(current, opts)
	if err != nil {
		t.Fatalf("stage solve: %v", err)
	}
	t.Logf("current after stage: %+v", current)
	if len(s.Equations) != 0 {
		ctx := evaluationContext{overrides: current, params: s.Params}
		val := s.Equations[0].Expr.Eval(ctx, nil)
		t.Fatalf("expected all equations solved, residual %f", val)
	}
	handles := s.ParamHandles()
	if len(handles) != 1 {
		t.Fatalf("expected single canonical handle, got %v", handles)
	}
	if v, ok := current[handles[0]]; !ok || math.Abs(v-5) > 1e-9 {
		t.Fatalf("expected value 5 got %v", current)
	}
}
