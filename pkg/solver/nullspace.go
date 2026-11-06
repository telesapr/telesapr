package solver

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
)

// NullSpace is a basis of the directions in which the unknowns of the
// group can move, to first order at their current values, and leave
// every equation as it is: the null space of the Jacobian of all the
// group's equations over all its unknowns. Each vector has a component
// for each of params, the unknowns in handle order, and the vectors are
// orthonormal. None when the equations fix every unknown, and nil when
// the system is too large to write. Nothing is solved or changed; the
// sketch should be solved first.
//
// The Jacobian is written without the substitution the solver makes of
// an equation a = b, so that each vector moves every unknown, b with a:
// a caller adds a step along the vectors to the unknowns' values.
func (s *System) NullSpace(g *Group) (params []core.HParam, null [][]float64) {
	s.writeEquationsExceptFor(core.NoConstraint, g)
	s.clearParamTags()
	s.clearEqTags()
	if !s.WriteJacobian(0) {
		return nil, nil
	}
	s.EvalJacobian()
	m, n := s.mat.m, s.mat.n
	params = append([]core.HParam(nil), s.mat.param...)
	a := make([][]float64, m)
	for i := range a {
		a[i] = append([]float64(nil), s.mat.Anum[i]...)
	}
	return params, nullVectors(a, m, n)
}

// nullVectors is an orthonormal basis of the null space of a, m rows
// of n, which it reduces in place.
func nullVectors(a [][]float64, m, n int) [][]float64 {
	pivotOf := rref(a, m, n)
	// A free column f gives the vector e_f minus, in each pivot column,
	// what the pivot's row holds in column f.
	var null [][]float64
	for f := range n {
		if pivotOf[f] >= 0 {
			continue
		}
		v := make([]float64, n)
		v[f] = 1
		for j := range n {
			if r := pivotOf[j]; r >= 0 {
				v[j] = -a[r][f]
			}
		}
		null = append(null, v)
	}
	return orthonormal(null)
}

// orthonormal makes the vectors vs orthonormal in place, by modified
// Gram-Schmidt, dropping any that the others span.
func orthonormal(vs [][]float64) [][]float64 {
	out := vs[:0]
	for _, v := range vs {
		for _, u := range out {
			d := dot(u, v)
			for k := range v {
				v[k] -= d * u[k]
			}
		}
		norm := math.Sqrt(dot(v, v))
		if norm < 1e-12 {
			continue
		}
		for k := range v {
			v[k] /= norm
		}
		out = append(out, v)
	}
	return out
}

func dot(a, b []float64) float64 {
	var s float64
	for k := range a {
		s += a[k] * b[k]
	}
	return s
}
