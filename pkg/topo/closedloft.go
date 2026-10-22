package topo

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A closed loft goes through its profiles, three or more, and from the
// last back to the first, as a ring: one surface closed both ways, with
// no caps, as a sweep round a closed path is. The surface is skinned
// through the profiles and the first again; where it closes, both ends
// take one slope across, that of the profiles either side of the first,
// so the seam is as smooth as the rest of the ring.

// ClosedLoftSections are the loops a closed loft goes through, as it
// takes them: of equal counts, each running counter-clockwise about the
// way round the ring there, from the one before it to the one after,
// and each after the first turned to start nearest where the one before
// it does. The last goes back to the first as the first is: profiles
// drawn turned a little more each time round the ring (a twisted ring)
// take what is left of the turn between the last and the first.
func ClosedLoftSections(loops [][]srf.SBezier) ([][]srf.SBezier, error) {
	k := len(loops)
	if k < 3 {
		return nil, errors.New("a closed loft needs three profiles at least")
	}
	for i, l := range loops {
		if err := checkLoop(l); err != nil {
			return nil, fmt.Errorf("profile %d: %w", i+1, err)
		}
		if newell(l).Magnitude() < geom.LengthEps {
			return nil, fmt.Errorf("profile %d encloses no area", i+1)
		}
	}
	loops = MatchCounts(loops)
	for i := range loops {
		if centroid(loops[(i+1)%k]).Minus(centroid(loops[i])).Magnitude() < geom.LengthEps {
			return nil, fmt.Errorf("profiles %d and %d lie on each other", i+1, (i+1)%k+1)
		}
		// The way round the ring at a profile, from the one before to
		// the one after: at the first and the last this takes the seam
		// in as any other.
		along := centroid(loops[(i+1)%k]).Minus(centroid(loops[(i+k-1)%k]))
		if along.Magnitude() < geom.LengthEps {
			return nil, fmt.Errorf("the profiles either side of profile %d lie on each other", i+1)
		}
		if newell(loops[i]).Dot(along) < 0 {
			loops[i] = reverseLoop(loops[i])
		}
	}
	for i := 1; i < k; i++ {
		loops[i] = alignTo(loops[i], loops[i-1])
	}
	return loops, nil
}

// ClosedLoftSurface is the B-spline surface of a closed loft through
// loops (see ClosedLoftSections), u along the sections, v across from
// the first profile round to it again.
func ClosedLoftSurface(loops [][]srf.SBezier) (*srf.BSplineSurface, error) {
	loops, err := ClosedLoftSections(loops)
	if err != nil {
		return nil, err
	}
	deg := 1
	for _, l := range loops {
		for _, sb := range l {
			deg = max(deg, sb.Degree)
		}
	}
	k := len(loops)
	curves := make([]srf.BSplineCurve, k+1)
	for i, l := range loops {
		if curves[i], err = srf.ChainCurve(l, deg); err != nil {
			return nil, err
		}
	}
	curves[k] = curves[0]
	// Chord lengths across, round to the first again.
	ts := make([]float64, k+1)
	for s := 1; s <= k; s++ {
		d := 0.0
		for i := range curves[0].Ctrl {
			d += curves[s].Ctrl[i].Minus(curves[s-1].Ctrl[i]).Magnitude()
		}
		ts[s] = ts[s-1] + d/float64(len(curves[0].Ctrl))
	}
	for s := range ts {
		ts[s] /= ts[k]
	}
	ts[k] = 1
	// The slope across at the seam, in homogeneous coordinates, from the
	// profile after the first to the one before it (the last), over the
	// parameter between them through the seam: both ends take it, so the
	// surface's derivative across is one there.
	h := func(s, i int) geom.Vector4 {
		return geom.Vector4FromWeighted(curves[s].Weight[i], curves[s].Ctrl[i])
	}
	span := ts[1] + 1 - ts[k-1]
	slope := make([]geom.Vector4, len(curves[0].Ctrl))
	for i := range slope {
		slope[i] = h(1, i).Minus(h(k-1, i)).ScaledBy(1 / span)
	}
	return srf.SkinThrough(curves, ts, slope, slope)
}

// MakeClosedLoft is the shell of a closed loft through loops: one face
// of the surface's patches, their identities from origin on, sheet
// origin, and no caps.
func MakeClosedLoft(loops [][]srf.SBezier, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	bs, err := ClosedLoftSurface(loops)
	if err != nil {
		return nil, err
	}
	return joinSkins([][]*srf.BSplineSurface{{bs}}, color, origin, tol)
}
