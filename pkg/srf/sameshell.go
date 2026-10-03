package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// SameShell reports whether a and b bound the same solid patch for
// patch: each surface of b has a surface of a of its own with the same
// control points and weights, in any order, and trims that start and
// end at the same points. A part symmetric about a plane, reflected in
// it, gives such a shell; a Boolean of the two would put every face
// through as coincident, which it can't do for curved faces (roadmap
// 13.4).
func SameShell(a, b *SShell) bool {
	if len(a.Surface) != len(b.Surface) || len(a.Surface) == 0 {
		return false
	}
	used := make([]bool, len(a.Surface))
	for i := range b.Surface {
		found := false
		for j := range a.Surface {
			if !used[j] && samePatch(&a.Surface[j], &b.Surface[i]) {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// samePatch reports whether s and t have the same degrees, either way
// round, the same control points with their weights as a set, and trims
// with the same ends as a set.
func samePatch(s, t *SSurface) bool {
	if !(s.DegM == t.DegM && s.DegN == t.DegN) && !(s.DegM == t.DegN && s.DegN == t.DegM) {
		return false
	}
	type wp struct {
		p geom.Vector
		w float64
	}
	var ps, pt []wp
	for i := 0; i <= s.DegM; i++ {
		for j := 0; j <= s.DegN; j++ {
			ps = append(ps, wp{s.Ctrl[i][j], s.Weight[i][j]})
		}
	}
	for i := 0; i <= t.DegM; i++ {
		for j := 0; j <= t.DegN; j++ {
			pt = append(pt, wp{t.Ctrl[i][j], t.Weight[i][j]})
		}
	}
	if !matchAll(len(ps), func(i, j int) bool {
		return ps[i].p.Equals(pt[j].p) && math.Abs(ps[i].w-pt[j].w) < geom.LengthEps
	}) {
		return false
	}
	if len(s.Trim) != len(t.Trim) {
		return false
	}
	return matchAll(len(s.Trim), func(i, j int) bool {
		a, b := s.Trim[i], t.Trim[j]
		return (a.StartP.Equals(b.StartP) && a.FinishP.Equals(b.FinishP)) ||
			(a.StartP.Equals(b.FinishP) && a.FinishP.Equals(b.StartP))
	})
}

// matchAll reports whether the n items of one list can each be paired
// with a different one of n items of another by same, taken greedily.
func matchAll(n int, same func(i, j int) bool) bool {
	used := make([]bool, n)
	for i := 0; i < n; i++ {
		found := false
		for j := 0; j < n; j++ {
			if !used[j] && same(i, j) {
				used[j], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
