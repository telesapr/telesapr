package topo

import (
	"errors"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A fitted sweep whose path comes back within the profile's reach of
// where it has been passes through itself. The sketch makes it in spans
// each clear of itself, joined by Booleans: each span is swept along its
// stretch of the path with the whole path's frames, so the twist, the
// held frames and the closing turn are the same as in one piece, and
// capped where it is cut, the caps of neighbouring spans flush. Round a
// closed path the first span starts and the last ends where the profile
// is drawn, so their caps meet there too.

// ErrSpanRigid is MakePathSweepSpan's error for a path with a run swept
// rigidly beside a corner at an arc, whose joint holds the run whole.
var ErrSpanRigid = errors.New("the path is swept rigidly beside a corner and can't be cut")

// Spannable reports whether the path can be swept in spans: it has no
// run swept rigidly beside a corner at an arc.
func (sp *SweepPath) Spannable() bool { return !slices.Contains(sp.rigid, true) }

// spanCutEnd is how near, by the chain's parameter, a cut may come to
// the end of a span: nearer would leave a span of no length.
const spanCutEnd = 1e-6

// CutAt is where to cut the path between lo and hi, by the chain's
// parameter, at about the fraction f of the arc length between: never
// inside a straight piece beside a corner, whose skin is made whole and
// mitred, but at its end away from the corner. False where no such
// place lies inside the span.
func (sp *SweepPath) CutAt(lo, hi, f float64) (float64, bool) {
	a0, a1 := sp.frameAt(lo).arc, sp.frameAt(hi).arc
	want := a0 + f*(a1-a0)
	s0, s1 := lo, hi
	for range 60 {
		m := (s0 + s1) / 2
		if sp.frameAt(m).arc < want {
			s0 = m
		} else {
			s1 = m
		}
	}
	s := (s0 + s1) / 2
	if i := min(int(math.Floor(s)), len(sp.pieces)-1); sp.prism[i] {
		switch {
		case !sp.corner[i] && (s-float64(i) <= float64(i+1)-s || sp.corner[i+1]):
			s = float64(i)
		case !sp.corner[i+1]:
			s = float64(i + 1)
		default:
			return 0, false
		}
	}
	if s-lo < spanCutEnd || hi-s < spanCutEnd {
		return 0, false
	}
	return s, true
}

// MakePathSweepSpan is MakePathSweep along the path from lo to hi only,
// by the chain's parameter, capped at both: the start cap takes
// identity startFace, the end endFace, and the surfaces' patches
// identities from origin on. Closed or not, the span has its caps.
func MakePathSweepSpan(sp *SweepPath, loops [][]srf.SBezier, color core.RGBA, startFace, endFace, origin uint32, tol srf.Tolerance, lo, hi float64) (*srf.SShell, error) {
	if !sp.Spannable() {
		return nil, ErrSpanRigid
	}
	sp.tol = tol
	surfs, _, err := sp.sweptSurfacesIn(loops, lo, hi)
	if err != nil {
		return nil, err
	}
	return closeSkins(surfs, color, startFace, endFace, origin, tol, sp.pace)
}
