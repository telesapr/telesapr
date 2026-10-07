package topo

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Guide curves (roadmap 28.3) steer a smooth loft between its profiles:
// each runs from a point of the first profile through every profile to
// the last, and the loft's surface is moved so that it holds the
// guides. The loft without guides gives sections across, at
// guideSamples steps between each two profiles; each section is moved
// within its own plane so that the point of it that a guide holds lies
// on the guide, where the guide crosses that plane. One guide moves a
// section; two move, turn and scale it, each holding its own point.
// The surface is then skinned through the moved sections, so it holds
// the guides exactly at the sections and closely between them, and
// still goes through each profile, which the guides pass through.

// LoftGuide is a guide curve, as points along it close enough together
// that it is straight between them.
type LoftGuide []geom.Vector

// guideSamples is how many sections a guided loft takes between two
// profiles.
const guideSamples = 12

// guideTol is how far off a profile a guide may pass it.
const guideTol = 1e-3

// MakeGuidedLoft is MakeSmoothLoftEnds held to guides, one or two;
// none is MakeSmoothLoftEnds.
func MakeGuidedLoft(loops [][]srf.SBezier, turn int, ends LoftEnds, guides []LoftGuide, color core.RGBA, origin uint32, tol srf.Tolerance) (*srf.SShell, error) {
	bs, err := GuidedLoftSurface(loops, turn, ends, guides)
	if err != nil {
		return nil, err
	}
	return closeSkins([][]*srf.BSplineSurface{{bs}}, color, origin, origin+1, origin+2, tol)
}

