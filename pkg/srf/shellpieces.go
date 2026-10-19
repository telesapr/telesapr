package srf

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Pieces are the shell's pieces: the sets of its surfaces that trim
// curves join, each as a shell of its own with the surfaces' and
// curves' handles kept. A shell assembled of solids side by side is a
// piece for each, which may stand in each other, where a Boolean of
// the whole is not defined; a Boolean of a piece is. A shell of one
// piece is itself.
func (s *SShell) Pieces() []*SShell {
	at := make(map[core.HSSurface]int, len(s.Surface))
	for i := range s.Surface {
		at[s.Surface[i].H] = i
	}
	root := make([]int, len(s.Surface))
	for i := range root {
		root[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if root[i] != i {
			root[i] = find(root[i])
		}
		return root[i]
	}
	for i := range s.Curve {
		a, okA := at[s.Curve[i].SurfA]
		b, okB := at[s.Curve[i].SurfB]
		if okA && okB {
			root[find(a)] = find(b)
		}
	}
	piece := make(map[int]int) // a root's piece
	var out []*SShell
	of := make([]int, len(s.Surface))
	for i := range s.Surface {
		r := find(i)
		k, ok := piece[r]
		if !ok {
			k = len(out)
			piece[r] = k
			out = append(out, &SShell{Tol: s.Tol})
		}
		of[i] = k
	}
	if len(out) <= 1 {
		return []*SShell{s}
	}
	for i := range s.Surface {
		ss := FromTransformationOf(&s.Surface[i], geom.Vector{}, geom.QuaternionIdentity, 1, true)
		ss.Tol = s.Tol
		out[of[i]].Surface = append(out[of[i]].Surface, ss)
	}
	for i := range s.Curve {
		if a, ok := at[s.Curve[i].SurfA]; ok {
			out[of[a]].Curve = append(out[of[a]].Curve,
				FromTransformationOfSCurve(&s.Curve[i], geom.Vector{}, geom.QuaternionIdentity, 1))
		}
	}
	return out
}
