package slvs

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/solver"
)

// TestParityGroupSolveVsCpp regenerates every fixture C++ can load and
// checks that each group solves, with the degrees of freedom C++
// reports, that its shell's faces are C++'s, and that every
// constraint's equations hold afterwards.
func TestParityGroupSolveVsCpp(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "../../../testdata")
	for name, dofs := range cppGroupDOF {
		t.Run(name, func(t *testing.T) {
			doc, err := LoadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			sk := doc.Sketch
			sk.GenerateAll()

			for hv, dof := range dofs {
				g := sk.Group(core.HGroup{V: hv})
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

			// Every group's own shell has the faces C++ gives it.
			for _, hg := range sk.GroupOrder {
				g := sk.Group(hg)
				var faces []uint32
				for _, ss := range g.ThisShell.Surface {
					faces = append(faces, uint32(ss.Face))
				}
				slices.Sort(faces)
				want := cppShellFaces[name][hg.V]
				if fmt.Sprint(faces) != fmt.Sprint(want) {
					t.Errorf("group %08x: shell faces %x, C++ %x", hg.V, faces, want)
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
