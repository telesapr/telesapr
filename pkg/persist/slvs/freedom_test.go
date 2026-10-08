package slvs_test

import (
	"path/filepath"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/persist/slvs"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// TestFreedomVsShowDof compares Sketch.GroupFreedom, which finds the
// free params of a group in one pass, with the SolveSpace algorithm behind
// Analyze > Show Degrees of Freedom (MarkParamsFree, a rank test per
// param), on every fixture's every solved group. What Show DOF finds
// free must be free here. The converse does not hold: Show DOF reports
// a param the solver substituted away, such as a coordinate of a point
// coincident with another, as fixed whatever its partner is, and here
// it is as free as its partner. pkg/solver's TestRrefFixedColumnsVsRank
// checks the analysis against the rank test's definition.
func TestFreedomVsShowDof(t *testing.T) {
	var groups, params, cppFreeOnly, oursFreeOnly int
	for _, fixture := range invariantFixtures(t) {
		doc, err := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(fixture)))
		if err != nil {
			continue
		}
		sk := doc.Sketch
		sk.GenerateAll()
		ours := map[core.HGroup]map[core.HParam]bool{}
		for _, hg := range sk.GroupOrder {
			if free := sk.GroupFreedom(hg); free != nil {
				ours[hg] = free
			}
		}
		sk.GenerateAllFindFree(sketch.GenerateAll)
		for hg, free := range ours {
			groups++
			for h, f := range free {
				params++
				p := sk.Param(h)
				if p == nil {
					continue
				}
				switch {
				case p.Free && !f:
					cppFreeOnly++
					t.Errorf("%s g%x: param %x is free for Show DOF, fixed here", fixture, hg.V, h.V)
				case f && !p.Free:
					oursFreeOnly++
				}
			}
		}
	}
	t.Logf("%d groups, %d params; free only for Show DOF %d, free only here %d", groups, params, cppFreeOnly, oursFreeOnly)
}

// largestGroup finds the fixture group that solves for the most params.
func largestGroup(b *testing.B) (string, core.HGroup) {
	best, bestFile, bestGroup := 0, "", core.HGroup{}
	for _, fixture := range invariantFixtures(&testing.T{}) {
		doc, err := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(fixture)))
		if err != nil {
			continue
		}
		sk := doc.Sketch
		sk.GenerateAll()
		for _, hg := range sk.GroupOrder {
			if n := len(sk.GroupFreedom(hg)); n > best {
				best, bestFile, bestGroup = n, fixture, hg
			}
		}
	}
	b.Logf("largest group: %s g%x, %d params", bestFile, bestGroup.V, best)
	return bestFile, bestGroup
}

// BenchmarkGroupFreedom is what colouring by constraint state adds to
// each regeneration, on the largest sketch group of the fixtures;
// BenchmarkRegenerateLargest is the regeneration itself.
func BenchmarkGroupFreedom(b *testing.B) {
	file, hg := largestGroup(b)
	doc, _ := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(file)))
	sk := doc.Sketch
	sk.GenerateAll()
	b.ResetTimer()
	for range b.N {
		sk.Generation++ // as a regeneration would, so the cache is missed
		sk.GroupFreedom(hg)
	}
}

func BenchmarkRegenerateLargest(b *testing.B) {
	file, _ := largestGroup(b)
	doc, _ := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(file)))
	sk := doc.Sketch
	sk.GenerateAll()
	b.ResetTimer()
	for range b.N {
		sk.GenerateAll()
	}
}
