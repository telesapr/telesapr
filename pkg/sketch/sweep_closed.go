package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A sweep along a closed path, a ring, a frame or a closed curve,
// goes round from the profile and back to it, with no caps. The profile
// is drawn across the path somewhere along it, and the sweep starts
// there: the path is cut where the profile's plane crosses it, nearest
// the profile, and taken on from that point round to it. Started part
// way along a piece, the sweep closes where the path is smooth, so the
// corners of a frame are mitred as an open sweep's are, the joint where
// it closes needing none. Along lines and arcs the profile must come
// back as it left, as it does round a path in a plane; round a curve, or
// lines and arcs out of a plane, the frames are fitted and the turn they
// come back with is taken out evenly along the path
// (topo.PathSweepOptions.Closed).

// closedChainAt is a closed chain cut where the plane through onPlane
// of normal n crosses it, nearest near, and started there.
func (sk *Sketch) closedChainAt(chain []pathPiece, n, onPlane, near geom.Vector) ([]pathPiece, error) {
	// A circle, and pieces a corner radius cut or made, have no curves
	// yet.
	chain = append([]pathPiece(nil), chain...)
	for k := range chain {
		switch p := &chain[k]; {
		case p.curves != nil:
		case p.arc:
			p.curves = arcCurves(p.center, p.axis, p.start, p.angle)
		default:
			p.curves = []srf.SBezier{srf.SBezierFrom2(p.start, p.end)}
		}
	}
	type crossing struct {
		piece, curve int
		t            float64
		at           geom.Vector
	}
	var best *crossing
	side := func(p geom.Vector) float64 { return p.Minus(onPlane).Dot(n) }
	for k, p := range chain {
		for ci := range p.curves {
			c := &p.curves[ci]
			const steps = 64
			prev := side(c.PointAt(0))
			for j := 1; j <= steps; j++ {
				lo, hi := float64(j-1)/steps, float64(j)/steps
				next := side(c.PointAt(hi))
				if prev*next > 0 || prev == 0 && j > 1 {
					prev = next
					continue
				}
				// Halve the step down to where the curve meets the plane.
				flo := prev
				for range 60 {
					m := (lo + hi) / 2
					if fm := side(c.PointAt(m)); fm*flo > 0 {
						lo, flo = m, fm
					} else {
						hi = m
					}
				}
				t := (lo + hi) / 2
				x := c.PointAt(t)
				if best == nil || x.Minus(near).Magnitude() < best.at.Minus(near).Magnitude() {
					best = &crossing{piece: k, curve: ci, t: t, at: x}
				}
				prev = next
			}
		}
	}
	if best == nil {
		return nil, pathErr(&onPlane, "the profile's plane doesn't cross the closed path: draw the profile across it")
	}
	x := best.at
	k := best.piece
	p := chain[k]
	rotated := func(from int) []pathPiece {
		out := append([]pathPiece(nil), chain[from:]...)
		return append(out, chain[:from]...)
	}
	var out []pathPiece
	switch {
	case x.Minus(p.start).Magnitude() < sweepTol:
		out = rotated(k)
	case x.Minus(p.end).Magnitude() < sweepTol:
		out = rotated((k + 1) % len(chain))
	default:
		a, b := splitPiece(p, best.curve, best.t, x)
		out = append([]pathPiece{b}, chain[k+1:]...)
		out = append(out, chain[:k]...)
		out = append(out, a)
	}
	if out[len(out)-1].tangentAt(true).Dot(out[0].tangentAt(false)) < sweepTangentCos {
		return nil, pathErr(&x, "the profile is at a corner of the closed path, at (%g, %g, %g): "+
			"draw it across the path where it is smooth", x.X, x.Y, x.Z)
	}
	return out, nil
}

// roundClosedCorners is roundCorners round a closed chain, the joint
// where it closes too.
func (sk *Sketch) roundClosedCorners(chain []pathPiece, r float64) ([]pathPiece, error) {
	last := len(chain) - 1
	if chain[last].tangentAt(true).Dot(chain[0].tangentAt(false)) < sweepTangentCos {
		fillet, a, b, err := sk.roundCorner(chain[last], chain[0], r)
		if err != nil {
			return nil, err
		}
		chain = append([]pathPiece(nil), chain...)
		chain[last], chain[0] = a, b
		chain = append(chain, fillet)
	}
	return sk.roundCorners(chain, r)
}

// splitPiece cuts p at x, the point at t on its curve ci, into the
// piece up to x and the piece on from it.
func splitPiece(p pathPiece, ci int, t float64, x geom.Vector) (pathPiece, pathPiece) {
	a, b := p, p
	a.end, b.start = x, x
	switch {
	case p.arc:
		r0, rx := p.start.Minus(p.center), x.Minus(p.center)
		phi := math.Atan2(r0.Cross(rx).Dot(p.axis), r0.Dot(rx))
		if phi < 0 {
			phi += 2 * math.Pi
		}
		a.angle, b.angle = phi, p.angle-phi
		a.curves = arcCurves(p.center, p.axis, p.start, a.angle)
		b.curves = arcCurves(p.center, p.axis, x, b.angle)
	case !p.free:
		a.curves = []srf.SBezier{srf.SBezierFrom2(p.start, x)}
		b.curves = []srf.SBezier{srf.SBezierFrom2(x, p.end)}
	default:
		var bef, aft srf.SBezier
		p.curves[ci].SplitAt(t, &bef, &aft)
		bef.Ctrl[bef.Degree], aft.Ctrl[0] = x, x
		a.curves = append(append([]srf.SBezier(nil), p.curves[:ci]...), bef)
		b.curves = append([]srf.SBezier{aft}, p.curves[ci+1:]...)
	}
	return a, b
}

// arcCurves are the rational quadratics of the arc from start about
// the axis through center by angle, by the right-hand rule, a quarter
// turn at most each.
func arcCurves(center, axis, start geom.Vector, angle float64) []srf.SBezier {
	n := max(1, int(math.Ceil(angle/(math.Pi/2)-1e-9)))
	th := angle / float64(n)
	r0 := start.Minus(center)
	var out []srf.SBezier
	for i := range n {
		p0 := center.Plus(r0.RotatedAroundAxis(axis, float64(i)*th))
		p2 := center.Plus(r0.RotatedAroundAxis(axis, float64(i+1)*th))
		if i == 0 {
			p0 = start
		}
		mid := r0.RotatedAroundAxis(axis, (float64(i)+0.5)*th)
		p1 := center.Plus(mid.ScaledBy(1 / math.Cos(th/2)))
		sb := srf.SBezierFrom3(p0, p1, p2)
		sb.Weight[1] = math.Cos(th / 2)
		out = append(out, sb)
	}
	// The last ends where the arc does, as the next piece starts.
	return out
}

// loopsCentre is the middle of the control points of the profile's
// first outer loop: where the profile is, to find the crossing of the
// path nearest it.
func loopsCentre(sbls *srf.SBezierLoopSetSet) geom.Vector {
	var sum geom.Vector
	n := 0
	for _, c := range sbls.Sets[0].Outer.Curves {
		for i := 0; i <= c.Degree; i++ {
			sum = sum.Plus(c.Ctrl[i])
			n++
		}
	}
	return sum.ScaledBy(1 / float64(n))
}
