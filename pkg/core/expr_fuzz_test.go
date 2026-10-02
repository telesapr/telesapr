package core

import "testing"

// varsOf collects the variables an expression names.
func varsOf(e *Expr, vars map[string]float64) {
	if e == nil {
		return
	}
	if e.Op == OpVariable {
		vars[e.VarID] = 1.5
	}
	varsOf(e.Left, vars)
	varsOf(e.Right, vars)
	for _, a := range e.Args {
		varsOf(a, vars)
	}
}

// FuzzExpr parses any text as an expression: it is an expression or an
// error, never a panic, and an expression evaluates once its variables
// have values.
func FuzzExpr(f *testing.F) {
	for _, s := range []string{
		"1", "-2.5e3", "1+2*3", "(1+2)*3", "2*pi*r", "sqrt(sq(3) + sq(4))",
		"sin(30) + cos(60) - tan(45)", "asin(1)/acos(0)", "wall*4 - bore/2",
		"min(a, 2, max(b))", "((", ")", "1 +", "*2", "sqrt()", "foo(1)", "1e", "--1", "2 3",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		e, err := NewFromString(s)
		if err != nil {
			if e != nil {
				t.Errorf("%q: an expression and the error %v", s, err)
			}
			return
		}
		if e == nil {
			t.Fatalf("%q: no expression and no error", s)
		}
		vars := map[string]float64{}
		varsOf(e, vars)
		e.Eval(nil, vars)
	})
}
