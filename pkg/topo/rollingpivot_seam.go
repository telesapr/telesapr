package topo

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A corner crossing a seam of the shared face: where a pocket's corner
// stands near the seam between two patches of a curved face, as a
// lathe's quarters, one fillet's foot on the shared face may lie on one
// patch and the other's on the next, the corner's foot crossing the
// seam between them as it swings. The seam runs from the rim, inside
// the fillets' band, out across the corner's side: it is cut where that
// side crosses it, its piece back to the rim goes, and each patch takes
// the corner's side up to the seam from its own fillet's foot.
//
// The point is found on the corner's own side, where the face's
// parameter across the seam reaches the patch's side, and the side and
// the patch of the corner it bounds are split there. Skinned in two at
// that share of the way round instead, with a section through the
// point, the corner had a break on the crease too: near the fillet's
// foot that lies nearer the vertex, where the corner leaves the crease
// tangent to the owner wall and its sections a little short of it lean
// past the wall's plane by a ten-thousandth of a millimetre, the
// triangles from that break crossed the wall's (7 of 90 corners of the
// dish's pocket turned onto its seam, fillets of one radius, refused
// as not closed).

// seamCrossing finds where the corner's side on the shared face crosses
// the seam from a's patch of that face to b's, and the seam's end on the
// rim.
func (pf *pivotFrame) seamCrossing() error {
	c := pf.c
	t := pf.ra.t
	s := t.Surface(c.fd)
	if s == nil {
		return filletErr(c.v, "the face the fillets share is not in the shell")
	}
	onRim := map[int]bool{}
	for _, rb := range []*rollingBall{pf.ra, pf.rb} {
		for _, v := range rb.verts {
			onRim[v] = true
		}
	}
	edge, end := t.seamBetween(c.fd, c.fdb, onRim, c.v)
	if edge < 0 {
		return filletErr(c.v, "the fillets' corner at the inside corner crosses a seam of the face they share "+
			"that doesn't reach the rim near it: make them smaller")
	}
	pts := t.Edges[edge].Points
	past, ok := seamParam(s, pts)
	if !ok {
		return filletErr(c.v, "the seam the fillets' corner crosses at the inside corner is not a side of the face's patch")
	}
	pieces := c.sideCurves(1 - c.creaseU)
	// Once across, looked for at a few points along each piece.
	at, crossings := -1, 0
	prev := past(pieces[0].Start())
	for i := range pieces {
		for j := 1; j <= 8; j++ {
			x := past(pieces[i].PointAt(float64(j) / 8))
			if (x < 0) != (prev < 0) {
				at = i
				crossings++
			}
			prev = x
		}
	}
	if crossings != 1 || !(past(pieces[0].Start()) < 0) {
		return filletErr(c.v, "the fillets' corner at the inside corner crosses a seam of the face they share "+
			"more than once: make them smaller")
	}
	sb := pieces[at]
	lo, hi := 0.0, 1.0
	for range 60 {
		m := (lo + hi) / 2
		if past(sb.PointAt(m)) < 0 {
			lo = m
		} else {
			hi = m
		}
	}
	c.seamPiece, c.seamAt = at, (lo+hi)/2
	c.seamX = sb.PointAt(c.seamAt)
	if d := distanceToPolyline(c.seamX, pts); d > math.Max(10*pf.ra.tol, edgeTol(pts)) {
		return filletErr(c.seamX, "the fillets' corner at the inside corner meets the seam of the face they share "+
			"%.3g mm off it", d)
	}
	c.seam, c.seamEnd = true, t.Vertices[end].P
	return nil
}

// splitAtSeam is the corner's side on the shared face, from a's foot to
// b's, as pieces up to the seam and on from it.
func (c *pivotCorner) splitAtSeam(pieces []srf.SBezier) (before, after []srf.SBezier) {
	i := c.seamPiece
	switch {
	case c.seamAt < seamEndTol:
		return pieces[:i], pieces[i:]
	case c.seamAt > 1-seamEndTol:
		return pieces[:i+1], pieces[i+1:]
	}
	before = append(before, pieces[:i]...)
	var bef, aft srf.SBezier
	pieces[i].SplitAt(c.seamAt, &bef, &aft)
	before = append(before, bef)
	after = append(append(after, aft), pieces[i+1:]...)
	return before, after
}

// seamEndTol is how near an end of a piece of the corner's side, in its
// parameter, the seam is taken to cross at that end.
const seamEndTol = 1e-9

// carryAcrossSeam cuts the seam where the corner's side on the shared
// face crosses it, drops its piece back to the rim from both patches, and
// gives each patch the corner's side up to the seam from its own
// fillet's foot.
func (c *pivotCorner) carryAcrossSeam(sh *srf.SShell, sD, sDb *srf.SSurface, pieces []srf.SBezier) error {
	if err := splitNearest(sh, c.seamX, c.seamEnd); err != nil {
		return filletErr(c.seamX, "the fillets' corner doesn't meet the seam it crosses at the inside corner: make them smaller")
	}
	for _, s := range []*srf.SSurface{sD, sDb} {
		var kept []srf.STrimBy
		for _, tb := range s.Trim {
			if tb.StartP.Equals(c.seamX) && tb.FinishP.Equals(c.seamEnd) || tb.StartP.Equals(c.seamEnd) && tb.FinishP.Equals(c.seamX) {
				continue
			}
			kept = append(kept, tb)
		}
		s.Trim = kept
		forget(s)
	}
	before, after := c.splitAtSeam(pieces)
	if err := c.carryShared(sh, sD, c.da, before); err != nil {
		return err
	}
	// b's side, from b's foot back to the seam.
	back := make([]srf.SBezier, 0, len(after))
	for _, sb := range slices.Backward(after) {
		sb.Reverse()
		back = append(back, sb)
	}
	return c.carryShared(sh, sDb, c.db, back)
}

// seamLoop is the loop of the corner's patch in row r, column b, split
// at the seam where its side on the shared face crosses it.
func (c *pivotCorner) seamLoop(r, b int, loop []srf.SBezier) []srf.SBezier {
	if !c.seam || b != c.seamPiece || r != (1-c.creaseU)*(len(c.all)-1) {
		return loop
	}
	var out []srf.SBezier
	for _, sb := range loop {
		out = append(out, splitAtPoints(sb, []geom.Vector{c.seamX})...)
	}
	return out
}
