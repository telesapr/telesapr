package topo

import (
	"cmp"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Pivot corners: two rolling-ball fillets that meet at a concave crease,
// an inside corner, as the rim of a pocket cut into a curved face turns
// at. There the ball doesn't stop where the two swept surfaces cross, as
// at a mitre (rollingmitre.go): rolling along one edge up to the crease,
// it swings round the crease's line, touching that line and the face
// both chains share, until it touches the other chain's creased face
// and rolls on along that edge. So each fillet ends at the ball's own
// section whose foot on its creased side is on the crease (pivotS), and
// between the two is a corner (rollingpivot_corner.go): a patch bounded
// by the two end sections, a curve on the shared face, and the piece of
// the crease between the two ends' feet. Where the shared face slopes
// differently each way the two feet lie at different depths on the
// crease: that piece stays an edge of the wall whose foot lies nearer
// the vertex, the other wall's edge with the corner. Chamfers turn such
// a corner the same way, their corner ruled. Where the two feet are one
// point, as round a pocket in a flat face, the two ends meet in a mitre
// instead (rollingmitre_inside.go).

// concaveAt reports whether the crease at end k of the open chain is
// concave: the face the chain ends at faces back along it.
func (rb *rollingBall) concaveAt(k int) bool {
	end := rb.ends[k]
	x, v := rb.pieces[0].Edge, rb.verts[0]
	if k == 1 {
		x, v = rb.pieces[len(rb.pieces)-1].Edge, rb.verts[len(rb.verts)-1]
	}
	beyond := edgeDirAt(rb.t, x, v).ScaledBy(-1)
	return end.n.Dot(beyond) < 0
}

// pivotAt reports whether end k of the open chain pivots.
func (rb *rollingBall) pivotAt(k int) bool {
	return !rb.closed && rb.ends[k] != nil && rb.ends[k].pivot
}

// creaseSide is the side of the chain whose face is creased at end k,
// the one whose ball is banned from the face across (rbEnd.ban), or -1.
func (rb *rollingBall) creaseSide(k int) int {
	for i := range 2 {
		if rb.ends[k].ban[i] >= 0 {
			return i
		}
	}
	return -1
}

// pivotS is the arc length of the ball's section at pivot end k whose
// foot on the creased side lies on the crease: in the plane of the face
// across it.
func (rb *rollingBall) pivotS(k int) (float64, error) {
	end := rb.ends[k]
	at := rb.t.Vertices[end.v].P
	i := rb.creaseSide(k)
	if i < 0 {
		return 0, filletErr(at, "the fillet's end at the inside corner has no crease")
	}
	// From half a radius in: the section through the vertex itself lies
	// in the face across where the crease is square to the edge.
	in := math.Min(rb.r/2, rb.length/4)
	s0 := in
	if k == 1 {
		s0 = rb.length - in
	}
	s, _, err := rb.planeRoot(func(x rbSection) float64 { return rb.endHeight(end, x.foot[i].p) }, s0, endDir(k), at)
	return s, err
}

// pivotCorner is the corner where end ka of blend a meets end kb of
// blend b at a concave crease.
type pivotCorner struct {
	a, b   *rbBlend
	ka, kb int
	// v is the vertex the crease ends at under both edges; qa and qb the
	// two fillets' feet on the crease, da and db on the shared face.
	v              geom.Vector
	qa, qb, da, db geom.Vector
	// fa, fb and fd are the creased faces of a and b and the shared face.
	fa, fb, fd int
	// creaseU is the side of the corner's patches along the crease, 0
	// or 1 in u; the shared face's is the other.
	creaseU int
	all     [][]srf.SSurface
}

// pivotCorners are the corners between the blends' pivot ends, each
// pair once.
func pivotCorners(bls []*rbBlend) ([]*pivotCorner, error) {
	var out []*pivotCorner
	for i, bl := range bls {
		if bl.rb.closed {
			continue
		}
		for k, end := range bl.rb.ends {
			if end == nil || !end.pivot {
				continue
			}
			j := slices.IndexFunc(bls, func(o *rbBlend) bool { return o.rb == end.mate })
			if j < 0 {
				return nil, filletErr(bl.rb.t.Vertices[end.v].P, "the fillet meeting this one at the inside corner is not being made")
			}
			if j < i || j == i && end.mateK < k {
				continue
			}
			c, err := newPivotCorner(bl, k, bls[j], end.mateK)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
	}
	return out, nil
}

// endSec is the blend's last section at end k.
func (bl *rbBlend) endSec(k int) rbSection {
	if k == 0 {
		return bl.secs[0]
	}
	return bl.secs[len(bl.secs)-1]
}

// sideRow is the row of the corner's patches along its side in u: 0 at
// u = 0, 1 at u = 1.
func (c *pivotCorner) sideRow(side int) []srf.SSurface {
	return c.all[side*(len(c.all)-1)]
}

// sideCurves are the pieces of the corner's side in u, from a's end to
// b's.
func (c *pivotCorner) sideCurves(side int) []srf.SBezier {
	var out []srf.SBezier
	for _, p := range c.sideRow(side) {
		edge := 3
		if side == 1 {
			edge = 1
		}
		out = append(out, p.EdgeCurve(edge))
	}
	return out
}

// cut makes room in the shell for the corner: the crease split at both
// feet, its piece up to the vertex gone from both walls and the piece
// between the feet from the wall whose foot lies further from it; and
// the shared face's edge carried round the corner from a's foot to b's.
func (c *pivotCorner) cut(sh *srf.SShell) error {
	t := c.a.rb.t
	near, far := c.qa, c.qb
	owner, other := c.fa, c.fb
	if c.qb.Minus(c.v).Magnitude() < c.qa.Minus(c.v).Magnitude() {
		near, far, owner, other = c.qb, c.qa, c.fb, c.fa
	}
	sOwner := sh.FindSurface(t.Faces[owner].Surface)
	sOther := sh.FindSurface(t.Faces[other].Surface)
	sD := sh.FindSurface(t.Faces[c.fd].Surface)
	if sOwner == nil || sOther == nil || sD == nil {
		return filletErr(c.v, "a face at the inside corner is not in the shell")
	}
	if err := c.splitCrease(sh, near, far); err != nil {
		return err
	}
	c.dropCrease(sh, sOwner, sOther, near, far)
	return c.carryShared(sh, sD)
}

// splitCrease splits the crease at the feet, near the vertex first, and
// between them at the corner's own breaks along it, so that its patches
// share the owner's edge piece by piece.
func (c *pivotCorner) splitCrease(sh *srf.SShell, near, far geom.Vector) error {
	if err := splitNearest(sh, near, c.v); err != nil {
		return filletErr(near, "the fillet's foot doesn't meet the crease at the inside corner: make it smaller")
	}
	if err := splitNearest(sh, far, near); err != nil {
		return filletErr(far, "the fillets' feet don't meet the crease at the inside corner: make them smaller")
	}
	var breaks []geom.Vector
	for _, sb := range c.sideCurves(c.creaseU)[1:] {
		breaks = append(breaks, sb.Start())
	}
	slices.SortFunc(breaks, func(x, y geom.Vector) int {
		return cmp.Compare(x.Minus(near).Magnitude(), y.Minus(near).Magnitude())
	})
	prev := near
	for _, p := range breaks {
		if err := splitNearest(sh, p, prev); err != nil {
			return filletErr(p, "the fillets' corner doesn't follow the crease at the inside corner")
		}
		prev = p
	}
	return nil
}

// dropCrease takes the crease's piece from the vertex to the nearer foot
// from both walls, and the pieces between the feet from the other wall:
// they stay the owner's edges, with the corner.
func (c *pivotCorner) dropCrease(sh *srf.SShell, sOwner, sOther *srf.SSurface, near, far geom.Vector) {
	onPiece := func(p geom.Vector) bool {
		return distanceToPolyline(p, []geom.Vector{near, far}) < 10*geom.LengthEps
	}
	keep := func(s *srf.SSurface, drop func(tb srf.STrimBy) bool) {
		var kept []srf.STrimBy
		for _, tb := range s.Trim {
			if !drop(tb) {
				kept = append(kept, tb)
			}
		}
		s.Trim = kept
		forget(s)
	}
	upper := func(tb srf.STrimBy) bool {
		return tb.StartP.Equals(c.v) && tb.FinishP.Equals(near) || tb.StartP.Equals(near) && tb.FinishP.Equals(c.v)
	}
	between := func(tb srf.STrimBy) bool {
		return onPiece(tb.StartP) && onPiece(tb.FinishP) && !tb.StartP.Equals(tb.FinishP)
	}
	keep(sOwner, upper)
	keep(sOther, func(tb srf.STrimBy) bool { return upper(tb) || between(tb) })
	for _, tb := range sOwner.Trim {
		if between(tb) {
			if cv := sh.FindCurve(tb.Curve); cv != nil {
				cv.SurfA, cv.SurfB = sOwner.H, core.HSSurface{}
			}
		}
	}
}

// carryShared adds to the shared face's loop the corner's side on it,
// from a's foot there to b's, the way the loop runs.
func (c *pivotCorner) carryShared(sh *srf.SShell, sD *srf.SSurface) error {
	fwd, found := true, false
	for _, tb := range sD.Trim {
		switch {
		case tb.FinishP.Equals(c.da):
			fwd, found = true, true
		case tb.StartP.Equals(c.da):
			fwd, found = false, true
		}
	}
	if !found {
		return filletErr(c.da, "the shared face's edge doesn't reach the fillet's end at the inside corner")
	}
	h := newCurveHandle(sh)
	pieces := c.sideCurves(1 - c.creaseU)
	if !fwd {
		slices.Reverse(pieces)
		for i := range pieces {
			pieces[i].Reverse()
		}
	}
	for _, sb := range pieces {
		var pts []geom.Vector
		sb.MakePwlIntoPoints(&pts, sh.Tol, 0)
		pts[0], pts[len(pts)-1] = sb.Start(), sb.Finish()
		nc := srf.SCurve{H: h, Exact: sb, IsExact: true, Deg: sb.Degree, SurfA: sD.H}
		h.V++
		for _, p := range pts {
			nc.Points = append(nc.Points, srf.SCurvePt{Vertex: true, P: p})
		}
		sD.Trim = append(sD.Trim, srf.STrimBy{Curve: nc.H, StartP: sb.Start(), FinishP: sb.Finish(), Start: 0, Finish: 1})
		sh.Curve = append(sh.Curve, nc)
	}
	forget(sD)
	return nil
}

// put adds the corner's patches, with the colour, origin and sheet of
// a's fillet.
func (c *pivotCorner) put(fa *FaceAdder, color core.RGBA, origin uint32) error {
	for _, row := range c.all {
		for b := range row {
			p := row[b]
			loop := patchLoop(&p)
			p.Color, p.Origin, p.Sheet = color, origin, origin
			if _, err := fa.Add(p, [][]srf.SBezier{loop}); err != nil {
				return filletErr(loop[0].Start(), "the fillets' corner at the inside corner can't be put in: %v", err)
			}
		}
	}
	return nil
}
