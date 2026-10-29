package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A sweep with a corner radius (ValB) rounds the corners of its path
// on the path itself: at each corner the two pieces are cut back to
// where a circle of the radius, tangent to both, touches them, and that
// circle's arc joins them. The path is then pieces that meet tangent,
// which the kernel sweeps exactly, an arc's piece of the solid the
// profile turned about the bend's axis (srf.MakeFromSweepOf); no
// surface is intersected, so it works for any profile, beside arcs as
// beside lines, and at corners too sharp for the kernel to join beside
// an arc (srf.CheckSweepCorners). The circle is
// found where the pieces' offsets by the radius toward the inside of
// the turn cross: a line's offset is a line, an arc's a circle about
// its centre.

// errCornerBesideArc is a corner next to an arc on a path with no
// corner radius whose pieces the kernel can't join: a corner radius
// rounds it.
var errCornerBesideArc = errors.New("a corner next to an arc needs a corner radius")

// cornerJointError is the path's error for err, srf.CheckSweepCorners'
// for a corner beside an arc.
func cornerJointError(err error) error {
	ce, ok := errors.AsType[*srf.CornerError](err)
	if !ok {
		return pathErr(nil, "%v", err)
	}
	x := ce.At
	why := "its pieces' sides don't meet"
	switch {
	case errors.Is(err, srf.ErrCornerOutOfPlane):
		return pathErr(&x, "the path has a corner at (%g, %g, %g) out of the plane of the arc beside it: "+
			"a sweep joins only a corner in an arc's plane", x.X, x.Y, x.Z)
	case errors.Is(err, srf.ErrCornerTooLong):
		why = "a piece beside it is too short for its joint"
	case errors.Is(err, srf.ErrCornerTooWide):
		why = "its joint goes too far round the arc"
	}
	return &pathError{at: &x, kind: errCornerBesideArc, msg: fmt.Sprintf(
		"the path's corner at (%g, %g, %g) beside an arc can't be joined: %s; "+
			"give the sweep a corner radius to round it", x.X, x.Y, x.Z, why)}
}

// roundCorners replaces each corner of chain by an arc of radius r
// tangent to the pieces either side, cutting them back to it.
func (sk *Sketch) roundCorners(chain []pathPiece, r float64) ([]pathPiece, error) {
	out := []pathPiece{chain[0]}
	for k := 1; k < len(chain); k++ {
		a, b := out[len(out)-1], chain[k]
		if a.tangentAt(true).Dot(b.tangentAt(false)) >= sweepTangentCos {
			out = append(out, b)
			continue
		}
		fillet, a2, b2, err := sk.roundCorner(a, b, r)
		if err != nil {
			return nil, err
		}
		out[len(out)-1] = a2
		out = append(out, fillet, b2)
	}
	return out, nil
}

// roundCorner is the arc of radius r joining a, which ends at the
// corner, to b, which starts there, and the two cut back to it.
func (sk *Sketch) roundCorner(a, b pathPiece, r float64) (fillet, a2, b2 pathPiece, err error) {
	p := b.start
	d1, d2 := a.tangentAt(true), b.tangentAt(false)
	n := d1.Cross(d2)
	if n.Magnitude() < 1e-9 {
		return fillet, a, b, pathErr(&p, "the path turns back on itself at (%g, %g, %g)", p.X, p.Y, p.Z)
	}
	n = n.WithMagnitude(1)
	for _, q := range []pathPiece{a, b} {
		if q.arc && math.Abs(q.axis.Dot(n)) < 1-1e-9 {
			return fillet, a, b, pathErr(&p, "the corner at (%g, %g, %g) is out of the plane of arc %s next to it: "+
				"only a corner in an arc's plane can be rounded", p.X, p.Y, p.Z, sk.entityName(q.h))
		}
	}
	// In the corner's plane, from the corner: x along a's way in, y to
	// the inside of the turn.
	ex, ey := d1, n.Cross(d1)
	to2 := func(v geom.Vector) [2]float64 { w := v.Minus(p); return [2]float64{w.Dot(ex), w.Dot(ey)} }
	from2 := func(u [2]float64) geom.Vector { return p.Plus(ex.ScaledBy(u[0])).Plus(ey.ScaledBy(u[1])) }

	// Each piece's offset by r toward the inside: a line, as a point on
	// it and its direction, or a circle, as its centre and radius.
	type offset struct {
		line        bool
		at, dir     [2]float64
		center      [2]float64
		radius      float64
		pieceRadius float64
	}
	offsetOf := func(q pathPiece, d geom.Vector) offset {
		in := n.Cross(d)
		if !q.arc {
			dd := to2(p.Plus(d))
			o := to2(p.Plus(in.ScaledBy(r)))
			return offset{line: true, at: o, dir: dd}
		}
		c := to2(q.center)
		ra := q.start.Minus(q.center).Magnitude()
		// The inside of the turn faces the arc's centre: the circle is
		// inside the arc's, else outside it.
		rr := ra + r
		if q.center.Minus(p).Dot(in) > 0 {
			rr = ra - r
		}
		return offset{center: c, radius: rr, pieceRadius: ra}
	}
	oa, ob := offsetOf(a, d1), offsetOf(b, d2)
	if !oa.line && oa.radius <= 0 || !ob.line && ob.radius <= 0 {
		return fillet, a, b, pathErr(&p, "the corner radius %g is larger than the arc at (%g, %g, %g)", r, p.X, p.Y, p.Z)
	}
	cs := crossOffsets(oa.line, oa.at, oa.dir, oa.center, oa.radius, ob.line, ob.at, ob.dir, ob.center, ob.radius)
	// The centre nearest the corner, on the inside of both.
	best, found := [2]float64{}, false
	for _, c := range cs {
		if c[1] <= 0 {
			continue
		}
		if !found || c[0]*c[0]+c[1]*c[1] < best[0]*best[0]+best[1]*best[1] {
			best, found = c, true
		}
	}
	if !found {
		return fillet, a, b, pathErr(&p, "no circle of radius %g fits the corner at (%g, %g, %g)", r, p.X, p.Y, p.Z)
	}
	center := from2(best)
	touch := func(q pathPiece, o offset) geom.Vector {
		if q.arc {
			return q.center.Plus(center.Minus(q.center).WithMagnitude(o.pieceRadius))
		}
		d := q.end.Minus(q.start).WithMagnitude(1)
		return q.start.Plus(d.ScaledBy(center.Minus(q.start).Dot(d)))
	}
	ta, tb := touch(a, oa), touch(b, ob)
	if a2, err = sk.cutPiece(a, ta, true); err != nil {
		return fillet, a, b, err
	}
	if b2, err = sk.cutPiece(b, tb, false); err != nil {
		return fillet, a, b, err
	}
	ra, rb := ta.Minus(center), tb.Minus(center)
	angle := math.Atan2(ra.Cross(rb).Dot(n), ra.Dot(rb))
	if angle < 0 {
		angle += 2 * math.Pi
	}
	fillet = pathPiece{start: ta, end: tb, arc: true, center: center, axis: n, angle: angle, rounded: true}
	return fillet, a2, b2, nil
}

