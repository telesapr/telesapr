package solver

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// The equations of a redundant group depend on each other: some
// combination y of the Jacobian's rows, y'J = 0, is zero. Each such y is
// a vector of the left null space of J, the null space of J', and the
// equations y holds a non-zero weight of over-determine each other: drop
// any one of them and that dependency is gone. So the groups to name are
// the supports of a basis of the left null space, all found in one
// reduction, where removing one constraint at a time and testing the
// rank again finds one dependency and only those of its constraints the
// search order happens to try.
//
// A basis is not unique, and an orthonormal one mixes every dependency
// into every vector. The basis taken is the reduced one (each vector
// has an equation of its own no other vector holds, the fundamental
// circuits of the rows the reduction keeps), made sparser by taking
// one vector from another wherever that shortens it. The groups are
// these supports, not the connected components of their union: two
// dependencies that share an equation stay two groups, as five
// equations in three unknowns with a fifth twice the fourth have the
// group {E4, E5} and one of four of the others (Zou and Feng 2019),
// where the union would be all five.

// zeroWeight is how small a weight of a null vector, against its
// largest, is taken as zero: the reduction leaves rounding where an
// exact one gives zero.
const zeroWeight = 1e-9

// dependentRows lists the dependency groups of the rows of a, m rows of
// n columns: each group the rows, in order, of one vector of a sparse
// basis of the left null space; the groups smallest first. None when
// the rows are independent. A row of zeros is a group of its own. The
// rows are reduced a connected component at a time (rows sharing no
// column with each other's fall apart), which keeps a long chain's
// dense reduction to the rows that depend.
func dependentRows(a [][]float64, m, n int) [][]int {
	var groups [][]int
	for _, comp := range rowComponents(a, m, n) {
		if len(comp.cols) == 0 {
			// A row that reads no unknown is a dependency by itself.
			groups = append(groups, comp.rows)
			continue
		}
		if len(comp.rows) == 1 {
			// One row that reads an unknown depends on nothing.
			continue
		}
		for _, v := range leftNullBasis(a, comp) {
			var g []int
			for k, w := range v {
				if w != 0 {
					g = append(g, comp.rows[k])
				}
			}
			groups = append(groups, g)
		}
	}
	slices.SortStableFunc(groups, func(x, y []int) int {
		if c := cmp.Compare(len(x), len(y)); c != 0 {
			return c
		}
		return slices.Compare(x, y)
	})
	return groups
}

// rowComponents splits the rows of a into the sets no non-zero entry
// joins, each with the columns its rows read; a row of zeros is a
// component of its own with no columns.
func rowComponents(a [][]float64, m, n int) []component {
	parent := make([]int, n)
	for j := range n {
		parent[j] = j
	}
	find := func(j int) int {
		for parent[j] != j {
			parent[j] = parent[parent[j]]
			j = parent[j]
		}
		return j
	}
	first := make([]int, m)
	for i := range m {
		first[i] = -1
		for j, v := range a[i] {
			if v == 0 {
				continue
			}
			if first[i] < 0 {
				first[i] = j
				continue
			}
			if x, y := find(first[i]), find(j); x != y {
				parent[max(x, y)] = min(x, y)
			}
		}
	}
	index := map[int]int{}
	var comps []component
	for i := range m {
		if first[i] < 0 {
			comps = append(comps, component{rows: []int{i}})
			continue
		}
		r := find(first[i])
		k, ok := index[r]
		if !ok {
			k = len(comps)
			index[r] = k
			comps = append(comps, component{})
		}
		comps[k].rows = append(comps[k].rows, i)
	}
	for j := range n {
		if k, ok := index[find(j)]; ok {
			comps[k].cols = append(comps[k].cols, j)
		}
	}
	return comps
}

// leftNullBasis is a sparse basis of the left null space of the rows
// and columns of a that c holds: each vector a weight for each of
// c.rows, the weights below zeroWeight of the largest made zero.
func leftNullBasis(a [][]float64, c component) [][]float64 {
	m, n := len(c.rows), len(c.cols)
	// J' of the component: a row per unknown, a column per equation.
	at := make([][]float64, n)
	for j, col := range c.cols {
		at[j] = make([]float64, m)
		for i, row := range c.rows {
			at[j][i] = a[row][col]
		}
	}
	pivotOf := rref(at, n, m)
	// A free column f of J', an equation the others before it combine
	// to, gives the vector e_f minus, at each pivot column, what the
	// pivot's row holds in column f.
	var basis [][]float64
	for f := range m {
		if pivotOf[f] >= 0 {
			continue
		}
		v := make([]float64, m)
		v[f] = 1
		for j := range m {
			if r := pivotOf[j]; r >= 0 {
				v[j] = -at[r][f]
			}
		}
		basis = append(basis, clean(v))
	}
	return sparsify(basis)
}

// clean makes zero the weights of v below zeroWeight of its largest,
// and returns v.
func clean(v []float64) []float64 {
	top := 0.0
	for _, w := range v {
		top = math.Max(top, math.Abs(w))
	}
	for k, w := range v {
		if math.Abs(w) <= zeroWeight*top {
			v[k] = 0
		}
	}
	return v
}

