package slvs

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/solver"
)

// cleanFixtures are the fixtures cppGroupDOF has no entry for whose
// groups must still all regenerate clean.
var cleanFixtures = []string{"group/link/normal.slvs"}

// TestFixtureGroupsSolve regenerates every fixture in cppGroupDOF and
// requires of each group that it is clean and solves, with the degrees
// of freedom recorded, that its own shell's faces are those recorded
// (cppShellFaces), and that every constraint's equations hold
// afterwards; of a constraint fixture, that it has the groups,
// entities and constraints recorded (constraintFixtureGoldens). The
// records were made while the kernel was verified against SolveSpace,
// and change only when a change to the loader or the solver means
// them to.
func TestFixtureGroupsSolve(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(thisFile), "../../../testdata")
	names := slices.Sorted(func(yield func(string) bool) {
		for name := range cppGroupDOF {
			if !yield(name) {
				return
			}
		}
	})
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			sk := loadAndRegenerate(t, filepath.Join(root, name))
			for hv, dof := range cppGroupDOF[name] {
				g := sk.Group(core.HGroup{V: hv})
				if g == nil {
					t.Errorf("group %08x missing", hv)
					continue
				}
				if g.Solved.How != core.SolveOkay {
					t.Errorf("group %08x: solve result %d, want OKAY", hv, g.Solved.How)
				}
				if g.Solved.Dof != dof {
					t.Errorf("group %08x: dof %d, want %d", hv, g.Solved.Dof, dof)
				}
			}
			requireClean(t, sk)

			for _, hg := range sk.GroupOrder {
				g := sk.Group(hg)
				var faces []uint32
				for _, ss := range g.ThisShell.Surface {
					faces = append(faces, uint32(ss.Face))
				}
				slices.Sort(faces)
				want := cppShellFaces[name][hg.V]
				if fmt.Sprint(faces) != fmt.Sprint(want) {
					t.Errorf("group %08x: shell faces %x, want %x", hg.V, faces, want)
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

			dir, file := filepath.Split(name)
			if file != "normal.slvs" || !strings.HasPrefix(dir, "constraint/") {
				return
			}
			want, ok := constraintFixtureGoldens[filepath.Base(dir)]
			if !ok {
				return
			}
			if got := len(sk.Groups); got != want.Groups {
				t.Errorf("groups: got %d, want %d", got, want.Groups)
			}
			if got := len(sk.Entities); got != want.Entities {
				t.Errorf("entities: got %d, want %d", got, want.Entities)
			}
			if got := len(sk.Constraints); got != want.Constraints {
				t.Errorf("constraints: got %d, want %d", got, want.Constraints)
			}
		})
	}
	for _, name := range cleanFixtures {
		t.Run(name, func(t *testing.T) {
			requireClean(t, loadAndRegenerate(t, filepath.Join(root, name)))
		})
	}
}

func loadAndRegenerate(t *testing.T, path string) *sketch.Sketch {
	t.Helper()
	doc, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	doc.Sketch.GenerateAll()
	return doc.Sketch
}

// requireClean fails t for every group that did not regenerate clean.
func requireClean(t *testing.T, sk *sketch.Sketch) {
	t.Helper()
	for _, hg := range sk.GroupOrder {
		if g := sk.Group(hg); !g.Clean {
			t.Errorf("group %08x: not clean", hg.V)
		}
	}
}
