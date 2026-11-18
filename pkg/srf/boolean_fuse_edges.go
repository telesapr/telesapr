package srf

import (
	"cmp"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// fuseCoincidentEdges gives b's curved exact edges, where they run
// along a's, a's points over that stretch, in a copy of b, and returns
// b if no edge of it does. Two faces lying face to face, turned copies'
// caps meeting on the plane through their axis (roadmap K3), have the
// same outline in both operands, each chorded in the Boolean that made
// it, at other points; the trims of the faces either side then don't
// meet, and the curves where the other operand's faces cross that
// outline can follow only one of them (piecesAlongEdges). The points of
// b's curves that its trims end at, or other curves of b do, stay (the
// Boolean splits a's curve there too, as at every vertex of b lying on
// it), and the ends of a's stretch are added: b keeps its geometry,
// and its two faces at each curve share the curve's new points.
func fuseCoincidentEdges(a, b *SShell) *SShell {
	type edge struct {
		i      int
		lo, hi geom.Vector
	}
	curved := func(sh *SShell) []edge {
		var es []edge
		for i := range sh.Curve {
			c := &sh.Curve[i]
			if !c.IsExact || c.Exact.Degree < 2 || len(c.Points) < 2 {
				continue
			}
			lo, hi := c.Points[0].P, c.Points[0].P
			for _, p := range c.Points[1:] {
				lo = geom.Vector{X: min(lo.X, p.P.X), Y: min(lo.Y, p.P.Y), Z: min(lo.Z, p.P.Z)}
				hi = geom.Vector{X: max(hi.X, p.P.X), Y: max(hi.Y, p.P.Y), Z: max(hi.Z, p.P.Z)}
			}
			es = append(es, edge{i, lo, hi})
		}
		return es
	}
	ea, eb := curved(a), curved(b)
	if len(ea) == 0 || len(eb) == 0 {
		return b
	}
	var out *SShell
	var ends []geom.Vector
	for _, e := range eb {
		var along []alongPiece
		cb := &b.Curve[e.i]
		for _, f := range ea {
			if e.hi.BoundingBoxesDisjoint(e.hi, e.lo, f.hi, f.lo) {
				continue
			}
			ca := &a.Curve[f.i]
			if pc, ok := cb.Exact.runsAlong(ca); ok {
				along = append(along, pc)
			}
		}
		if len(along) == 0 {
			continue
		}
		if ends == nil {
			ends = vertexPoints(b)
		}
		pts, ok := followAlong(cb, along, ends)
		if !ok || samePoints(pts, cb.Points) {
			// Chorded alike already, as the outline a feature was drawn
			// on mostly is: the curve keeps its own points to the bit.
			continue
		}
		if out == nil {
			c := b.Clone()
			out = &c
		}
		out.Curve[e.i].Points = pts
	}
	if out == nil {
		return b
	}
	return out
}

// vertexPoints are the points sh's trims start and finish at, and its
// curves' ends.
func vertexPoints(sh *SShell) []geom.Vector {
	var ends []geom.Vector
	for i := range sh.Surface {
		for _, tr := range sh.Surface[i].Trim {
			ends = append(ends, tr.StartP, tr.FinishP)
		}
	}
	for i := range sh.Curve {
		if pts := sh.Curve[i].Points; len(pts) > 0 {
			ends = append(ends, pts[0].P, pts[len(pts)-1].P)
		}
	}
	return ends
}

// followAlong is c's points with each stretch of along, the edges of
// the other operand running along it, taken over from that edge: c's
// points within a stretch are dropped but for those in ends, and the
// edge's points between the stretch's ends put in their place. It
// reports false if the stretches overlap.
func followAlong(c *SCurve, along []alongPiece, ends []geom.Vector) ([]SCurvePt, bool) {
	slices.SortFunc(along, func(x, y alongPiece) int { return cmp.Compare(x.t0, y.t0) })
	for i := 1; i < len(along); i++ {
		if along[i].t0 < along[i-1].t1-1e-9 {
			return nil, false
		}
	}
	type at struct {
		t  float64
		pt SCurvePt
	}
	var pts []at
	tOf := func(p geom.Vector) float64 {
		t, _ := c.Exact.ClosestPointTo(p, true)
		return t
	}
	first, last := c.Points[0], c.Points[len(c.Points)-1]
	inside := func(t float64) bool {
		return slices.ContainsFunc(along, func(pc alongPiece) bool { return t > pc.t0+1e-9 && t < pc.t1-1e-9 })
	}
	for k, q := range c.Points {
		if k == 0 || k == len(c.Points)-1 {
			continue
		}
		t := tOf(q.P)
		if inside(t) && !slices.ContainsFunc(ends, q.P.Equals) {
			continue
		}
		pts = append(pts, at{t, q})
	}
	for _, pc := range along {
		p0, p1 := c.Exact.PointAt(pc.t0), c.Exact.PointAt(pc.t1)
		if pc.t0 <= 1e-9 {
			p0 = first.P
		}
		if pc.t1 >= 1-1e-9 {
			p1 = last.P
		}
		p0, p1 = pc.edge.endAt(p0), pc.edge.endAt(p1)
		for k, q := range pc.edge.pointsBetween(p0, p1) {
			t := tOf(q.P)
			if t <= 1e-9 || t >= 1-1e-9 {
				continue
			}
			if k == 0 || q.P.Equals(p1) {
				q.Vertex = true
			}
			pts = append(pts, at{t, q})
		}
	}
	slices.SortStableFunc(pts, func(x, y at) int { return cmp.Compare(x.t, y.t) })
	out := make([]SCurvePt, 0, len(pts)+2)
	out = append(out, first)
	for _, p := range pts {
		if p.pt.P.Equals(out[len(out)-1].P) {
			continue
		}
		out = append(out, p.pt)
	}
	if out[len(out)-1].P.Equals(last.P) {
		out = out[:len(out)-1]
	}
	return append(out, last), true
}

// samePoints tells whether p and q are the same points, each within
// LengthEps.
func samePoints(p, q []SCurvePt) bool {
	return slices.EqualFunc(p, q, func(x, y SCurvePt) bool { return x.P.Equals(y.P) })
}
