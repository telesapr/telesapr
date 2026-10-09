package slvs_test

import (
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/golden"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/solver"
)

// fixtureGroupsPath records, for every fixture that loads, its number of
// groups, entities and constraints, and of each group how it solved, its
// degrees of freedom, whether it regenerated clean and the sorted face
// entities of its own shell's surfaces.
var fixtureGroupsPath = filepath.Join("..", "..", "..", "testdata", "fixture-groups.txt")

// TestFixtureGroupsSolve regenerates every fixture and compares what
// fixtureGroupsPath records; of every group that solved, it also requires
// each constraint's equations to hold afterwards. TELESAPR_REGEN_GOLDEN=1
// (scripts/regen-golden.sh groups) rewrites the record; review its diff.
func TestFixtureGroupsSolve(t *testing.T) {
	var dump strings.Builder
	for _, name := range invariantFixtures(t) {
		if name == invalidFixture {
			continue
		}
		doc, err := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s: load: %v", name, err)
			continue
		}
		sk := doc.Sketch
		sk.GenerateAll()
		dumpFixtureGroups(&dump, name, sk)
		requireEquationsHold(t, name, sk)
	}
	golden.CheckDump(t, fixtureGroupsPath, dump.String())
}

// dumpFixtureGroups writes a fixture's line and one line per group.
func dumpFixtureGroups(b *strings.Builder, name string, sk *sketch.Sketch) {
	fmt.Fprintf(b, "%s groups=%d entities=%d constraints=%d\n",
		name, len(sk.Groups), len(sk.Entities), len(sk.Constraints))
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		fmt.Fprintf(b, "%s g%08x how=%d dof=%d clean=%v", name, hg.V, g.Solved.How, g.Solved.Dof, g.Clean)
		var faces []uint32
		for _, ss := range g.ThisShell.Surface {
			faces = append(faces, uint32(ss.Face))
		}
		slices.Sort(faces)
		if len(faces) > 0 {
			b.WriteString(" faces")
			for _, f := range faces {
				fmt.Fprintf(b, " %08x", f)
			}
		}
		b.WriteString("\n")
	}
}

// requireEquationsHold fails t for every equation of a constraint in a
// group that solved whose residual is above the solver's tolerance.
func requireEquationsHold(t *testing.T, name string, sk *sketch.Sketch) {
	t.Helper()
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if g := sk.Group(c.Group); g == nil || g.Solved.How != core.SolveOkay {
			continue
		}
		var eqs []core.Equation
		c.GenerateEquations(sk, &eqs, true)
		for _, eq := range eqs {
			if r := core.EvalExpr(sk, eq.Expr); math.Abs(r) > solver.ConvergeTolerance {
				t.Errorf("%s: constraint %08x (type %d) equation %08x: residual %g",
					name, c.H.V, c.Type, eq.H.V, r)
			}
		}
	}
}
