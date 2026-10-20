package solver

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/libm"
)

// A tape is an expression flattened in evaluation order: each
// instruction reads its operands from the slots of earlier ones and
// writes its own. It is a prototype, in this test only, to put a
// number on what the recursive walk of Expr.Eval costs against a
// loop; the solver does not use it.
type tapeInstr struct {
	op   core.Operation
	a, b int32
	val  float64
	p    *core.Param
}

type tape struct {
	instrs []tapeInstr
	slots  []float64
}

// compileTape flattens e, bound to params as the Jacobian's copies
// are, with the same folding of constants already done.
func compileTape(e *core.Expr) *tape {
	t := &tape{}
	t.add(e)
	t.slots = make([]float64, len(t.instrs))
	return t
}

func (t *tape) add(e *core.Expr) int32 {
	in := tapeInstr{op: e.Op, a: -1, b: -1}
	switch e.Op {
	case core.OpConstant:
		in.val = e.Value
	case core.OpParamPtr:
		in.p = e.ParamPtr
	default:
		if e.Left != nil {
			in.a = t.add(e.Left)
		}
		if e.Right != nil {
			in.b = t.add(e.Right)
		}
	}
	t.instrs = append(t.instrs, in)
	return int32(len(t.instrs) - 1)
}

func (t *tape) eval() float64 {
	s := t.slots
	for i := range t.instrs {
		in := &t.instrs[i]
		var v float64
		switch in.op {
		case core.OpConstant:
			v = in.val
		case core.OpParamPtr:
			v = in.p.Val
		case core.OpPlus:
			v = s[in.a] + s[in.b]
		case core.OpMinus:
			v = s[in.a] - s[in.b]
		case core.OpTimes:
			v = s[in.a] * s[in.b]
		case core.OpDiv:
			v = s[in.a] / s[in.b]
		case core.OpNegate:
			v = -s[in.a]
		case core.OpSqrt:
			v = math.Sqrt(s[in.a])
		case core.OpSquare:
			v = s[in.a] * s[in.a]
		case core.OpSin:
			v = libm.Sin(s[in.a])
		case core.OpCos:
			v = libm.Cos(s[in.a])
		default:
			panic("tape: op not compiled")
		}
		s[i] = v
	}
	return s[len(s)-1]
}

// jacobianExprs is the folded, bound equations and partials of the
// closed chain's Jacobian, as the Newton iterations evaluate them.
func jacobianExprs(b *testing.B) []*core.Expr {
	b.Helper()
	s := evaluatedSystem(b, makeChain(100, true))
	var es []*core.Expr
	es = append(es, s.mat.B...)
	for i := range s.mat.m {
		for _, pd := range s.mat.A[i] {
			if pd != nil {
				es = append(es, pd)
			}
		}
	}
	return es
}

// TestTapeSameAsEval holds the tape to Expr.Eval on the Jacobian's
// expressions.
func TestTapeSameAsEval(t *testing.T) {
	t.Parallel()
	var b testing.B
	for _, e := range jacobianExprs(&b) {
		if got, want := compileTape(e).eval(), e.Eval(nil, nil); got != want {
			t.Fatalf("tape %g, Eval %g", got, want)
		}
	}
}

// BenchmarkEvalTree evaluates the Jacobian's expressions by the
// recursive walk; BenchmarkEvalTape by the tape compiled once.
func BenchmarkEvalTree(b *testing.B) {
	es := jacobianExprs(b)
	b.ResetTimer()
	for b.Loop() {
		for _, e := range es {
			e.Eval(nil, nil)
		}
	}
}

func BenchmarkEvalTape(b *testing.B) {
	es := jacobianExprs(b)
	tapes := make([]*tape, len(es))
	for i, e := range es {
		tapes[i] = compileTape(e)
	}
	b.ResetTimer()
	for b.Loop() {
		for _, t := range tapes {
			t.eval()
		}
	}
}