// cutPiece cuts q back to t, which must lie on it: its end to t with
// atEnd, else its start. A piece left with no length means the radius
// is too large for it.
func (sk *Sketch) cutPiece(q pathPiece, t geom.Vector, atEnd bool) (pathPiece, error) {
	tooLarge := func() error {
		return pathErr(&t, "the corner radius is too large for %s: the rounded corners use all of it", sk.entityName(q.h))
	}
	if !q.arc {
		d := q.end.Minus(q.start)
		s := t.Minus(q.start).Dot(d) / d.Dot(d)
		if atEnd && s <= sweepTol/d.Magnitude() || !atEnd && s >= 1-sweepTol/d.Magnitude() || s < -1e-9 || s > 1+1e-9 {
			return q, tooLarge()
		}
		if atEnd {
			q.end = t
		} else {
			q.start = t
		}
		q.curves = nil
		return q, nil
	}
	// The angle along the arc from its start to t.
	r0, rt := q.start.Minus(q.center), t.Minus(q.center)
	phi := math.Atan2(r0.Cross(rt).Dot(q.axis), r0.Dot(rt))
	if phi < -1e-9 {
		phi += 2 * math.Pi
	}
	ra := r0.Magnitude()
	if phi > q.angle+1e-9 || atEnd && phi*ra <= sweepTol || !atEnd && (q.angle-phi)*ra <= sweepTol {
		return q, tooLarge()
	}
	if atEnd {
		q.end, q.angle = t, phi
	} else {
		q.start, q.angle = t, q.angle-phi
	}
	q.curves = nil
	return q, nil
}

// crossOffsets are where two offsets cross, each a line through at
// along dir or a circle about center of radius.
func crossOffsets(aLine bool, aAt, aDir, aC [2]float64, aR float64, bLine bool, bAt, bDir, bC [2]float64, bR float64) [][2]float64 {
	switch {
	case aLine && bLine:
		// aAt + s aDir = bAt + t bDir.
		den := aDir[0]*bDir[1] - aDir[1]*bDir[0]
		if math.Abs(den) < 1e-15 {
			return nil
		}
		s := ((bAt[0]-aAt[0])*bDir[1] - (bAt[1]-aAt[1])*bDir[0]) / den
		return [][2]float64{{aAt[0] + s*aDir[0], aAt[1] + s*aDir[1]}}
	case aLine:
		return lineCircle(aAt, aDir, bC, bR)
	case bLine:
		return lineCircle(bAt, bDir, aC, aR)
	}
	return circleCircle(aC, aR, bC, bR)
}

// lineCircle are where the line through at along dir meets the circle.
func lineCircle(at, dir, c [2]float64, r float64) [][2]float64 {
	fx, fy := at[0]-c[0], at[1]-c[1]
	qa := dir[0]*dir[0] + dir[1]*dir[1]
	qb := 2 * (fx*dir[0] + fy*dir[1])
	qc := fx*fx + fy*fy - r*r
	disc := qb*qb - 4*qa*qc
	if disc < 0 {
		return nil
	}
	sq := math.Sqrt(disc)
	var out [][2]float64
	for _, s := range []float64{(-qb - sq) / (2 * qa), (-qb + sq) / (2 * qa)} {
		out = append(out, [2]float64{at[0] + s*dir[0], at[1] + s*dir[1]})
	}
	return out
}

// circleCircle are where two circles meet.
func circleCircle(c1 [2]float64, r1 float64, c2 [2]float64, r2 float64) [][2]float64 {
	dx, dy := c2[0]-c1[0], c2[1]-c1[1]
	d := math.Hypot(dx, dy)
	if d == 0 || d > r1+r2 || d < math.Abs(r1-r2) {
		return nil
	}
	along := (r1*r1 - r2*r2 + d*d) / (2 * d)
	h := math.Sqrt(math.Max(0, r1*r1-along*along))
	mx, my := c1[0]+along*dx/d, c1[1]+along*dy/d
	return [][2]float64{{mx - h*dy/d, my + h*dx/d}, {mx + h*dy/d, my - h*dx/d}}
}
