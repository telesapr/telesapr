package srf

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Degree elevation: a Bezier curve or patch written again with one
// more control point in a direction, the same curve or surface point
// for point. Patches of different degrees meet along a seam only once
// they have one degree there, and a surface of degree above 3 can be
// built from any lower one for testing.

// Elevated is the curve raised by one degree, or an error at
// MaxDegree.
func (b *SBezier) Elevated() (SBezier, error) {
	n := b.Degree
	if n >= MaxDegree {
		return SBezier{}, fmt.Errorf("srf: a curve of degree %d can't be raised past %d", n, MaxDegree)
	}
	var in [MaxDegree + 1]geom.Vector4
	for i := 0; i <= n; i++ {
		in[i] = geom.Vector4FromWeighted(b.Weight[i], b.Ctrl[i])
	}
	out := *b
	out.Degree = n + 1
	for i := 0; i <= n+1; i++ {
		a := float64(i) / float64(n+1)
		var q geom.Vector4
		if i > 0 {
			q = q.Plus(in[i-1].ScaledBy(a))
		}
		if i <= n {
			q = q.Plus(in[i].ScaledBy(1 - a))
		}
		out.Ctrl[i] = q.PerspectiveProject()
		out.Weight[i] = q.W
	}
	return out, nil
}

// ElevatedTo is the curve raised to degree deg, at least its own.
func (b *SBezier) ElevatedTo(deg int) (SBezier, error) {
	out := *b
	for out.Degree < deg {
		var err error
		if out, err = out.Elevated(); err != nil {
			return SBezier{}, err
		}
	}
	return out, nil
}

// Elevated is the patch raised by one degree in u (byU) or v. Its
// trims are kept: the surface is the same.
func (s *SSurface) Elevated(byU bool) (SSurface, error) {
	out := *s
	deg, lines := s.DegN, s.DegM
	if byU {
		deg, lines = s.DegM, s.DegN
	}
	if deg >= MaxDegree {
		return SSurface{}, fmt.Errorf("srf: a patch of degree %d can't be raised past %d", deg, MaxDegree)
	}
	if byU {
		out.DegM++
	} else {
		out.DegN++
	}
	for l := 0; l <= lines; l++ {
		var row SBezier
		row.Degree = deg
		for k := 0; k <= deg; k++ {
			i, j := l, k
			if byU {
				i, j = k, l
			}
			row.Ctrl[k], row.Weight[k] = s.Ctrl[i][j], s.Weight[i][j]
		}
		up, _ := row.Elevated()
		for k := 0; k <= deg+1; k++ {
			i, j := l, k
			if byU {
				i, j = k, l
			}
			out.Ctrl[i][j], out.Weight[i][j] = up.Ctrl[k], up.Weight[k]
		}
	}
	return out, nil
}

// ElevatedTo is the patch raised to degrees m and n, at least its own.
func (s *SSurface) ElevatedTo(m, n int) (SSurface, error) {
	out := *s
	for out.DegM < m || out.DegN < n {
		var err error
		if out, err = out.Elevated(out.DegM < m); err != nil {
			return SSurface{}, err
		}
	}
	return out, nil
}
