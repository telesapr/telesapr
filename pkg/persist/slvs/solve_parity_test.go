package slvs

import (
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/solver"
)

// solveParityExclusions are the fixtures TestParityGroupSolveVsCpp
// skips, with the reason.
var solveParityExclusions = map[string]string{
	"request/image/linked.slvs": "LINKED import isn't ported (PORTING.md)",
}

// TestParityGroupSolveVsCpp regenerates every fixture C++ can load and
// checks that each group solves, with the degrees of freedom C++
// reports, and that every constraint's equations hold afterwards.
func TestParityGroupSolveVsCpp(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "../../../testdata")
	for name, dofs := range cppGroupDOF {
		t.Run(name, func(t *testing.T) {
			if reason, ok := solveParityExclusions[name]; ok {
				t.Skip(reason)
			}
			doc, err := LoadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			sk := doc.Sketch
			sk.GenerateAll()

			for hv, dof := range dofs {
				g := sk.GetGroup(core.HGroup{V: hv})
				if g == nil {
					t.Errorf("group %08x missing", hv)
					continue
				}
				if g.Solved.How != core.SolveOkay {
					t.Errorf("group %08x: solve result %d, want OKAY", hv, g.Solved.How)
				}
				if g.Solved.Dof != dof {
					t.Errorf("group %08x: dof %d, C++ %d", hv, g.Solved.Dof, dof)
				}
			}

			for i := range sk.Constraints {
				c := &sk.Constraints[i]
				var eqs []core.Equation
				c.GenerateEquations(sk, &eqs, true)
				for _, eq := range eqs {
					if r := core.EvalExpr(sk, eq.Expr); math.Abs(r) > solver.ConvergeTolerance {
						t.Errorf("constraint %08x (type %d) equation %08x: residual %g",
							c.H.V, c.Type, eq.H.V, r)
					}
				}
			}
		})
	}
}
