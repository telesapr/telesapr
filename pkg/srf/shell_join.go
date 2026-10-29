package srf

import (
	"errors"
	"fmt"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
)

// Two solids made apart that end on one plane, with outlines that are
// the same curves there, are one solid once the two planes are taken out
// and the curves round them shared: no Boolean, whose contact between
// the coplanar caps would be the hard case. A sweep made of a fitted run
// and an exact one (topo's path sweep beside a corner at an arc) is
// joined so.

// joinEps is how far apart two caps' outline curves may lie and still be
// taken as one: their solids are made by different constructions, whose
// frames there agree to rounding.
const joinEps = 1e-6

// ErrCapsDiffer is a join of two caps whose outlines are not the same
// curves.
var ErrCapsDiffer = errors.New("the caps' outlines differ")

// JoinedAtCaps is s with its surfaces of origins a and b, planes with
// the same outline facing each other, taken out, each curve round b given up
// for the one round a it lies along, which then bounds the surface b's
// bounded on its other side.
func (s *SShell) JoinedAtCaps(a, b uint32) (*SShell, error) {
	ha, err := s.capOf(a)
	if err != nil {
		return nil, err
	}
	hb, err := s.capOf(b)
	if err != nil {
		return nil, err
	}
	round := func(h core.HSSurface) []int {
		var out []int
		for i := range s.Curve {
			if s.Curve[i].SurfA == h || s.Curve[i].SurfB == h {
				out = append(out, i)
			}
		}
		return out
	}
	ra, rb := round(ha), round(hb)
	if len(ra) != len(rb) {
		return nil, fmt.Errorf("%w: %d curves round one, %d round the other", ErrCapsDiffer, len(ra), len(rb))
	}
	// For each curve round b, the one round a in its place, and whether
	// it runs the other way.
	type mate struct {
		h        core.HSCurve
		reversed bool
	}
	mates := map[core.HSCurve]mate{}
	used := make([]bool, len(ra))
	for _, ib := range rb {
		cb := &s.Curve[ib]
		found := false
		for j, ia := range ra {
			if used[j] {
				continue
			}
			ca := &s.Curve[ia]
			same, reversed := sameCurve(ca, cb)
			if !same {
				continue
			}
			used[j], found = true, true
			mates[cb.H] = mate{ca.H, reversed}
			// ca now bounds what cb bounded beyond b.
			other := cb.SurfA
			if other == hb {
				other = cb.SurfB
			}
			if ca.SurfA == ha {
				ca.SurfA = other
			} else {
				ca.SurfB = other
			}
			break
		}
		if !found {
			return nil, fmt.Errorf("%w: no curve round one cap lies along the curve from %v", ErrCapsDiffer, cb.Points[0].P)
		}
	}
	out := &SShell{Tol: s.Tol}
	for i := range s.Curve {
		if _, gone := mates[s.Curve[i].H]; !gone {
			out.Curve = append(out.Curve, s.Curve[i])
		}
	}
	for i := range s.Surface {
		ss := s.Surface[i]
		if ss.H == ha || ss.H == hb {
			continue
		}
		ss.Trim = slices.Clone(ss.Trim)
		for k := range ss.Trim {
			if m, ok := mates[ss.Trim[k].Curve]; ok {
				ss.Trim[k].Curve = m.h
				ss.Trim[k].Backwards = ss.Trim[k].Backwards != m.reversed
			}
		}
		out.Surface = append(out.Surface, ss)
	}
	// Numbered in order again.
	var joined SShell
	joined.MakeFromAssemblyOf(out, &SShell{Tol: s.Tol})
	return &joined, nil
}

// capOf is the one surface of origin o.
func (s *SShell) capOf(o uint32) (core.HSSurface, error) {
	var h core.HSSurface
	n := 0
	for i := range s.Surface {
		if s.Surface[i].Origin == o {
			h = s.Surface[i].H
			n++
		}
	}
	if n != 1 {
		return h, fmt.Errorf("srf: %d surfaces of origin %d to join at", n, o)
	}
	return h, nil
}

// sameCurve reports whether a and b lie along each other, by their ends
// and points between, and whether b runs the other way.
func sameCurve(a, b *SCurve) (same, reversed bool) {
	pa, pb := a.Points, b.Points
	if len(pa) < 2 || len(pb) < 2 {
		return false, false
	}
	a0, a1 := pa[0].P, pa[len(pa)-1].P
	b0, b1 := pb[0].P, pb[len(pb)-1].P
	switch {
	case a0.Minus(b0).Magnitude() < joinEps && a1.Minus(b1).Magnitude() < joinEps:
	case a0.Minus(b1).Magnitude() < joinEps && a1.Minus(b0).Magnitude() < joinEps:
		reversed = true
	default:
		return false, false
	}
	// A closed curve's ends say nothing of where it runs, nor two curves
	// between the same points of an outline: the points between must lie
	// on the other.
	if !a.IsExact || !b.IsExact {
		return false, false
	}
	for _, t := range []float64{0.25, 0.5, 0.75} {
		tb := t
		if reversed {
			tb = 1 - t
		}
		p := a.Exact.PointAt(t)
		if p.Minus(b.Exact.PointAt(tb)).Magnitude() > joinEps*1e3 {
			// Parametrised otherwise: the nearest point of b.
			u, ok := b.Exact.ClosestPointTo(p, false)
			if !ok || p.Minus(b.Exact.PointAt(u)).Magnitude() > joinEps {
				return false, false
			}
		}
	}
	return true, reversed
}
