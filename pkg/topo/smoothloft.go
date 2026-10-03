package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A smooth loft (roadmap E8) joins closed profiles, three or more, by
// one surface through them all: each profile written as one B-spline
// curve, the curves made of one degree and knot vector, then
// interpolated across (srf.Skin). The surface is exact B-spline output,
// so no tolerance enters; the end profiles' planes close it.

// curveLength is a curve's length, by sampling.
func curveLength(sb srf.SBezier) float64 {
	const n = 16
	l := 0.0
	p := sb.Start()
	for i := 1; i <= n; i++ {
		q := sb.PointAt(float64(i) / n)
		l += q.Minus(p).Magnitude()
		p = q
	}
	return l
}

// MatchCounts splits curves of the loops until each has as many as the
// loop with the most (roadmap 20.1): the longest curve of a loop that
// has fewer is cut in two at its middle, again and again. A triangle
// lofted to a square has its longest side split; a circle of four arcs
// is already a square's match. The loops come back new.
func MatchCounts(loops [][]srf.SBezier) [][]srf.SBezier {
	n := 0
	for _, l := range loops {
		n = max(n, len(l))
	}
	out := make([][]srf.SBezier, len(loops))
	for li, l := range loops {
		l = append([]srf.SBezier(nil), l...)
		for len(l) < n {
			longest, best := 0, -1.0
			for i, sb := range l {
				if d := curveLength(sb); d > best+geom.LengthEps {
					longest, best = i, d
				}
			}
			var a, b srf.SBezier
			l[longest].SplitAt(0.5, &a, &b)
			l = append(l[:longest], append([]srf.SBezier{a, b}, l[longest+1:]...)...)
		}
		out[li] = l
	}
	return out
}

// alignTo turns loop, curve by curve, so that its joints lie closest to
// those of prev, each measured from its loop's centre: the sum of the
// squared distances is least.
func alignTo(loop, prev []srf.SBezier) []srf.SBezier {
	n := len(loop)
	cl, cp := centroid(loop), centroid(prev)
	best, bestK := math.Inf(1), 0
	for k := 0; k < n; k++ {
		d := 0.0
		for i := 0; i < n; i++ {
			a := loop[(i+k)%n].Start().Minus(cl)
			b := prev[i].Start().Minus(cp)
			d += a.Minus(b).MagSquared()
		}
		if d < best-geom.LengthEps {
			best, bestK = d, k
		}
	}
	return append(append([]srf.SBezier(nil), loop[bestK:]...), loop[:bestK]...)
}

// SmoothLoftSections are the loops a smooth loft goes through, as it
// takes them: of equal counts (MatchCounts), each running
// counter-clockwise about the way to the next, and each after the first
// turned to start nearest where the one before it does, then turned on
// by turn curves more.
func SmoothLoftSections(loops [][]srf.SBezier, turn int) ([][]srf.SBezier, error) {
	if len(loops) < 2 {
		return nil, errors.New("a loft needs two profiles at least")
	}
	for i, l := range loops {
		if err := checkLoop(l); err != nil {
			return nil, fmt.Errorf("profile %d: %v", i+1, err)
		}
		if newell(l).Magnitude() < geom.LengthEps {
			return nil, fmt.Errorf("profile %d encloses no area", i+1)
		}
	}
	loops = MatchCounts(loops)
	n := len(loops[0])
	for i := range loops {
		var along geom.Vector
		if i+1 < len(loops) {
			along = centroid(loops[i+1]).Minus(centroid(loops[i]))
		} else {
			along = centroid(loops[i]).Minus(centroid(loops[i-1]))
		}
		if along.Magnitude() < geom.LengthEps {
			return nil, fmt.Errorf("profiles %d and %d lie on each other", i+1, i+2)
		}
		if newell(loops[i]).Dot(along) < 0 {
			loops[i] = reverseLoop(loops[i])
		}
	}
	k := ((turn % n) + n) % n
	for i := 1; i < len(loops); i++ {
		loops[i] = alignTo(loops[i], loops[i-1])
	}
	for i := 1; i < len(loops); i++ {
		loops[i] = append(loops[i][k:], loops[i][:k]...)
	}
	return loops, nil
}

// MakeSmoothLoft is the shell of a smooth loft through closed loops, in
// order, as SmoothLoftSections takes them with turn. The first
// profile's cap takes origin, the last's origin + 1, and the surface's
// patches origins from origin + 2 on, one face of sheet origin + 2.
func MakeSmoothLoft(loops [][]srf.SBezier, turn int, color core.RgbaColor, origin uint32) (*srf.SShell, error) {
	bs, err := SmoothLoftSurface(loops, turn)
	if err != nil {
		return nil, err
	}
	sh := &srf.SShell{}
	if _, err := AddBSplineFace(sh, bs, origin+2, origin+2, color); err != nil {
		return nil, err
	}
	ps, err := bs.Patches()
	if err != nil {
		return nil, err
	}
	// The surface's normal is along u (counter-clockwise about the way
	// on) crossed with v (on), outward. Its patches run the first
	// section's edges with u falling, so its cap runs them with u, and
	// the last's the other way.
	var first, last []srf.SBezier
	for a := range ps {
		first = append(first, ps[a][0].EdgeCurve(0))
		last = append(last, ps[a][len(ps[a])-1].EdgeCurve(2))
	}
	if _, err := AddPlanarFace(sh, [][]srf.SBezier{first}, origin, color); err != nil {
		return nil, fmt.Errorf("the first profile's cap: %v", err)
	}
	if _, err := AddPlanarFace(sh, [][]srf.SBezier{reverseLoop(last)}, origin+1, color); err != nil {
		return nil, fmt.Errorf("the last profile's cap: %v", err)
	}
	t, err := Build(sh)
	if err != nil {
		return nil, err
	}
	if probs := t.Validate(); len(probs) > 0 {
		return nil, fmt.Errorf("the loft's surface is not closed: %s", probs[0].Msg)
	}
	return sh, nil
}

// SmoothLoftSurface is the B-spline surface of a smooth loft through
// loops (see MakeSmoothLoft), u along the sections, v across.
func SmoothLoftSurface(loops [][]srf.SBezier, turn int) (*srf.BSplineSurface, error) {
	loops, err := SmoothLoftSections(loops, turn)
	if err != nil {
		return nil, err
	}
	deg := 1
	for _, l := range loops {
		for _, sb := range l {
			deg = max(deg, sb.Degree)
		}
	}
	curves := make([]srf.BSplineCurve, len(loops))
	for i, l := range loops {
		if curves[i], err = srf.ChainCurve(l, deg); err != nil {
			return nil, err
		}
	}
	bs, _, err := srf.Skin(curves)
	return bs, err
}
