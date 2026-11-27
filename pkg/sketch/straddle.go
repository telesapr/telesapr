package sketch

import (
	"errors"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ErrProfileStraddles is the problem of a lathe, a revolve, a helix or a
// sweep round an arc whose profile moves forward through its own plane
// in one part and back in another: its plane does not hold the axis,
// and the profile reaches both sides of the line on it where the motion
// runs along the plane. The kernel sweeps the profile's sides and caps
// alone, which would cross one another, and the part moving backwards
// would count against the rest, so the solid is refused rather than
// made wrong.
var ErrProfileStraddles = errors.New("the profile moves forward through its own plane in one part and back in another")

// straddleAdvice is what the problem tells the user to do about it.
const straddleAdvice = "draw the profile on a plane through the axis, or keep it to one side of the marked line, where the motion runs along its plane"

// ErrProfileCrossesAxis is the problem of a lathe, a revolve or a helix
// whose profile, on a plane that holds the axis, reaches both sides of
// it: the part across the axis would turn the other way through the
// plane, and count against the rest. A sweep's path says the same of
// its own arcs, as a bend too tight for the profile.
var ErrProfileCrossesAxis = errors.New("the profile crosses the axis it turns about")

// crossAdvice is what that problem tells the user to do about it.
const crossAdvice = "keep it to one side of the axis"

// straddleDepth bounds the halving of a curve while its extremes are
// sought; a curve that only comes within straddleEps of the line after
// that many halvings touches it.
const straddleDepth = 48

// straddleEps is how far past the line, along the motion, a profile must
// reach on both sides to be refused: a profile touching the line, as
// one with an edge on it, is made.
const straddleEps = core.LengthEps

// turnStraddle is the point on the line where the motion runs along the
// profile's plane, inside the profile, where the profile reaches both
// sides of that line as it turns about the axis through pt along the
// unit axis by da while advancing dx along it; ok false where it does
// not, or where the plane holds the axis (that is axisCrossing's).
func turnStraddle(sbls *srf.SBezierLoopSet, pt, axis geom.Vector, da, dx float64) (at geom.Vector, ok bool) {
	n := sbls.Normal.WithMagnitude(1)
	if holdsAxis(sbls, n, pt, axis) {
		return geom.Vector{}, false
	}
	// The velocity at q is da axis×(q - pt) + dx axis; along the normal
	// that is q·across + c, linear over the plane.
	across := n.Cross(axis).ScaledBy(da)
	c := -pt.Dot(across) + dx*n.Dot(axis)
	if across.Magnitude() < 1e-12*math.Max(1, math.Abs(da)) {
		// The plane is square to the axis: all of it moves one way.
		return geom.Vector{}, false
	}
	// Scaled to a length in the plane, so that straddleEps is one.
	scale := 1 / across.Magnitude()
	g := func(q geom.Vector) float64 { return (q.Dot(across) + c) * scale }
	hi, okHi := loopSetReaches(sbls, g, 1)
	if !okHi {
		return geom.Vector{}, false
	}
	lo, okLo := loopSetReaches(sbls, g, -1)
	if !okLo {
		return geom.Vector{}, false
	}
	// g is linear, so it is zero on the chord between them at this
	// fraction: a point on the line, inside a convex profile.
	return zeroBetween(g, hi, lo), true
}

// axisCrossing is the point of the axis through pt along the unit axis
// inside the profile, where the profile's plane holds the axis and the
// profile reaches both sides of it; ok false where it does not, as one
// that only touches the axis.
func axisCrossing(sbls *srf.SBezierLoopSet, pt, axis geom.Vector) (at geom.Vector, ok bool) {
	n := sbls.Normal.WithMagnitude(1)
	if !holdsAxis(sbls, n, pt, axis) {
		return geom.Vector{}, false
	}
	// The distance from the axis, signed by the side of it.
	side := n.Cross(axis).WithMagnitude(1)
	g := func(q geom.Vector) float64 { return q.Minus(pt).Dot(side) }
	hi, okHi := loopSetReaches(sbls, g, 1)
	if !okHi {
		return geom.Vector{}, false
	}
	lo, okLo := loopSetReaches(sbls, g, -1)
	if !okLo {
		return geom.Vector{}, false
	}
	return zeroBetween(g, hi, lo), true
}

// zeroBetween is the point of the chord from hi to lo where the linear
// g, positive at hi and negative at lo, is zero: inside a convex
// profile they are points of.
func zeroBetween(g func(geom.Vector) float64, hi, lo geom.Vector) geom.Vector {
	gh, gl := g(hi), g(lo)
	return hi.Plus(lo.Minus(hi).ScaledBy(gh / (gh - gl)))
}

// holdsAxis reports whether the plane of the loop set, of unit normal
// n, holds the axis through pt along axis, within LengthEps over the
// profile's reach along it.
func holdsAxis(sbls *srf.SBezierLoopSet, n, pt, axis geom.Vector) bool {
	if math.Abs(n.Dot(pt.Minus(sbls.Point))) > core.LengthEps {
		return false
	}
	reach := 1.0
	for _, l := range loopsOf(sbls) {
		for i := range l.Curves {
			b := &l.Curves[i]
			for k := 0; k <= b.Degree; k++ {
				reach = math.Max(reach, b.Ctrl[k].Minus(pt).Magnitude())
			}
		}
	}
	return math.Abs(n.Dot(axis))*reach <= core.LengthEps
}

// loopsOf is the outer loop and the holes of the set.
func loopsOf(sbls *srf.SBezierLoopSet) []*srf.SBezierLoop {
	ls := []*srf.SBezierLoop{&sbls.Outer}
	for i := range sbls.Holes {
		ls = append(ls, &sbls.Holes[i])
	}
	return ls
}

// loopSetReaches is a point of the loop set's curves where sign g is
// past straddleEps, if there is one.
func loopSetReaches(sbls *srf.SBezierLoopSet, g func(geom.Vector) float64, sign float64) (geom.Vector, bool) {
	for _, l := range loopsOf(sbls) {
		for i := range l.Curves {
			if p, ok := curveReaches(l.Curves[i], g, sign, straddleDepth); ok {
				return p, true
			}
		}
	}
	return geom.Vector{}, false
}

// curveReaches is a point of the curve where sign g is past straddleEps,
// if there is one. A rational curve of positive weights lies in the hull
// of its control points, and g is linear, so the curve stays short of
// straddleEps where every control point does; else it is halved, and
// its halves sought in turn, which finds an extreme inside the curve,
// as a circle's, and not only at its ends.
func curveReaches(b srf.SBezier, g func(geom.Vector) float64, sign float64, depth int) (geom.Vector, bool) {
	for _, p := range []geom.Vector{b.Start(), b.Finish(), b.PointAt(0.5)} {
		if sign*g(p) > straddleEps {
			return p, true
		}
	}
	short := true
	for k := 0; k <= b.Degree; k++ {
		if b.Weight[k] <= 0 {
			// No hull to bound it: sampled instead.
			for j := range 257 {
				if p := b.PointAt(float64(j) / 256); sign*g(p) > straddleEps {
					return p, true
				}
			}
			return geom.Vector{}, false
		}
		if sign*g(b.Ctrl[k]) > straddleEps {
			short = false
		}
	}
	if short || depth == 0 {
		return geom.Vector{}, false
	}
	var bef, aft srf.SBezier
	b.SplitAt(0.5, &bef, &aft)
	if p, ok := curveReaches(bef, g, sign, depth-1); ok {
		return p, true
	}
	return curveReaches(aft, g, sign, depth-1)
}

// refuseTurned sets the group's problem and reports true where one of
// the source's loop sets crosses the axis through pt along axis, in a
// plane that holds it, or else straddles the line on its plane where the
// turn about it, by da with an advance of dx along it, runs along the
// plane.
func (g *Group) refuseTurned(sets []srf.SBezierLoopSet, pt, axis geom.Vector, da, dx float64) bool {
	for i := range sets {
		if at, ok := axisCrossing(&sets[i], pt, axis); ok {
			g.setProblem(&at, "%v: %s", ErrProfileCrossesAxis, crossAdvice)
			return true
		}
		if at, ok := turnStraddle(&sets[i], pt, axis, da, dx); ok {
			g.setProblem(&at, "%v: %s", ErrProfileStraddles, straddleAdvice)
			return true
		}
	}
	return false
}

// refuseStraddlingSweep refuses a straddling profile at each arc of a
// sweep's rigid steps, the profile moved there by the steps before it.
// A profile crossing an arc's axis is refused by the sweep's path.
func (g *Group) refuseStraddlingSweep(prof *srf.SBezierLoopSetSet, steps []srf.SweepStep) bool {
	r := srf.RigidIdentity
	for _, st := range steps {
		if st.Arc && !st.Corner {
			for i := range prof.Sets {
				moved := srf.TransformedLoopSet(&prof.Sets[i], r)
				if at, ok := turnStraddle(moved, st.Center, st.Axis.WithMagnitude(1), st.Angle, 0); ok {
					g.setProblem(&at, "%v: %s", ErrProfileStraddles, straddleAdvice)
					return true
				}
			}
		}
		r = r.Then(st.Motion())
	}
	return false
}
