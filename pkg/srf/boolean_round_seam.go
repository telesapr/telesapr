package srf

import (
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
)

// roundSeam is a straight trim, from a to b, of a patch of the other
// operand lying on the same cylinder or cone as a surface a curve
// bounds: the seam between two of its patches there, or the end of one.
type roundSeam struct{ a, b geom.Vector }

// roundSeamsUnder are the straight trims of the patches of agnst lying
// on the cylinder or cone of srfA or srfB, the surfaces the exact curve
// c bounds, near c: where c, lying on that surface, crosses one, it
// passes from one of agnst's patches there to the next.
//
// Two operands' faces on one cylinder or cone cut into patches at other
// angles round the axis (a cutter swept through its own breaks, lying on
// a bore lathed in quarters) meet nowhere a ray cast along c's chords
// sees: the chords dip inside the surface. Unsplit, the edge where a
// face of the first meets that surface (a ring's flat end along the
// bore) kept its points, while the curve where that face meets each of
// the second's patches ran along part of it from a seam to a seam, with
// a point of its own there, and neither face's trim assembled (roadmap
// K3, 2026-11-18).
func roundSeamsUnder(c *SCurve, agnst *SShell, srfA, srfB *SSurface) []roundSeam {
	if !c.IsExact || c.Exact.Degree < 2 || agnst == nil {
		return nil
	}
	var out []roundSeam
	var cmax, cmin geom.Vector
	c.AxisAlignedBounding(&cmax, &cmin)
	grow := geom.Vector{X: 1, Y: 1, Z: 1}.ScaledBy(tolOf(srfA).ChordTolMm() + geom.LengthEps)
	cmax, cmin = cmax.Plus(grow), cmin.Minus(grow)
	for _, own := range []*SSurface{srfA, srfB} {
		if own == nil || (own.DegM < 2 && own.DegN < 2) {
			continue
		}
		f, ok := RoundFaceOf(own)
		if !ok || !roundAbout(&c.Exact, f) {
			continue
		}
		for i := range agnst.Surface {
			ss := &agnst.Surface[i]
			if ss.DegM < 2 && ss.DegN < 2 {
				continue
			}
			var smax, smin geom.Vector
			ss.AxisAlignedBounding(&smax, &smin)
			if cmax.BoundingBoxesDisjoint(cmax, cmin, smax, smin) || !sameRoundSurface(own, ss) {
				continue
			}
			for j := range ss.Trim {
				tc := agnst.FindCurve(ss.Trim[j].Curve)
				if tc == nil || !tc.IsExact || tc.Exact.Degree != 1 {
					continue
				}
				s := roundSeam{tc.Exact.Start(), tc.Exact.Finish()}
				if !slices.ContainsFunc(out, func(o roundSeam) bool {
					return (o.a.Equals(s.a) && o.b.Equals(s.b)) || (o.a.Equals(s.b) && o.b.Equals(s.a))
				}) {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// roundAbout reports whether sb is an arc of a circle about f's axis:
// its control points in one plane square to the axis. Only such a
// curve, as where a flat face meets a cylinder or cone square to it, is
// split at the other operand's seams; a thread's helices lying on its
// blank's cylinder cross the blank's seams too, and are left as they
// were: split there, their trims moved in the last bits, which the
// thread goldens record, with nothing to mend.
func roundAbout(sb *SBezier, f RoundFace) bool {
	if sb.Degree != 2 {
		return false
	}
	h := sb.Ctrl[0].Dot(f.Axis)
	for _, p := range sb.Ctrl[1:3] {
		if math.Abs(p.Dot(f.Axis)-h) > geom.LengthEps {
			return false
		}
	}
	return true
}

// roundSeamCrossings appends to il the points where c, from its point a
// to b, crosses one of seams: on c's exact curve and on the seam, inside
// the segment and not at its ends. The chord passes the seam within its
// sag, which tol bounds, and the point is then found on both exactly.
func roundSeamCrossings(il []SInter, c *SCurve, seams []roundSeam, a, b geom.Vector, tol float64) []SInter {
	tol = max(tol, b.Minus(a).Magnitude()/4)
	for _, s := range seams {
		q, ok := segmentsMeet(a, b, s.a, s.b, tol)
		if !ok {
			continue
		}
		d := s.b.Minus(s.a)
		for range 8 {
			t, _ := c.Exact.ClosestPointTo(q, false)
			q = c.Exact.PointAt(t)
			l := q.ClosestPointOnLine(s.a, d)
			if l.Minus(q).Magnitude() < geom.LengthEps/10 {
				break
			}
			q = l
		}
		ab := b.Minus(a)
		if q.DistanceToLine(s.a, d) > geom.LengthEps || !q.OnLineSegment(s.a, s.b, geom.LengthEps) {
			continue
		}
		if f := q.Minus(a).Dot(ab); f <= 0 || f >= ab.Dot(ab) {
			continue
		}
		if q.EqualsTol(a, splitEndTol) || q.EqualsTol(b, splitEndTol) ||
			slices.ContainsFunc(il, func(o SInter) bool { return o.P.EqualsTol(q, splitEndTol) }) {
			continue
		}
		il = append(il, SInter{P: q})
	}
	return il
}

// segmentsMeet is the middle of the shortest join between the segments
// a-b and c-d, where it is no longer than tol and lies inside a-b and
// within c-d.
func segmentsMeet(a, b, c, d geom.Vector, tol float64) (geom.Vector, bool) {
	u, v, w := b.Minus(a), d.Minus(c), a.Minus(c)
	uu, uv, vv := u.Dot(u), u.Dot(v), v.Dot(v)
	uw, vw := u.Dot(w), v.Dot(w)
	den := uu*vv - uv*uv
	if uu <= 0 || vv <= 0 || den <= 1e-12*uu*vv {
		return geom.Vector{}, false
	}
	s := (uv*vw - vv*uw) / den
	t := (uu*vw - uv*uw) / den
	const eps = 1e-9
	if s <= eps || s >= 1-eps || t < -eps || t > 1+eps {
		return geom.Vector{}, false
	}
	p, q := a.Plus(u.ScaledBy(s)), c.Plus(v.ScaledBy(t))
	if p.Minus(q).Magnitude() > tol || math.IsNaN(p.X) {
		return geom.Vector{}, false
	}
	return p.Plus(q).ScaledBy(0.5), true
}

// piecesOnRoundPatch are the pieces of sb, a curve lying on the
// cylinder or cone f of patch, that lie within the patch's turn about
// f's axis: sb cut where it crosses the patch's straight sides. A
// plane's outline lying on a face of the other operand there (a ring's
// flat end along a bore, the other operand's patch on the bore cut at
// other angles) is the curve where the plane meets the patch only over
// the patch: whole, it ran on past the patch's sides, where the patch's
// trim took it back onto the patch's edges, and the trim did not close
// (roadmap K3, 2026-11-18). A curve within the patch, as where both are
// cut at the same angles, is given back whole, and one beside it none.
func piecesOnRoundPatch(sb SBezier, patch *SSurface, f RoundFace) []SBezier {
	e1 := f.Axis.NormalDir(0).WithMagnitude(1)
	e2 := f.Axis.Cross(e1).WithMagnitude(1)
	angle := func(p geom.Vector) float64 {
		q := p.Minus(f.Point)
		return math.Atan2(q.Dot(e2), q.Dot(e1))
	}
	turn := func(a, b float64) float64 {
		d := math.Mod(b-a, 2*math.Pi)
		if d < 0 {
			d += 2 * math.Pi
		}
		return d
	}
	var p0, p1 geom.Vector
	if patch.DegN == 1 {
		p0, p1 = patch.PointAt(0, 0.5), patch.PointAt(1, 0.5)
	} else {
		p0, p1 = patch.PointAt(0.5, 0), patch.PointAt(0.5, 1)
	}
	a0, a1, am := angle(p0), angle(p1), angle(patch.PointAt(0.5, 0.5))
	if turn(a0, am) > turn(a0, a1) {
		a0, a1 = a1, a0
	}
	span := turn(a0, a1)
	if span < 1e-9 {
		return []SBezier{sb}
	}
	// Inside the turn by a margin below LengthEps at the curve's radius,
	// so that a curve ending on a side is not cut there.
	inside := func(t float64) bool {
		p := sb.PointAt(t)
		return turn(a0, angle(p)) <= span || turn(a1, angle(p)) > 2*math.Pi-1e-12
	}
	const steps = 64
	var cuts []float64
	t0, in0 := 0.0, inside(0)
	for i := 1; i <= steps; i++ {
		t1 := float64(i) / steps
		in1 := inside(t1)
		if in0 != in1 {
			lo, hi := t0, t1
			for range 60 {
				m := (lo + hi) / 2
				if inside(m) == in0 {
					lo = m
				} else {
					hi = m
				}
			}
			t := (lo + hi) / 2
			if !sb.PointAt(t).Equals(sb.Start()) && !sb.PointAt(t).Equals(sb.Finish()) {
				cuts = append(cuts, t)
			}
		}
		t0, in0 = t1, in1
	}
	if len(cuts) == 0 {
		if !inside(0.5) {
			return nil
		}
		return []SBezier{sb}
	}
	bounds := append(append([]float64{0}, cuts...), 1)
	var out []SBezier
	for i := 0; i+1 < len(bounds); i++ {
		if !inside((bounds[i] + bounds[i+1]) / 2) {
			continue
		}
		out = append(out, sb.Sub(bounds[i], bounds[i+1]))
	}
	return out
}
