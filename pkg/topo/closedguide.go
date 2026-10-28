package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A closed loft is held by guides as an open one is (guidedloft.go),
// each guide a closed curve going round the ring through every profile:
// the ring without guides gives sections across between each two
// profiles, the last two the first's again, and each is moved within its
// plane to hold the guides. The surface is skinned through the moved
// sections and the first profile again, both ends taking one slope
// across, that of the moved sections either side of the seam, so it
// closes as smoothly as the free ring does.

// ClosedGuidedLoftSurface is the surface of a closed loft through loops
// (see ClosedLoftSections) held by guides, one or two; none is
// ClosedLoftSurface.
func ClosedGuidedLoftSurface(loops [][]srf.SBezier, guides []LoftGuide) (*srf.BSplineSurface, error) {
	if len(guides) == 0 {
		return ClosedLoftSurface(loops)
	}
	if len(guides) > 2 {
		return nil, fmt.Errorf("a loft takes two guides at most, not %d", len(guides))
	}
	r, err := closedLoftRing(loops)
	if err != nil {
		return nil, err
	}
	free, err := ClosedLoftSurface(loops)
	if err != nil {
		return nil, err
	}
	k := len(r.loops)
	gs := make([]*guideTrack, len(guides))
	for i, g := range guides {
		if gs[i], err = newClosedGuideTrack(r, g, i+1); err != nil {
			return nil, err
		}
	}
	if len(gs) == 2 {
		for i := range k {
			if gs[0].points[i].Minus(gs[1].points[i]).Magnitude() < guideTol {
				return nil, fmt.Errorf("the two guides meet profile %d at one point", i+1)
			}
		}
	}
	sections, vs, _, err := guidedSections(free, r.ts, r.curves, k, gs)
	if err != nil {
		return nil, err
	}
	// The slope across at the seam, in homogeneous coordinates, from the
	// section after the first to the one before the last, over the
	// parameter between them through the seam.
	last := len(sections) - 1
	span := vs[1] + 1 - vs[last-1]
	slope := make([]geom.Vector4, len(sections[0].Ctrl))
	for i := range slope {
		a := geom.Vector4FromWeighted(sections[1].Weight[i], sections[1].Ctrl[i])
		b := geom.Vector4FromWeighted(sections[last-1].Weight[i], sections[last-1].Ctrl[i])
		slope[i] = a.Minus(b).ScaledBy(1 / span)
	}
	return srf.SkinThrough(sections, vs, slope, slope)
}

// MakeClosedGuidedLoft is the shell of ClosedGuidedLoftSurface, as
// MakeClosedLoft's: one face, sheet origin, no caps.
func MakeClosedGuidedLoft(loops [][]srf.SBezier, guides []LoftGuide, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	bs, err := ClosedGuidedLoftSurface(loops, guides)
	if err != nil {
		return nil, err
	}
	return joinSkins([][]*srf.BSplineSurface{{bs}}, color, origin, tol)
}

// newClosedGuideTrack finds where the closed guide g meets each profile
// of the ring r, and the first again: g is started where it crosses the
// first profile and turned to go the way round the ring.
func newClosedGuideTrack(r closedRing, g LoftGuide, number int) (*guideTrack, error) {
	if len(g) < 3 || g[0].Minus(g[len(g)-1]).Magnitude() > guideTol {
		return nil, fmt.Errorf("guide %d doesn't close on itself: a closed loft's guide goes round the ring", number)
	}
	g, err := startGuideAt(g, r.curves[0], newell(r.loops[0]).WithMagnitude(1), centroid(r.loops[0]))
	if err != nil {
		return nil, fmt.Errorf("guide %d: %w", number, err)
	}
	k := len(r.loops)
	t := &guideTrack{number: number, pts: g}
	for i := 0; i <= k; i++ {
		var p geom.Vector
		switch i {
		case 0:
			p = g[0]
		case k:
			p = g[len(g)-1]
		default:
			n := newell(r.loops[i]).WithMagnitude(1)
			q, ok := t.cross(centroid(r.loops[i]), n)
			if !ok {
				return nil, fmt.Errorf("guide %d doesn't reach profile %d's plane", number, i+1)
			}
			p = q
		}
		u, d := nearestOnCurve(r.curves[i], p)
		if d > guideTol {
			return nil, fmt.Errorf("guide %d misses profile %d by %.4g mm", number, i%k+1, d)
		}
		if i > 0 {
			u += math.Round(t.us[i-1] - u)
		}
		t.points = append(t.points, p)
		t.us = append(t.us, u)
	}
	t.at = 0
	return t, nil
}

// startGuideAt is the closed guide g started where it crosses the plane
// through m of normal n on the curve c, the first profile, and running
// from there the way n points, round to that point again.
func startGuideAt(g LoftGuide, c srf.BSplineCurve, n, m geom.Vector) (LoftGuide, error) {
	side := func(p geom.Vector) float64 { return p.Minus(m).Dot(n) }
	for i := 0; i+1 < len(g); i++ {
		a, b := g[i], g[i+1]
		sa, sb := side(a), side(b)
		if sa*sb > 0 || sa == sb {
			continue
		}
		p := a.Plus(b.Minus(a).ScaledBy(sa / (sa - sb)))
		if _, d := nearestOnCurve(c, p); d > guideTol {
			continue
		}
		// From p on to the end, which is the start, and on round to p.
		out := LoftGuide{p}
		out = append(out, g[i+1:]...)
		out = append(out, g[1:i+1]...)
		out = append(out, p)
		if sb < sa {
			// It goes against the way round: the other way.
			for l, h := 0, len(out)-1; l < h; l, h = l+1, h-1 {
				out[l], out[h] = out[h], out[l]
			}
		}
		return out, nil
	}
	return nil, errors.New("it doesn't pass through the first profile")
}
