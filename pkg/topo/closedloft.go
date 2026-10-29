package topo

import (
	"errors"
	"fmt"
	"math"

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
	// Sections between profiles are turned with the ring's frame
	// (ringSections), on a curve leaving each profile square to it: past
	// a third of a turn from one to the next that curve swings wide of
	// the chord between them and loops on itself near half a turn.
	for i := range loops {
		a, b := newell(loops[i]).WithMagnitude(1), newell(loops[(i+1)%k]).WithMagnitude(1)
		if a.Dot(b) < -0.5-1e-6 {
			return nil, fmt.Errorf("profiles %d and %d turn by more than a third of a turn from one to the other; a closed loft needs profiles between them", i+1, (i+1)%k+1)
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
	r, err := closedLoftRing(loops)
	if err != nil {
		return nil, err
	}
	curves, ts, _ := ringSections(r)
	last := len(curves) - 1
	// The slope across at the seam, in homogeneous coordinates, from the
	// section after the first to the one before it (the last), over the
	// parameter between them through the seam: both ends take it, so the
	// surface's derivative across is one there.
	span := ts[1] + 1 - ts[last-1]
	h := func(c srf.BSplineCurve, i int) geom.Vector4 { return geom.Vector4FromWeighted(c.Weight[i], c.Ctrl[i]) }
	slope := make([]geom.Vector4, len(curves[0].Ctrl))
	for i := range slope {
		slope[i] = h(curves[1], i).Minus(h(curves[last-1], i)).ScaledBy(1 / span)
	}
	return srf.SkinThrough(curves, ts, slope, slope)
}

// closedRing is a closed loft's sections as it takes them: the loops
// (ClosedLoftSections), each as one curve, the first again at the end,
// and the parameter round the ring of each, 0 at the first and 1 at it
// again.
type closedRing struct {
	loops  [][]srf.SBezier
	curves []srf.BSplineCurve
	ts     []float64
}

// closedLoftRing is the ring of sections a closed loft through loops
// takes.
func closedLoftRing(loops [][]srf.SBezier) (closedRing, error) {
	loops, err := ClosedLoftSections(loops)
	if err != nil {
		return closedRing{}, err
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
			return closedRing{}, err
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
	return closedRing{loops: loops, curves: curves, ts: ts}, nil
}

// ClosedLoftSurfaces are the surfaces of a closed loft through loops
// held square to profiles: held[j], when not 0, makes the surface square
// to the plane of profile j, its slope across there held[j] times the
// length of the ring from profile to profile, as an open loft's middle
// profiles are held (MakeSmoothLoftThrough). The ring is then one
// surface from each held profile round to the next, meeting tangent
// there; with none held it is ClosedLoftSurface. held is as long as
// loops, or shorter, the rest free.
func ClosedLoftSurfaces(loops [][]srf.SBezier, held []float64) ([]*srf.BSplineSurface, error) {
	if len(held) > len(loops) {
		return nil, fmt.Errorf("a closed loft of %d profiles can't be held at %d", len(loops), len(held))
	}
	var at []int
	for j, x := range held {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("a loft's tangency at a profile is a number, not %g", x)
		}
		if x != 0 {
			at = append(at, j)
		}
	}
	if len(at) == 0 {
		bs, err := ClosedLoftSurface(loops)
		if err != nil {
			return nil, err
		}
		return []*srf.BSplineSurface{bs}, nil
	}
	r, err := closedLoftRing(loops)
	if err != nil {
		return nil, err
	}
	k := len(r.loops)
	length := 0.0
	for i := range k {
		length += centroid(r.loops[(i+1)%k]).Minus(centroid(r.loops[i])).Magnitude()
	}
	// The slope across at held profile s against the ring's parameter:
	// square to its plane, which ClosedLoftSections turned to face the
	// way round.
	slope := func(s int, span float64) []geom.Vector4 {
		n := newell(r.loops[s]).WithMagnitude(held[s] * length * span)
		out := make([]geom.Vector4, len(r.curves[s].Ctrl))
		for i := range out {
			out[i] = geom.Vector4FromWeighted(r.curves[s].Weight[i], n)
			out[i].W = 0
		}
		return out
	}
	// From each held profile round to the next, past the seam where it
	// lies between: the parameter goes on past 1 there.
	curves, cts, idx := ringSections(r)
	d := len(curves) - 1
	param := func(s int) float64 { return float64(s/d) + cts[s%d] }
	var out []*srf.BSplineSurface
	for c, from := range at {
		to := at[(c+1)%len(at)]
		if to <= from {
			to += k
		}
		span := float64(to/k) + r.ts[to%k] - r.ts[from]
		lo, hi := idx[from], idx[to%k]+to/k*d
		sections := make([]srf.BSplineCurve, 0, hi-lo+1)
		ts := make([]float64, 0, hi-lo+1)
		for s := lo; s <= hi; s++ {
			sections = append(sections, curves[s%d])
			ts = append(ts, (param(s)-param(lo))/span)
		}
		ts[len(ts)-1] = 1
		piece, err := srf.SkinThrough(sections, ts, slope(from, span), slope(to%k, span))
		if err != nil {
			return nil, fmt.Errorf("from profile %d to %d: %w", from+1, to%k+1, err)
		}
		out = append(out, piece)
	}
	return out, nil
}

// MakeClosedLoft is the shell of a closed loft through loops: one face
// of the surface's patches, their identities from origin on, sheet
// origin, and no caps.
func MakeClosedLoft(loops [][]srf.SBezier, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	return MakeClosedLoftHeld(loops, nil, color, origin, tol)
}

// MakeClosedLoftHeld is MakeClosedLoft held square to profiles as
// ClosedLoftSurfaces says: still one face, of every piece's patches.
func MakeClosedLoftHeld(loops [][]srf.SBezier, held []float64, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	bss, err := ClosedLoftSurfaces(loops, held)
	if err != nil {
		return nil, err
	}
	return joinSkins([][]*srf.BSplineSurface{bss}, color, origin, tol)
}