// GuidedLoftSurface is the surface of MakeGuidedLoft.
func GuidedLoftSurface(loops [][]srf.SBezier, turn int, ends LoftEnds, guides []LoftGuide) (*srf.BSplineSurface, error) {
	if len(guides) == 0 {
		return SmoothLoftSurfaceEnds(loops, turn, ends)
	}
	if len(guides) > 2 {
		return nil, fmt.Errorf("a loft takes two guides at most, not %d", len(guides))
	}
	base, err := smoothLoftBase(loops, turn, ends)
	if err != nil {
		return nil, err
	}
	k := len(base.curves)
	gs := make([]*guideTrack, len(guides))
	for i, g := range guides {
		if gs[i], err = newGuideTrack(base, g, i+1); err != nil {
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

	// The sections: the profiles themselves, and between each two
	// guideSamples - 1 of the loft's own, moved.
	var sections []srf.BSplineCurve
	var vs []float64
	var moved [][]geom.Vector // each section's control points' moves
	for i := range k {
		if i > 0 {
			for s := 1; s < guideSamples; s++ {
				v := base.ts[i-1] + (base.ts[i]-base.ts[i-1])*float64(s)/guideSamples
				c := base.surface.IsoV(v)
				n := curveNormal(c)
				if n.Magnitude() < geom.LengthEps {
					return nil, fmt.Errorf("the loft's section at %.3g of the way has no plane for a guide to cross", v)
				}
				n = n.WithMagnitude(1)
				var from, to []geom.Vector
				for _, g := range gs {
					m := c.PointAt(g.uAt(v, base.ts))
					p, ok := g.cross(m, n)
					if !ok {
						return nil, fmt.Errorf("guide %d doesn't cross the loft's section %.3g of the way from profile %d to %d",
							g.number, (v-base.ts[i-1])/(base.ts[i]-base.ts[i-1]), i, i+1)
					}
					from, to = append(from, m), append(to, p)
				}
				move, err := guideMove(from, to, n)
				if err != nil {
					return nil, fmt.Errorf("%w between profiles %d and %d", err, i, i+1)
				}
				d := make([]geom.Vector, len(c.Ctrl))
				for j, q := range c.Ctrl {
					c.Ctrl[j] = move(q)
					d[j] = c.Ctrl[j].Minus(q)
				}
				sections, vs, moved = append(sections, c), append(vs, v), append(moved, d)
			}
		}
		sections = append(sections, base.curves[i])
		vs = append(vs, base.ts[i])
		moved = append(moved, make([]geom.Vector, len(base.curves[i].Ctrl)))
	}

	// The derivatives across at the ends: the loft's own, and the
	// guides' turn there too unless the end is set square.
	bs := base.surface
	dv := bs.DegV
	nv := len(bs.Ctrl[0])
	a := float64(dv) / (bs.KnotsV[dv+1] - bs.KnotsV[0])
	b := float64(dv) / (bs.KnotsV[len(bs.KnotsV)-1] - bs.KnotsV[len(bs.KnotsV)-dv-2])
	start := make([]geom.Vector4, len(bs.Ctrl))
	end := make([]geom.Vector4, len(bs.Ctrl))
	last := len(sections) - 1
	for i := range bs.Ctrl {
		h := func(j int) geom.Vector4 { return geom.Vector4FromWeighted(bs.Weight[i][j], bs.Ctrl[i][j]) }
		start[i] = h(1).Minus(h(0)).ScaledBy(a)
		end[i] = h(nv - 1).Minus(h(nv - 2)).ScaledBy(b)
		if ends.Start == 0 {
			w := sections[1].Weight[i]
			start[i] = start[i].Plus(geom.Vector4FromWeighted(w, moved[1][i]).ScaledBy(1 / (vs[1] - vs[0])))
			start[i].W = h(1).Minus(h(0)).ScaledBy(a).W
		}
		if ends.End == 0 {
			w := sections[last-1].Weight[i]
			end[i] = end[i].Minus(geom.Vector4FromWeighted(w, moved[last-1][i]).ScaledBy(1 / (vs[last] - vs[last-1])))
			end[i].W = h(nv - 1).Minus(h(nv - 2)).ScaledBy(b).W
		}
	}
	return srf.SkinThrough(sections, vs, start, end)
}

// curveNormal is the normal of a closed curve's plane, by Newell's
// formula through points along it.
func curveNormal(c srf.BSplineCurve) geom.Vector {
	lo, hi := c.Domain()
	const n = 64
	pts := make([]geom.Vector, n)
	for i := range pts {
		pts[i] = c.PointAt(lo + (hi-lo)*float64(i)/n)
	}
	return newellOf(pts)
}

// guideTrack is a guide as the loft takes it: its points from the first
// profile's end, where it meets each profile, and the parameter along
// each profile's curve of that point, unwrapped so that it changes
// least from profile to profile.
type guideTrack struct {
	number int
	pts    LoftGuide
	at     int // where along pts the last crossing was found
	points []geom.Vector
	us     []float64
}

// newGuideTrack finds where guide g meets each of the loft's profiles.
func newGuideTrack(base loftBase, g LoftGuide, number int) (*guideTrack, error) {
	if len(g) < 2 {
		return nil, fmt.Errorf("guide %d has no length", number)
	}
	k := len(base.curves)
	first, lastC := base.curves[0], base.curves[k-1]
	_, d0 := nearestOnCurve(first, g[0])
	_, d1 := nearestOnCurve(first, g[len(g)-1])
	if d1 < d0 {
		r := make(LoftGuide, len(g))
		for i, p := range g {
			r[len(g)-1-i] = p
		}
		g = r
		d0 = d1
	}
	if d0 > guideTol {
		return nil, fmt.Errorf("guide %d doesn't start on the first profile: it is %.4g mm off it", number, d0)
	}
	if _, d := nearestOnCurve(lastC, g[len(g)-1]); d > guideTol {
		return nil, fmt.Errorf("guide %d doesn't end on the last profile: it is %.4g mm off it", number, d)
	}
	t := &guideTrack{number: number, pts: g}
	for i, c := range base.curves {
		var p geom.Vector
		switch i {
		case 0:
			p = g[0]
		case k - 1:
			p = g[len(g)-1]
		default:
			n := newell(base.loops[i]).WithMagnitude(1)
			q, ok := t.cross(centroid(base.loops[i]), n)
			if !ok {
				return nil, fmt.Errorf("guide %d doesn't reach profile %d's plane", number, i+1)
			}
			p = q
		}
		u, d := nearestOnCurve(c, p)
		if d > guideTol {
			return nil, fmt.Errorf("guide %d misses profile %d by %.4g mm", number, i+1, d)
		}
		if i > 0 {
			prev := t.us[i-1]
			u += math.Round(prev - u)
		}
		t.points = append(t.points, p)
		t.us = append(t.us, u)
	}
	t.at = 0
	return t, nil
}

// uAt is the parameter along the section at v that the guide holds:
// between two profiles' parameters as v is between theirs, wrapped back
// into [0, 1].
func (t *guideTrack) uAt(v float64, ts []float64) float64 {
	i := 1
	for i < len(ts)-1 && v > ts[i] {
		i++
	}
	f := (v - ts[i-1]) / (ts[i] - ts[i-1])
	u := t.us[i-1] + (t.us[i]-t.us[i-1])*f
	return u - math.Floor(u)
}

// cross is where the guide, from where it last crossed on, crosses the
// plane through m of normal n: the first crossing, or none.
func (t *guideTrack) cross(m, n geom.Vector) (geom.Vector, bool) {
	side := func(p geom.Vector) float64 { return p.Minus(m).Dot(n) }
	for i := t.at; i+1 < len(t.pts); i++ {
		a, b := t.pts[i], t.pts[i+1]
		sa, sb := side(a), side(b)
		if sa == 0 {
			t.at = i
			return a, true
		}
		if sa*sb <= 0 {
			t.at = i
			return a.Plus(b.Minus(a).ScaledBy(sa / (sa - sb))), true
		}
	}
	return geom.Vector{}, false
}

// guideMove is the move of a section that takes its points from to the
// guides' points to, within the plane of normal n: a shift for one
// guide; for two, a turn about n and a scale as well.
func guideMove(from, to []geom.Vector, n geom.Vector) (func(geom.Vector) geom.Vector, error) {
	if len(from) == 1 {
		d := to[0].Minus(from[0])
		return func(p geom.Vector) geom.Vector { return p.Plus(d) }, nil
	}
	a, b := from[1].Minus(from[0]), to[1].Minus(to[0])
	// Within the plane.
	a, b = a.Minus(n.ScaledBy(a.Dot(n))), b.Minus(n.ScaledBy(b.Dot(n)))
	if a.Magnitude() < geom.LengthEps || b.Magnitude() < geom.LengthEps {
		return nil, errors.New("the two guides meet a section at one point")
	}
	s := b.Magnitude() / a.Magnitude()
	angle := math.Atan2(a.Cross(b).Dot(n), a.Dot(b))
	o, d := from[0], to[0]
	return func(p geom.Vector) geom.Vector {
		q := p.Minus(o)
		along := n.ScaledBy(q.Dot(n))
		in := q.Minus(along).RotatedAroundAxis(n, angle).ScaledBy(s)
		return d.Plus(in).Plus(along)
	}, nil
}

// nearestOnCurve is the parameter of the curve's point nearest p, and
// how far that is: the nearest of points along it, then narrowed.
func nearestOnCurve(c srf.BSplineCurve, p geom.Vector) (float64, float64) {
	lo, hi := c.Domain()
	const n = 512
	best, bu := math.Inf(1), lo
	for i := 0; i <= n; i++ {
		u := lo + (hi-lo)*float64(i)/n
		if d := c.PointAt(u).Minus(p).Magnitude(); d < best {
			best, bu = d, u
		}
	}
	step := (hi - lo) / n
	for step > 1e-13 {
		for _, u := range []float64{bu - step, bu + step} {
			if u < lo || u > hi {
				continue
			}
			if d := c.PointAt(u).Minus(p).Magnitude(); d < best {
				best, bu = d, u
			}
		}
		step /= 2
	}
	return bu, best
}
