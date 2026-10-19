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

// MakeFromPiecewiseOf cuts a by b, or keeps what of a is in b with
// intersect, piece by piece, and assembles what comes of each: a
// Boolean of a shell of parts assembled side by side fails where the
// parts touch face to face, as a shaft standing on its gearbox cut
// across the plane they share, though each part's Boolean with b is
// sound. Cutting each part is cutting the whole, as the parts don't
// stand in each other. A piece clear of b is kept, or dropped with
// intersect, as it is. It reports whether a has more than one piece;
// when it hasn't, s is left alone.
func (s *SShell) MakeFromPiecewiseOf(a, b *SShell, intersect bool) bool {
	pieces := a.Pieces()
	if len(pieces) < 2 {
		return false
	}
	bMax, bMin := s.boundsOf(b)
	var out SShell
	out.Tol = a.Tol.Or(b.Tol)
	failed := false
	for _, p := range pieces {
		pMax, pMin := s.boundsOf(p)
		apart := pMin.X > bMax.X || pMin.Y > bMax.Y || pMin.Z > bMax.Z ||
			bMin.X > pMax.X || bMin.Y > pMax.Y || bMin.Z > pMax.Z
		var r SShell
		switch {
		case apart && intersect:
			continue
		case apart:
			r.MakeFromCopyOf(p)
		case intersect:
			r.MakeFromIntersectionOf(p, b)
		default:
			r.MakeFromDifferenceOf(p, b)
		}
		failed = failed || r.BooleanFailed
		var next SShell
		next.MakeFromAssemblyOf(&out, &r)
		out = next
	}
	*s = out
	s.BooleanFailed = failed
	return true
}

// boundsOf is the box about the shell's control points, which holds it.
func (*SShell) boundsOf(sh *SShell) (ptMax, ptMin geom.Vector) {
	ptMax = geom.Vector{X: veryNegative, Y: veryNegative, Z: veryNegative}
	ptMin = geom.Vector{X: veryPositive, Y: veryPositive, Z: veryPositive}
	for i := range sh.Surface {
		var hi, lo geom.Vector
		sh.Surface[i].AxisAlignedBounding(&hi, &lo)
		hi.MakeMaxMin(&ptMax, &ptMin)
		lo.MakeMaxMin(&ptMax, &ptMin)
	}
	return ptMax, ptMin
}
