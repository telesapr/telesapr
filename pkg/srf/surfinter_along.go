package srf

import (
	"cmp"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// alongPiece is a stretch of a new exact intersection curve, from t0
// to t1 of it, that an operand's own exact curve runs along.
type alongPiece struct {
	t0, t1 float64
	edge   *SCurve
	// whole is set where the edge lies within the stretch end to end:
	// the piece is then the edge's own curve.
	whole     bool
	backwards bool
}

// alongCut is a piece of a new exact intersection curve: its curve,
// and the points it follows where it runs along part of an operand's
// edge (nil where it makes its own).
type alongCut struct {
	curve  SBezier
	points []SCurvePt
}

// piecesAlongEdges cuts sb, the exact curve where s of the first
// operand meets srfB of the second, where it runs along the operands'
// own curved edges bounding s or srfB, and gives the pieces in order:
// a piece covering an edge end to end is that edge's curve, whose
// points the new curve then follows (addExactIntersectionCurve's
// identical match), and one along part of an edge follows the edge's
// points between its ends. The trims of both operands need the same
// chords there. Turned copies of a triquetra meet each other along the
// edges of a third (roadmap K3): the union of two has an ellipse along
// which their cylinders meet, cut into two arcs, and the third's cap
// meets each cylinder in that whole ellipse, its cylinders in its
// halves; given chords of their own, the curves part from the edge's
// by up to the chord tolerance, and the faces' trims don't assemble.
//
// Only conics and higher are cut: a line along an edge meets the same
// split points as the edge, and follows it already.
//
// An edge bounding neither, but a face lying on the same cylinder or
// cone as s or srfB, is followed too: the curve where a flat face meets
// a patch of the other operand on its bore runs on past the face's
// patch along the next one's edge, where both operands' faces on the
// bore are cut at other angles round it (roundOf).
func (s *SSurface) piecesAlongEdges(sb *SBezier, srfB *SSurface, agnstA, agnstB *SShell, into *curveSink) []alongCut {
	plain := []alongCut{{curve: *sb}}
	if sb.Degree < 2 || into.shell == nil {
		return plain
	}
	if m := into.findExact(sb); m.curve != nil {
		return plain
	}
	// The curve lies within its control points' box.
	lo, hi := sb.Ctrl[0], sb.Ctrl[0]
	for _, p := range sb.Ctrl[1 : sb.Degree+1] {
		lo = geom.Vector{X: min(lo.X, p.X), Y: min(lo.Y, p.Y), Z: min(lo.Z, p.Z)}
		hi = geom.Vector{X: max(hi.X, p.X), Y: max(hi.Y, p.Y), Z: max(hi.Z, p.Z)}
	}
	var pieces []alongPiece
	for i := range into.shell.Curve {
		c := &into.shell.Curve[i]
		if !c.IsExact || c.Exact.Degree < 2 || len(c.Points) < 2 {
			continue
		}
		own := c.edgeOf(s, srfB)
		if !own && !c.ofOperand() {
			continue
		}
		if !c.boxMeets(lo, hi) {
			continue
		}
		if pc, ok := sb.runsAlong(c); ok && (own || c.roundOf(s, srfB, agnstA, agnstB)) {
			pieces = append(pieces, pc)
		}
	}
	if len(pieces) == 0 {
		return plain
	}
	pieces = oneAlongEach(pieces)
	var out []alongCut
	t, at := 0.0, sb.Start()
	sub := func(t0, t1 float64, p0, p1 geom.Vector) SBezier {
		piece := sb.Sub(t0, t1)
		piece.Ctrl[0], piece.Ctrl[piece.Degree] = p0, p1
		return piece
	}
	for _, pc := range pieces {
		p0, p1 := sb.PointAt(pc.t0), sb.PointAt(pc.t1)
		if pc.whole {
			c := pc.edge.Exact
			if pc.backwards {
				c.Reverse()
			}
			p0, p1 = c.Start(), c.Finish()
			if !at.Equals(p0) {
				out = append(out, alongCut{curve: sub(t, pc.t0, at, p0)})
			}
			out = append(out, alongCut{curve: c})
			t, at = pc.t1, p1
			continue
		}
		if pc.t0 <= 1e-9 {
			p0 = sb.Start()
		}
		if pc.t1 >= 1-1e-9 {
			p1 = sb.Finish()
		}
		p0, p1 = pc.edge.endAt(p0), pc.edge.endAt(p1)
		if !at.Equals(p0) {
			out = append(out, alongCut{curve: sub(t, pc.t0, at, p0)})
		}
		out = append(out, alongCut{curve: sub(pc.t0, pc.t1, p0, p1), points: pc.edge.pointsBetween(p0, p1)})
		t, at = pc.t1, p1
	}
	if !at.Equals(sb.Finish()) {
		out = append(out, alongCut{curve: sub(t, 1, at, sb.Finish())})
	}
	return out
}

// oneAlongEach keeps one of the edges running along each stretch, in
// the order of the stretches. Where both operands have an edge along
// one stretch, as faces lying face to face do round their common
// outline, the first operand's is followed, as the first operand's
// edges keep their points through a Boolean; where an operand has two,
// as where three of its faces meet along one curve, the first. Both
// have the same points where fuseCoincidentEdges or an earlier Boolean
// gave them those.
func oneAlongEach(pieces []alongPiece) []alongPiece {
	ofA := func(pc alongPiece) bool {
		return pc.edge.Source == CurveSourceOrigA || pc.edge.Source == CurveSourceSplitA
	}
	slices.SortStableFunc(pieces, func(x, y alongPiece) int {
		switch {
		case ofA(x) && !ofA(y):
			return -1
		case ofA(y) && !ofA(x):
			return 1
		}
		return 0
	})
	var kept []alongPiece
	for _, pc := range pieces {
		if !slices.ContainsFunc(kept, func(k alongPiece) bool { return k.t0 < pc.t1-1e-9 && pc.t0 < k.t1-1e-9 }) {
			kept = append(kept, pc)
		}
	}
	slices.SortFunc(kept, func(x, y alongPiece) int { return cmp.Compare(x.t0, y.t0) })
	return kept
}

// edgeOf tells whether c is an operand's own curve bounding s, of the
// first operand, or srfB, of the second.
func (c *SCurve) edgeOf(s, srfB *SSurface) bool {
	switch c.Source {
	case CurveSourceOrigA, CurveSourceSplitA:
		return c.SurfA == s.H || c.SurfB == s.H
	case CurveSourceOrigB, CurveSourceSplitB:
		return c.SurfA == srfB.H || c.SurfB == srfB.H
	}
	return false
}

// ofOperand tells whether c is an operand's own curve.
func (c *SCurve) ofOperand() bool {
	switch c.Source {
	case CurveSourceOrigA, CurveSourceSplitA, CurveSourceOrigB, CurveSourceSplitB:
		return true
	}
	return false
}

// roundOf tells whether c, an operand's own curve, bounds a face lying
// on the same cylinder or cone as s or srfB (sameRoundSurface).
func (c *SCurve) roundOf(s, srfB *SSurface, agnstA, agnstB *SShell) bool {
	if agnstA == nil || agnstB == nil {
		return false
	}
	sh := agnstA
	if c.Source == CurveSourceOrigB || c.Source == CurveSourceSplitB {
		sh = agnstB
	}
	for _, f := range []*SSurface{sh.FindSurface(c.SurfA), sh.FindSurface(c.SurfB)} {
		if f != nil && (sameRoundSurface(f, s) || sameRoundSurface(f, srfB)) {
			return true
		}
	}
	return false
}

// boxMeets tells whether some of c's points lie within the box from lo
// to hi.
func (c *SCurve) boxMeets(lo, hi geom.Vector) bool {
	return slices.ContainsFunc(c.Points, func(p SCurvePt) bool { return !p.P.OutsideAndNotOn(hi, lo) })
}

// endAt is p, or the end of c's points it is.
func (c *SCurve) endAt(p geom.Vector) geom.Vector {
	for _, q := range c.Points {
		if q.P.Equals(p) {
			return q.P
		}
	}
	return p
}

// pointsBetween are c's points from p0 to p1, both on c, in that
// order: p0, the points of c between them, p1.
func (c *SCurve) pointsBetween(p0, p1 geom.Vector) []SCurvePt {
	t0, _ := c.Exact.ClosestPointTo(p0, true)
	t1, _ := c.Exact.ClosestPointTo(p1, true)
	lo, hi := min(t0, t1), max(t0, t1)
	var mid []SCurvePt
	for _, q := range c.Points {
		if q.P.Equals(p0) || q.P.Equals(p1) {
			continue
		}
		if t, _ := c.Exact.ClosestPointTo(q.P, true); t > lo && t < hi {
			mid = append(mid, q)
		}
	}
	if t0 > t1 {
		slices.Reverse(mid)
	}
	out := make([]SCurvePt, 0, len(mid)+2)
	out = append(out, SCurvePt{P: p0, Vertex: true})
	out = append(out, mid...)
	return append(out, SCurvePt{P: p1, Vertex: true})
}

// runsAlong tells whether sb and the edge c run along one another over
// a stretch of some length, and where on sb. The stretch's ends are
// ends of sb lying on c or ends of c lying on sb.
func (sb *SBezier) runsAlong(c *SCurve) (alongPiece, bool) {
	ce := &c.Exact
	onSb := func(p geom.Vector) (float64, bool) {
		t, _ := sb.ClosestPointTo(p, true)
		if t < -1e-9 || t > 1+1e-9 {
			return 0, false
		}
		return max(0, min(1, t)), sb.PointAt(t).Equals(p)
	}
	onC := func(p geom.Vector) bool {
		t, _ := ce.ClosestPointTo(p, true)
		return t >= -1e-9 && t <= 1+1e-9 && ce.PointAt(t).Equals(p)
	}
	var ts []float64
	tc0, ok0 := onSb(ce.Start())
	tc1, ok1 := onSb(ce.Finish())
	if ok0 {
		ts = append(ts, tc0)
	}
	if ok1 {
		ts = append(ts, tc1)
	}
	if onC(sb.Start()) {
		ts = append(ts, 0)
	}
	if onC(sb.Finish()) {
		ts = append(ts, 1)
	}
	if len(ts) < 2 {
		return alongPiece{}, false
	}
	t0, t1 := slices.Min(ts), slices.Max(ts)
	if sb.PointAt(t0).Equals(sb.PointAt(t1)) {
		return alongPiece{}, false
	}
	// Two curves can share both ends and part between them.
	if !onC(sb.PointAt((t0+t1)/2)) || !onC(sb.PointAt(0.75*t0+0.25*t1)) || !onC(sb.PointAt(0.25*t0+0.75*t1)) {
		return alongPiece{}, false
	}
	pc := alongPiece{t0: t0, t1: t1, edge: c, whole: ok0 && ok1}
	if pc.whole {
		if t0 <= 1e-9 && t1 >= 1-1e-9 {
			// The whole of sb: findExact's to match, if it is the same;
			// a curve of other control points is followed as a part.
			pc.whole = false
		}
		pc.backwards = tc0 > tc1
	}
	return pc, true
}
