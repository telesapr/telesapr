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
// gives a pair taken in the next round. The pairs no measured ray
// reached are found by where the triangles lie (wall_near.go).

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

// cornerWalk is what the corners measured of one part so far leave:
// the thinnest found, the pairs of triangles taken, and how many more
// corners may be measured.
type cornerWalk struct {
	least  float64
	seen   map[[2]int]bool
	budget int
}

// newCornerWalk starts the walk over the corners of a part whose
// points measured found.
func newCornerWalk(found []wallSample) *cornerWalk {
	cw := &cornerWalk{least: math.Inf(1), seen: map[[2]int]bool{}, budget: wallCornerMost}
	for _, f := range found {
		cw.least = min(cw.least, f.spot.Thickness)
	}
	return cw
}

// measureCorners measures the corners of the parts of triangles that
// the rays found join, wherever the walls there could be thinner than
// below or than the thinnest found, and adds them to found.
func (r *WallReport) measureCorners(w *clearancePart, below float64, cw *cornerWalk, found []wallSample) []wallSample {
	return r.walkCorners(w, below, cw, found, slices.Clone(found))
}

// walkCorners measures the corners of the pairs of triangles next
// joins, then those of the pairs the rays from them join, for
// wallCornerRounds rounds, and adds what they found to found.
func (r *WallReport) walkCorners(w *clearancePart, below float64, cw *cornerWalk, found, next []wallSample) []wallSample {
	for range wallCornerRounds {
		// The thinnest first, so that the thinnest found falls soon and
		// fewer corners are measured.
		slices.SortStableFunc(next, func(a, b wallSample) int { return cmp.Compare(a.spot.Thickness, b.spot.Thickness) })
		var more []wallSample
		for _, f := range next {
			p := [2]int{f.tri, f.hit}
			if f.hit < 0 || cw.seen[p] {
				continue
			}
			cw.seen[p] = true
			for _, c := range wallPairCorners(w.tree.Triangle(f.tri), w.tree.Triangle(f.hit)) {
				if c.t >= max(below, cw.least) || cw.budget == 0 {
					continue
				}
				cw.budget--
				sp, hit, end := w.thicknessFrom(f.tri, c.at, w.tree.Triangle(f.tri).Normal().WithMagnitude(-1))
				if end != rayOut {
					continue
				}
				r.Samples++
				sp.Group = w.g
				sp.sa = w.tree.Triangle(f.tri).Tag
				cw.least = min(cw.least, sp.Thickness)
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
	f, ok := newWallFrame(t)
	if !ok {
		return nil
	}
	return f.corners(h)
}

// wallFrame is a triangle laid flat in its plane, to be cut by the
// triangles it faces: its corner A, two directions along its plane,
// its inward normal and its corners in the plane.
type wallFrame struct {
	a, e1, e2, in geom.Vector
	poly          [][2]float64
}

// newWallFrame lays triangle t flat; false for a triangle of no area.
func newWallFrame(t *srf.STriangle) (wallFrame, bool) {
	n := t.Normal()
	if n.Magnitude() < 1e-12 {
		return wallFrame{}, false
	}
	n = n.WithMagnitude(1)
	f := wallFrame{a: t.A, e1: t.B.Minus(t.A).WithMagnitude(1), in: n.ScaledBy(-1)}
	f.e2 = n.Cross(f.e1)
	f.poly = [][2]float64{f.flat(t.A), f.flat(t.B), f.flat(t.C)}
	if area2(f.poly) < 1e-18 {
		return wallFrame{}, false
	}
	return f, true
}

// flat is point p in the frame's plane.
func (f *wallFrame) flat(p geom.Vector) [2]float64 {
	d := p.Minus(f.a)
	return [2]float64{d.Dot(f.e1), d.Dot(f.e2)}
}

// corners are wallPairCorners of the frame's triangle and h.
func (f *wallFrame) corners(h *srf.STriangle) []wallCorner {
	nh := h.Normal()
	if nh.Magnitude() < 1e-12 {
		return nil
	}
	nh = nh.WithMagnitude(1)
	cos := nh.Dot(f.in)
	if cos < 1e-9 {
		return nil
	}
	cut := [][2]float64{f.flat(h.A), f.flat(h.B), f.flat(h.C)}
	if area2(cut) < 0 {
		slices.Reverse(cut)
	}
	if area2(cut) < 1e-18 {
		return nil
	}
	poly := f.poly
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
		at := f.a.Plus(f.e1.ScaledBy(p[0])).Plus(f.e2.ScaledBy(p[1]))
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
