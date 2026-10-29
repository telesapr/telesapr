package sketch

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Between the points a wall check measures, the mesh's wall can be
// thinner than at any of them: under a narrow groove's tip, a floor's
// points a piece's width off the tip's line measure it many times its
// least. A ray square to a flat triangle T that leaves the part
// through triangle H travels a length linear in where it starts, over
// the part of T that H, seen along the ray, covers: so the least over
// that part is at one of its corners. Each pair of triangles a measured
// ray joined is taken so, its corners measured, and a ray from a corner
// that leaves through another triangle, the first hidden from T by it,
// gives a pair taken in the next round. A wall thinner than the points
// about it is then missed only where no measured ray reached the pair
// of triangles it lies between.

const (
	// wallCornerRounds is how many times the pairs that rays from
	// corners join are taken in turn.
	wallCornerRounds = 3
	// wallCornerMost is how many corners a part has measured.
	wallCornerMost = 50000
	// wallCornerIn is how far into the part of T a corner's ray starts,
	// so that it passes through H and not along an edge of it.
	wallCornerIn = 1e-8
)

// measureCorners measures the corners of the parts of triangles that
// the rays found join, wherever the walls there could be thinner than
// below or than the thinnest found, and adds them to found.
func (r *WallReport) measureCorners(w *clearancePart, below float64, found []wallSample) []wallSample {
	least := math.Inf(1)
	for _, f := range found {
		least = min(least, f.spot.Thickness)
	}
	type pair struct{ tri, hit int }
	seen := map[pair]bool{}
	next := slices.Clone(found)
	budget := wallCornerMost
	for range wallCornerRounds {
		// The thinnest first, so that the thinnest found falls soon and
		// fewer corners are measured.
		slices.SortStableFunc(next, func(a, b wallSample) int { return cmp.Compare(a.spot.Thickness, b.spot.Thickness) })
		var more []wallSample
		for _, f := range next {
			p := pair{f.tri, f.hit}
			if f.hit < 0 || seen[p] {
				continue
			}
			seen[p] = true
			for _, c := range wallPairCorners(w.tree.Triangle(f.tri), w.tree.Triangle(f.hit)) {
				if c.t >= max(below, least) || budget == 0 {
					continue
				}
				budget--
				sp, hit, ok := thicknessFrom(w.tree, f.tri, c.at, w.tree.Triangle(f.tri).Normal().WithMagnitude(-1))
				if !ok {
					continue
				}
				r.Samples++
				sp.Group = w.g
				sp.sa = w.tree.Triangle(f.tri).Tag
				least = min(least, sp.Thickness)
				more = append(more, wallSample{spot: sp, tri: f.tri, hit: hit})
			}
		}
		if len(more) == 0 {
			break
		}
		found = append(found, more...)
		next = more
	}
	return found
}

// wallCorner is a corner of the part of a triangle another covers,
// moved a little in: where a ray from it starts, and how far it travels
// to the other's plane.
type wallCorner struct {
	at geom.Vector
	t  float64
}

// wallPairCorners are the corners of the part of triangle t that
// triangle h covers, seen along t's inward normal, with the length of
// the ray from each to h's plane; none where h stands along the ray or
// faces back at it.
func wallPairCorners(t, h *srf.STriangle) []wallCorner {
	n := t.Normal()
	if n.Magnitude() < 1e-12 {
		return nil
	}
	n = n.WithMagnitude(1)
	in := n.ScaledBy(-1)
	nh := h.Normal()
	if nh.Magnitude() < 1e-12 {
		return nil
	}
	nh = nh.WithMagnitude(1)
	cos := nh.Dot(in)
	if cos < 1e-9 {
		return nil
	}
	e1 := t.B.Minus(t.A).WithMagnitude(1)
	e2 := n.Cross(e1)
	flat := func(p geom.Vector) [2]float64 {
		d := p.Minus(t.A)
		return [2]float64{d.Dot(e1), d.Dot(e2)}
	}
	poly := [][2]float64{flat(t.A), flat(t.B), flat(t.C)}
	cut := [][2]float64{flat(h.A), flat(h.B), flat(h.C)}
	if area2(cut) < 0 {
		slices.Reverse(cut)
	}
	if area2(cut) < 1e-18 || area2(poly) < 1e-18 {
		return nil
	}
	for k := range cut {
		poly = clipBy(poly, cut[k], cut[(k+1)%len(cut)])
		if len(poly) < 3 {
			return nil
		}
	}
	var mid [2]float64
	for _, p := range poly {
		mid[0] += p[0] / float64(len(poly))
		mid[1] += p[1] / float64(len(poly))
	}
	out := make([]wallCorner, 0, len(poly))
	for _, p := range poly {
		dx, dy := mid[0]-p[0], mid[1]-p[1]
		if l := math.Hypot(dx, dy); l > 0 {
			k := min(wallCornerIn, l/2) / l
			p[0] += dx * k
			p[1] += dy * k
		}
		at := t.A.Plus(e1.ScaledBy(p[0])).Plus(e2.ScaledBy(p[1]))
		out = append(out, wallCorner{at: at, t: nh.Dot(h.A.Minus(at)) / cos})
	}
	return out
}

// area2 is twice the signed area of a polygon in the plane, positive
// when it turns anticlockwise.
func area2(p [][2]float64) float64 {
	a := 0.0
	for i := range p {
		q, s := p[i], p[(i+1)%len(p)]
		a += q[0]*s[1] - s[0]*q[1]
	}
	return a
}

// clipBy is the part of the convex polygon p on the left of the line
// from a to b.
func clipBy(p [][2]float64, a, b [2]float64) [][2]float64 {
	side := func(q [2]float64) float64 { return (b[0]-a[0])*(q[1]-a[1]) - (b[1]-a[1])*(q[0]-a[0]) }
	var out [][2]float64
	for i := range p {
		q, s := p[i], p[(i+1)%len(p)]
		dq, ds := side(q), side(s)
		if dq >= 0 {
			out = append(out, q)
		}
		if (dq >= 0) != (ds >= 0) {
			f := dq / (dq - ds)
			out = append(out, [2]float64{q[0] + (s[0]-q[0])*f, q[1] + (s[1]-q[1])*f})
		}
	}
	return out
}
