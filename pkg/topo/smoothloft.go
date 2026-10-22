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
	for k := range n {
		d := 0.0
		for i := range n {
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
			return nil, fmt.Errorf("profile %d: %w", i+1, err)
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
func MakeSmoothLoft(loops [][]srf.SBezier, turn int, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	bs, err := SmoothLoftSurface(loops, turn)
	if err != nil {
		return nil, err
	}
	return closeSkins([][]*srf.BSplineSurface{{bs}}, color, origin, origin+1, origin+2, tol)
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

// LoftEnds sets how a smooth loft leaves its first profile and reaches
// its last (roadmap 28.3): Start and End, when not 0, make the surface
// square to that profile's plane there, the derivative across that
// many times the length of the loft from profile to profile; 0 leaves
// the end free, as the sections alone shape it.
type LoftEnds struct {
	Start, End float64
}

// LoftSides, where set, says which way a smooth loft leaves each point
// of its first profile (Start) and reaches each point of its last
// (End), a unit vector the way the loft goes, in place of square to the
// profile's plane: along a solid's sides (SideTangents). It takes effect
// where LoftEnds sets that end.
type LoftSides struct {
	Start, End func(p geom.Vector) (geom.Vector, error)
}

// MakeSmoothLoftEnds is MakeSmoothLoft with its ends set by ends; two
// profiles make a cubic surface, not a ruled one, when an end is set.
func MakeSmoothLoftEnds(loops [][]srf.SBezier, turn int, ends LoftEnds, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	return MakeSmoothLoftSides(loops, turn, ends, LoftSides{}, color, origin, tol)
}

// MakeSmoothLoftSides is MakeSmoothLoftEnds with the directions its ends
// leave and reach the profiles in set by sides.
func MakeSmoothLoftSides(loops [][]srf.SBezier, turn int, ends LoftEnds, sides LoftSides, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	if ends.Start == 0 && ends.End == 0 {
		bs, err := SmoothLoftSurface(loops, turn)
		if err != nil {
			return nil, err
		}
		return closeSkins([][]*srf.BSplineSurface{{bs}}, color, origin, origin+1, origin+2, tol)
	}
	b, err := smoothLoftBase(loops, turn, ends, sides)
	if err != nil {
		return nil, err
	}
	return closeSkins([][]*srf.BSplineSurface{{b.surface}}, color, origin, origin+1, origin+2, tol)
}

// SmoothLoftSurfaceEnds is SmoothLoftSurface with its ends set.
func SmoothLoftSurfaceEnds(loops [][]srf.SBezier, turn int, ends LoftEnds) (*srf.BSplineSurface, error) {
	if ends.Start == 0 && ends.End == 0 {
		return SmoothLoftSurface(loops, turn)
	}
	b, err := smoothLoftBase(loops, turn, ends, LoftSides{})
	if err != nil {
		return nil, err
	}
	return b.surface, nil
}

// loftBase is a smooth loft's surface with what made it: the loops as
// SmoothLoftSections takes them, each as one curve, and the parameter
// across of each.
type loftBase struct {
	loops   [][]srf.SBezier
	curves  []srf.BSplineCurve
	surface *srf.BSplineSurface
	ts      []float64
}

// smoothLoftBase is the smooth loft through loops with its ends set by
// ends, free or not.
func smoothLoftBase(loops [][]srf.SBezier, turn int, ends LoftEnds, sides LoftSides) (loftBase, error) {
	for _, x := range []float64{ends.Start, ends.End} {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return loftBase{}, fmt.Errorf("a loft's tangency at an end is a number, not %g", x)
		}
	}
	loops, err := SmoothLoftSections(loops, turn)
	if err != nil {
		return loftBase{}, err
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
			return loftBase{}, err
		}
	}
	// The free surface gives the parameters and the free ends.
	free, ts, err := srf.Skin(curves)
	if err != nil {
		return loftBase{}, err
	}
	b := loftBase{loops: loops, curves: curves, surface: free, ts: ts}
	if ends.Start == 0 && ends.End == 0 {
		return b, nil
	}
	length := 0.0
	for i := 1; i < len(loops); i++ {
		length += centroid(loops[i]).Minus(centroid(loops[i-1])).Magnitude()
	}
	k := len(curves)
	nv := len(free.Ctrl[0])
	dv := free.DegV
	start := make([]geom.Vector, len(curves[0].Ctrl))
	end := make([]geom.Vector, len(curves[0].Ctrl))
	n0 := newell(loops[0]).WithMagnitude(1)
	n1 := newell(loops[k-1]).WithMagnitude(1)
	startAlong := centroid(loops[1]).Minus(centroid(loops[0]))
	endAlong := centroid(loops[k-1]).Minus(centroid(loops[k-2]))
	// along is which way the loft goes through the point at control
	// point i's Greville abscissa of curve c, by side, or nil.
	along := func(side func(geom.Vector) (geom.Vector, error), c *srf.BSplineCurve, i int, way geom.Vector) (geom.Vector, error) {
		t := 0.0
		for j := 1; j <= c.Deg; j++ {
			t += c.Knots[i+j]
		}
		d, err := side(c.PointAt(t / float64(c.Deg)))
		if err != nil {
			return d, err
		}
		if d.Dot(way) <= 0 {
			return d, errSidesAgainst
		}
		return d, nil
	}
	for i := range start {
		// The free surface's derivatives across at its ends; its
		// weights don't change across, so its rows are polynomial there.
		a := float64(dv) / free.KnotsV[dv+1]
		start[i] = free.Ctrl[i][1].Minus(free.Ctrl[i][0]).ScaledBy(a)
		b := float64(dv) / (1 - free.KnotsV[len(free.KnotsV)-dv-2])
		end[i] = free.Ctrl[i][nv-1].Minus(free.Ctrl[i][nv-2]).ScaledBy(b)
		if ends.Start != 0 {
			start[i] = n0.ScaledBy(ends.Start * length)
			if sides.Start != nil {
				d, err := along(sides.Start, &curves[0], i, startAlong)
				if err != nil {
					return loftBase{}, fmt.Errorf("the first profile: %w", err)
				}
				start[i] = d.ScaledBy(ends.Start * length)
			}
		}
		if ends.End != 0 {
			end[i] = n1.ScaledBy(ends.End * length)
			if sides.End != nil {
				d, err := along(sides.End, &curves[k-1], i, endAlong)
				if err != nil {
					return loftBase{}, fmt.Errorf("the last profile: %w", err)
				}
				end[i] = d.ScaledBy(ends.End * length)
			}
		}
	}
	if b.surface, err = srf.SkinWithEnds(curves, ts, start, end); err != nil {
		return loftBase{}, err
	}
	return b, nil
}

// MakeSmoothLoftThrough is MakeSmoothLoftSides held square to profiles
// between its ends too: middle[j], when not 0, makes the surface square
// to the plane of profile j + 2 (the first between the ends is j 0),
// its slope across there middle[j] times the loft's length, as an end's
// tangency does. The loft is then one surface from each such profile to
// the next, meeting tangent there, all one face.
func MakeSmoothLoftThrough(loops [][]srf.SBezier, turn int, ends LoftEnds, sides LoftSides, middle []float64, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	bss, err := SmoothLoftThroughSurfaces(loops, turn, ends, sides, middle)
	if err != nil {
		return nil, err
	}
	return closeSkins([][]*srf.BSplineSurface{bss}, color, origin, origin+1, origin+2, tol)
}

// SmoothLoftThroughSurfaces are the surfaces of MakeSmoothLoftThrough,
// in order from the first profile.
func SmoothLoftThroughSurfaces(loops [][]srf.SBezier, turn int, ends LoftEnds, sides LoftSides, middle []float64) ([]*srf.BSplineSurface, error) {
	if len(middle) > max(len(loops)-2, 0) {
		return nil, fmt.Errorf("a loft of %d profiles has %d between its ends, not %d", len(loops), max(len(loops)-2, 0), len(middle))
	}
	for _, x := range middle {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("a loft's tangency at a profile is a number, not %g", x)
		}
	}
	base, err := smoothLoftBase(loops, turn, ends, sides)
	if err != nil {
		return nil, err
	}
	k := len(base.curves)
	cuts := []int{0}
	for j, x := range middle {
		if x != 0 {
			cuts = append(cuts, j+1)
		}
	}
	cuts = append(cuts, k-1)
	if len(cuts) == 2 {
		return []*srf.BSplineSurface{base.surface}, nil
	}
	length := 0.0
	for i := 1; i < k; i++ {
		length += centroid(base.loops[i]).Minus(centroid(base.loops[i-1])).Magnitude()
	}
	// The slopes across, in homogeneous coordinates against the whole
	// loft's v: at the ends the loft's own, at a cut square to the
	// profile's plane, which SmoothLoftSections turned to face the way
	// the loft goes.
	bs := base.surface
	dv := bs.DegV
	nv := len(bs.Ctrl[0])
	a := float64(dv) / (bs.KnotsV[dv+1] - bs.KnotsV[0])
	b := float64(dv) / (bs.KnotsV[len(bs.KnotsV)-1] - bs.KnotsV[len(bs.KnotsV)-dv-2])
	slope := func(s int) []geom.Vector4 {
		out := make([]geom.Vector4, len(bs.Ctrl))
		for i := range bs.Ctrl {
			h := func(j int) geom.Vector4 { return geom.Vector4FromWeighted(bs.Weight[i][j], bs.Ctrl[i][j]) }
			switch s {
			case 0:
				out[i] = h(1).Minus(h(0)).ScaledBy(a)
			case k - 1:
				out[i] = h(nv - 1).Minus(h(nv - 2)).ScaledBy(b)
			default:
				n := newell(base.loops[s]).WithMagnitude(middle[s-1] * length)
				out[i] = geom.Vector4FromWeighted(base.curves[s].Weight[i], n)
				out[i].W = 0
			}
		}
		return out
	}
	var out []*srf.BSplineSurface
	for c := 1; c < len(cuts); c++ {
		from, to := cuts[c-1], cuts[c]
		span := base.ts[to] - base.ts[from]
		ts := make([]float64, 0, to-from+1)
		for s := from; s <= to; s++ {
			ts = append(ts, (base.ts[s]-base.ts[from])/span)
		}
		ts[len(ts)-1] = 1
		start, end := slope(from), slope(to)
		for i := range start {
			start[i], end[i] = start[i].ScaledBy(span), end[i].ScaledBy(span)
		}
		piece, err := srf.SkinThrough(base.curves[from:to+1], ts, start, end)
		if err != nil {
			return nil, fmt.Errorf("from profile %d to %d: %w", from+1, to+1, err)
		}
		out = append(out, piece)
	}
	return out, nil
}
