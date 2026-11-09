package solver

import (
	"math"
	"math/rand"
	"slices"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
)

// Zou and Feng's five equations in three unknowns (arXiv 1904.00526,
// section 3.3): E5 is twice E4, and any four of E1 to E4 depend. A
// greedy search seeded at E1 reports {E1, E2, E3, E4} and
// {E1, E2, E3, E5}; the sparse basis has the minimal group {E4, E5},
// and for the second dependency one of three, E5 = 8 E1 - 2 E3.
var zouFeng = [][]float64{
	{1, 1, 1},
	{2, 1, 1},
	{3, 2, 1},
	{1, 2, 3},
	{2, 4, 6},
}

func TestDependentRowsZouFeng(t *testing.T) {
	t.Parallel()
	got := dependentRows(zouFeng, 5, 3)
	want := [][]int{{3, 4}, {0, 2, 4}}
	if !slices.EqualFunc(got, want, slices.Equal) {
		t.Errorf("groups %v, want %v", got, want)
	}
}

// The basis dependentRows takes is a basis of the left null space: as
// many vectors as rows past the rank, each taking the rows to zero.
func TestLeftNullBasisIsBasis(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(3))
	for trial := range 3000 {
		n := 1 + rng.Intn(8)
		m := 1 + rng.Intn(n+4)
		a := make([][]float64, m)
		for i := range a {
			a[i] = make([]float64, n)
			switch k := rng.Intn(4); {
			case k == 0 && i >= 1:
				src := a[rng.Intn(i)]
				for j := range a[i] {
					a[i][j] = 3 * src[j]
				}
			case k == 1 && i >= 2:
				for j := range a[i] {
					a[i][j] = 2*a[i-1][j] - a[i-2][j]
				}
			default:
				for j := range a[i] {
					if rng.Intn(3) == 0 {
						a[i][j] = float64(rng.Intn(7) - 3)
					}
				}
			}
		}
		total := 0
		for _, comp := range rowComponents(a, m, n) {
			if len(comp.cols) == 0 {
				total++
				continue
			}
			sub := make([][]float64, len(comp.rows))
			for k, r := range comp.rows {
				for _, col := range comp.cols {
					sub[k] = append(sub[k], a[r][col])
				}
			}
			basis := leftNullBasis(a, comp)
			if want := len(comp.rows) - rankOf(sub, len(comp.rows), len(comp.cols)); len(basis) != want {
				t.Fatalf("trial %d: %d vectors for %v, want %d", trial, len(basis), sub, want)
			}
			for _, v := range basis {
				for j := range comp.cols {
					s := 0.0
					for k := range comp.rows {
						s += v[k] * sub[k][j]
					}
					if math.Abs(s) > 1e-9 {
						t.Fatalf("trial %d: %v does not take %v to zero", trial, v, sub)
					}
				}
			}
			total += len(basis)
		}
		if groups := dependentRows(a, m, n); len(groups) != total {
			t.Fatalf("trial %d: %d groups, want %d", trial, len(groups), total)
		}
	}
}

func hcs(vs ...uint32) []core.HConstraint {
	var out []core.HConstraint
	for _, v := range vs {
		out = append(out, core.HConstraint{V: v})
	}
	return out
}

// A solve that converges on Zou and Feng's equations, made consistent,
// names both dependencies, the two equations first.
func TestSolveNamesZouFengGroups(t *testing.T) {
	t.Parallel()
	sk := testSketch{}
	x, y, z := sk.param(1, 0), sk.param(2, 0), sk.param(3, 0)
	row := func(r []float64, rhs float64) []*core.Expr {
		return []*core.Expr{p(x).Times(c(r[0])).Plus(p(y).Times(c(r[1]))).Plus(p(z).Times(c(r[2]))).Minus(c(rhs))}
	}
	// E1 to E3 give x = 1, y = -1, z = 0; E4 and E5 agree with them.
	g := testGroup([][]*core.Expr{
		row(zouFeng[0], 0), row(zouFeng[1], 1), row(zouFeng[2], 1),
		row(zouFeng[3], -1), row(zouFeng[4], -2),
	})
	s := newSystem(sk, x, y, z)
	how, _, bad := s.Solve(g, true, false, true)
	if how != core.SolveRedundantOkay {
		t.Fatalf("how %d, want REDUNDANT_OKAY", how)
	}
	want := [][]core.HConstraint{hcs(4, 5), hcs(1, 3, 5)}
	if !slices.EqualFunc(g.Dependent, want, slices.Equal) {
		t.Errorf("groups %v, want %v", g.Dependent, want)
	}
	if !slices.Equal(bad, hcs(1, 3, 4, 5)) {
		t.Errorf("bad %v, want 1, 3, 4 and 5", bad)
	}
}