func support(v []float64) int {
	n := 0
	for _, w := range v {
		if w != 0 {
			n++
		}
	}
	return n
}

// sparsify shortens the vectors of basis while one less a multiple of
// another, cancelling a weight they share, has fewer non-zero weights:
// the basis stays a basis, since only a multiple of another vector is
// taken from each. It is a local search, not the sparsest basis (that
// is NP-hard), but it is what separates a dependency of two equations
// from the longer one the reduction found it within.
func sparsify(basis [][]float64) [][]float64 {
	for changed := true; changed; {
		changed = false
		for x, v := range basis {
			for y, u := range basis {
				if x == y {
					continue
				}
				if w, ok := shorter(v, u); ok {
					basis[x], v = w, w
					changed = true
				}
			}
		}
	}
	return basis
}

// shorter is the shortest of v less a multiple of u that cancels a
// weight they share, if it is shorter than v.
func shorter(v, u []float64) ([]float64, bool) {
	var best []float64
	bestLen := support(v)
	for k := range v {
		if v[k] == 0 || u[k] == 0 {
			continue
		}
		f := v[k] / u[k]
		w := make([]float64, len(v))
		for i := range v {
			w[i] = v[i] - f*u[i]
		}
		w[k] = 0
		clean(w)
		if l := support(w); l > 0 && l < bestLen {
			best, bestLen = w, l
		}
	}
	return best, best != nil
}

// DependentConstraints names the constraints of g that over-determine
// each other, a group for each dependency of the equations, found from
// the left null space of the Jacobian at the unknowns' current values:
// the configuration the solve reached, which satisfies the constraints,
// so that the rank there is the one to trust. Each group lists its
// constraints in handle order, the point-coincident ones last; a
// dependency only of equations an entity or the group writes names
// nothing and is left out, and a group equal to another is listed once.
//
// The equations are written whole, without the substitution of a = b
// or the solving alone of an equation of one unknown that the solve
// makes: a constraint those took out is in the dependency it makes as
// much as the one the reduced system shows, as a horizontal line, made
// so by substitution, with a second horizontal line and a parallel
// between them. The caller sets each substituted unknown to the value
// of the one it was substituted by first (Solve does).
func (s *System) DependentConstraints(g *Group) [][]core.HConstraint {
	s.writeEquationsExceptFor(core.NoConstraint, g)
	s.clearParamTags()
	s.clearEqTags()
	if !s.WriteJacobian(0) {
		return nil
	}
	s.EvalJacobian()
	m, n := s.mat.m, s.mat.n
	a := make([][]float64, m)
	for i := range a {
		a[i] = append([]float64(nil), s.mat.Anum[i]...)
	}
	var out [][]core.HConstraint
	for _, rows := range dependentRows(a, m, n) {
		in := map[core.HConstraint]bool{}
		for _, r := range rows {
			if h := s.mat.eq[r].H; h.IsFromConstraint() {
				in[h.Constraint()] = true
			}
		}
		var hs []core.HConstraint
		for pass := range 2 {
			for _, c := range g.Constraints {
				if in[c.H] && (c.Type == core.ConstraintPointsCoincident) == (pass == 1) {
					hs = append(hs, c.H)
				}
			}
		}
		if len(hs) == 0 || slices.ContainsFunc(out, func(o []core.HConstraint) bool { return slices.Equal(o, hs) }) {
			continue
		}
		out = append(out, hs)
	}
	return out
}

// unionOf is the constraints of the groups, each once, in handle order,
// the point-coincident ones last, as g lists them.
func unionOf(g *Group, groups [][]core.HConstraint) []core.HConstraint {
	in := map[core.HConstraint]bool{}
	for _, hs := range groups {
		for _, h := range hs {
			in[h] = true
		}
	}
	var out []core.HConstraint
	for pass := range 2 {
		for _, c := range g.Constraints {
			if in[c.H] && (c.Type == core.ConstraintPointsCoincident) == (pass == 1) {
				out = append(out, c.H)
			}
		}
	}
	return out
}

// findDependent is the constraints to blame for a redundant group, with
// g.Dependent set to its dependencies: the union of the groups
// DependentConstraints finds, at the values the solve reached, a
// substituted unknown taking the value of the one it was substituted by
// (subMap). Only when that finds none, as where the rank test and the
// reduction disagree at the edge of their thresholds, does it fall back
// to removing each constraint in turn and testing the rank again
// (FindWhichToRemoveToFixJacobian), which g.FindToFixTimeout bounds.
func (s *System) findDependent(g *Group, subMap map[core.HParam]*Param, forceDofCheck bool) []core.HConstraint {
	for h, by := range subMap {
		if p := s.findParam(h); p != nil {
			p.Val = by.Val
		}
	}
	g.Timeout = false
	if groups := s.DependentConstraints(g); len(groups) > 0 {
		g.Dependent = groups
		return unionOf(g, groups)
	}
	bad := s.FindWhichToRemoveToFixJacobian(g, forceDofCheck)
	if len(bad) > 0 {
		g.Dependent = [][]core.HConstraint{bad}
	}
	return bad
}