// Two dependencies apart: removing any one constraint leaves the other,
// so a search that removes one at a time finds nothing to name.
func TestSolveNamesTwoGroups(t *testing.T) {
	t.Parallel()
	sk := testSketch{}
	x, y, u, w := sk.param(1, 0), sk.param(2, 0), sk.param(3, 0), sk.param(4, 0)
	g := testGroup([][]*core.Expr{
		{p(x).Plus(p(y)).Minus(c(3))},
		{p(x).Minus(p(y)).Minus(c(1))},
		{p(x).Plus(p(y)).Times(c(2)).Minus(c(6))},
		{p(u).Times(p(w)).Minus(c(2))},
		{p(u).Minus(p(w)).Minus(c(1))},
		{p(u).Times(p(w)).Times(c(3)).Minus(c(6))},
	})
	sk[u].Val, sk[w].Val = 3, 1
	s := newSystem(sk, x, y, u, w)
	if got := s.FindWhichToRemoveToFixJacobian(g, true); len(got) != 0 {
		t.Fatalf("removing one at a time names %v", got)
	}
	how, _, bad := s.Solve(g, true, false, true)
	if how != core.SolveRedundantOkay {
		t.Fatalf("how %d, want REDUNDANT_OKAY", how)
	}
	want := [][]core.HConstraint{hcs(1, 3), hcs(4, 6)}
	if !slices.EqualFunc(g.Dependent, want, slices.Equal) {
		t.Errorf("groups %v, want %v", g.Dependent, want)
	}
	if !slices.Equal(bad, hcs(1, 3, 4, 6)) {
		t.Errorf("bad %v", bad)
	}
}

// An equation a = b the solve substitutes away is still blamed: after
// it the other two equations are the same, b + k = 5, and only they
// show in the system solved, but the dependency is of all three.
func TestSolveBlamesSubstituted(t *testing.T) {
	t.Parallel()
	sk := testSketch{}
	a, b, k := sk.param(1, 0), sk.param(2, 0), sk.param(3, 0)
	sk[b].Val = 2
	g := testGroup([][]*core.Expr{
		{p(a).Minus(p(b))},
		{p(a).Plus(p(k)).Minus(c(5))},
		{p(b).Plus(p(k)).Minus(c(5))},
	})
	s := newSystem(sk, a, b, k)
	// forceDofCheck false: the solve substitutes a = b.
	how, _, bad := s.Solve(g, true, false, false)
	if how != core.SolveRedundantOkay {
		t.Fatalf("how %d, want REDUNDANT_OKAY", how)
	}
	if !slices.EqualFunc(g.Dependent, [][]core.HConstraint{hcs(1, 2, 3)}, slices.Equal) || !slices.Equal(bad, hcs(1, 2, 3)) {
		t.Errorf("groups %v, bad %v, want constraints 1, 2 and 3", g.Dependent, bad)
	}
	if math.Abs(sk[a].Val-sk[b].Val) > 1e-9 || math.Abs(sk[k].Val+sk[a].Val-5) > 1e-9 {
		t.Errorf("a %g b %g k %g", sk[a].Val, sk[b].Val, sk[k].Val)
	}
}

// The rank test without solving names the groups the same way.
func TestSolveRankNamesGroups(t *testing.T) {
	t.Parallel()
	sk := testSketch{}
	x, y := sk.param(1, 0), sk.param(2, 0)
	g := testGroup([][]*core.Expr{
		{p(x).Plus(p(y)).Minus(c(3))},
		{p(y).Minus(c(1))},
		{p(x).Plus(p(y)).Minus(c(3)).Times(c(2))},
	})
	s := newSystem(sk, x, y)
	how, _, _, bad := s.SolveRank(g, true, false)
	if how != core.SolveRedundantOkay || !slices.Equal(bad, hcs(1, 3)) {
		t.Errorf("how %d bad %v, want REDUNDANT_OKAY with 1 and 3", how, bad)
	}
}
