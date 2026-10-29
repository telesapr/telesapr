package srf

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Where a sweep's path has a corner beside an arc, no plane halves it:
// a plane through the corner cuts the arc's turned sides along curves
// the straight piece's sides don't pass through. Each point of the
// profile runs along its own path through the corner instead, a line
// along a straight piece and a circle about an arc's axis, all in the
// plane square to the corner's axis. Carried on past the corner, the two
// paths of a point cross where the solid's sides meet: on the outside of
// the turn each piece goes on past the corner to fill it, on the inside
// each stops where the other's side cuts it, as a mitre between two
// lines does. The joint of a curve of the profile is the curve of those
// crossings, shared by the two pieces' surfaces, each carried on as far
// as the joint needs.

// Errors CheckSweepCorners wraps, for a corner beside an arc.
var (
	// ErrCornerOutOfPlane is a corner whose turn is about an axis other
	// than the arc's beside it: the points' paths don't cross.
	ErrCornerOutOfPlane = errors.New("the corner is out of the plane of the arc beside it")
	// ErrCornerNoJoint is a corner where some point's paths along the
	// two pieces don't meet.
	ErrCornerNoJoint = errors.New("the pieces' sides don't meet at the corner")
	// ErrCornerTooLong is a joint running past the other end of a piece:
	// the piece is too short for its corners.
	ErrCornerTooLong = errors.New("a piece is too short for the joint at its corner")
	// ErrCornerTooWide is a joint carried further round an arc than one
	// surface of it holds.
	ErrCornerTooWide = errors.New("the joint goes too far round the arc")
)

// jointMargin is how far, as an angle, an arc's surface reaches past
// the last of its joint's points; jointSpan is the most a surface of a
// turn spans with its reach, short of the quarter turn a rational
// quadratic holds and a plane of a revolved line covers.
const (
	jointMargin = 0.02
	jointSpan   = math.Pi/2 - 0.05
)

// sweepCorner is a corner of a sweep's path with an arc beside it, a
// on the way in and b on the way out.
type sweepCorner struct {
	a, b SweepStep
	// endA is the profile's frame where a ends, startB where b takes it.
	endA, startB Rigid
	at, n        geom.Vector
	ex, ey       geom.Vector
	// The least and most travel of the profile's points past a's end and
	// into b: lengths along a line, angles along an arc.
	aMin, aMax, bMin, bMax float64
}

// jointedCorner says whether the corner between a and b is joined by
// the crossings of its points' paths, not by a plane.
func jointedCorner(a, b SweepStep) bool { return a.Arc || b.Arc }

// newSweepCorner is the corner cs between a and b, where a leaves the
// profile in the frame endA; samples are the profile's points, at
// which its travels are measured.
func newSweepCorner(a, cs, b SweepStep, endA Rigid, samples []geom.Vector) (*sweepCorner, error) {
	c := &sweepCorner{a: a, b: b, endA: endA, startB: endA.Then(cs.Motion()), at: cs.Center, n: cs.Axis.WithMagnitude(1)}
	for _, st := range []SweepStep{a, b} {
		if st.Arc && math.Abs(st.Axis.WithMagnitude(1).Dot(c.n)) < 1-1e-9 {
			return nil, ErrCornerOutOfPlane
		}
	}
	c.ex = c.n.NormalDir(0)
	c.ey = c.n.Cross(c.ex)
	c.aMin, c.aMax, c.bMin, c.bMax = math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
	for _, q := range samples {
		_, ta, tb, ok := c.cross(q)
		if !ok {
			return nil, ErrCornerNoJoint
		}
		c.aMin, c.aMax = math.Min(c.aMin, ta), math.Max(c.aMax, ta)
		c.bMin, c.bMax = math.Min(c.bMin, tb), math.Max(c.bMax, tb)
	}
	return c, nil
}

// track is a point's path along a piece, in the corner's plane: a line
// through at along the unit dir, or a circle about c of radius r.
type track struct {
	line    bool
	at, dir [2]float64
	c       [2]float64
	r       float64
	// sign is +1 where the arc turns about the corner's axis, -1
	// against it.
	sign float64
}

func (c *sweepCorner) to2(v geom.Vector) [2]float64 {
	w := v.Minus(c.at)
	return [2]float64{w.Dot(c.ex), w.Dot(c.ey)}
}

// trackOf is the path of the point at x along st.
func (c *sweepCorner) trackOf(st SweepStep, x geom.Vector) track {
	if !st.Arc {
		d := st.Along.WithMagnitude(1)
		return track{line: true, at: c.to2(x), dir: [2]float64{d.Dot(c.ex), d.Dot(c.ey)}}
	}
	t := track{c: c.to2(st.Center), at: c.to2(x), sign: 1}
	t.r = math.Hypot(t.at[0]-t.c[0], t.at[1]-t.c[1])
	if st.Axis.Dot(c.n) < 0 {
		t.sign = -1
	}
	return t
}

// travel is how far along the track's motion the point at its start
// goes to reach x: a length along a line, an angle along an arc.
func (t track) travel(x [2]float64) float64 {
	if t.line {
		return (x[0]-t.at[0])*t.dir[0] + (x[1]-t.at[1])*t.dir[1]
	}
	u := [2]float64{t.at[0] - t.c[0], t.at[1] - t.c[1]}
	v := [2]float64{x[0] - t.c[0], x[1] - t.c[1]}
	return t.sign * math.Atan2(u[0]*v[1]-u[1]*v[0], u[0]*v[0]+u[1]*v[1])
}

// length is a travel as a length.
func (t track) length(tr float64) float64 {
	if t.line {
		return math.Abs(tr)
	}
	return math.Abs(tr) * t.r
}

// cross is where the paths of the profile's point q along a and b
// cross, nearest the corner, and the travels to it: ta past a's end,
// tb into b, each negative the other way.
func (c *sweepCorner) cross(q geom.Vector) (x geom.Vector, ta, tb float64, ok bool) {
	pa, pb := c.endA.Apply(q), c.startB.Apply(q)
	tra, trb := c.trackOf(c.a, pa), c.trackOf(c.b, pb)
	best := math.Inf(1)
	var at [2]float64
	for _, p := range tracksCross(tra, trb) {
		u, v := tra.travel(p), trb.travel(p)
		if l := math.Max(tra.length(u), trb.length(v)); l < best {
			best, at, ta, tb = l, p, u, v
		}
	}
	if math.IsInf(best, 1) {
		return x, 0, 0, false
	}
	// Each point keeps its height along the corner's axis.
	h := pa.Minus(c.at).Dot(c.n)
	x = c.at.Plus(c.ex.ScaledBy(at[0])).Plus(c.ey.ScaledBy(at[1])).Plus(c.n.ScaledBy(h))
	return x, ta, tb, true
}

// tracksCross are where two tracks cross.
func tracksCross(a, b track) [][2]float64 {
	switch {
	case a.line && b.line:
		den := a.dir[0]*b.dir[1] - a.dir[1]*b.dir[0]
		if math.Abs(den) < 1e-15 {
			return nil
		}
		s := ((b.at[0]-a.at[0])*b.dir[1] - (b.at[1]-a.at[1])*b.dir[0]) / den
		return [][2]float64{{a.at[0] + s*a.dir[0], a.at[1] + s*a.dir[1]}}
	case a.line:
		return lineMeetsCircle(a, b)
	case b.line:
		return lineMeetsCircle(b, a)
	}
	return circleMeetsCircle(a, b)
}

// lineMeetsCircle are where the line l crosses the circle c.
func lineMeetsCircle(l, c track) [][2]float64 {
	fx, fy := l.at[0]-c.c[0], l.at[1]-c.c[1]
	b := fx*l.dir[0] + fy*l.dir[1]
	d := b*b - (fx*fx + fy*fy - c.r*c.r)
	if d < 0 {
		return nil
	}
	s := math.Sqrt(d)
	var out [][2]float64
	for _, t := range []float64{-b - s, -b + s} {
		out = append(out, [2]float64{l.at[0] + t*l.dir[0], l.at[1] + t*l.dir[1]})
	}
	return out
}

// circleMeetsCircle are where two circles cross.
func circleMeetsCircle(a, b track) [][2]float64 {
	dx, dy := b.c[0]-a.c[0], b.c[1]-a.c[1]
	d := math.Hypot(dx, dy)
	if d < 1e-12 || d > a.r+b.r || d < math.Abs(a.r-b.r) {
		return nil
	}
	l := (a.r*a.r - b.r*b.r + d*d) / (2 * d)
	h := math.Sqrt(math.Max(0, a.r*a.r-l*l))
	mx, my := a.c[0]+l*dx/d, a.c[1]+l*dy/d
	return [][2]float64{{mx - h*dy/d, my + h*dx/d}, {mx + h*dy/d, my - h*dx/d}}
}

// sweepSamples are points along every curve of the profile, at which
// a corner's travels are measured.
func sweepSamples(sbls *SBezierLoopSet) []geom.Vector {
	var out []geom.Vector
	for _, l := range sbls.loops() {
		for i := range l.Curves {
			for k := range 33 {
				out = append(out, l.Curves[i].PointAt(float64(k)/32))
			}
		}
	}
	return out
}

// sameArc says whether two arc steps turn about the same axis.
func sameArc(a, b SweepStep) bool {
	return a.Arc && b.Arc && a.Center.Minus(b.Center).Magnitude() < 1e-9 &&
		a.Axis.WithMagnitude(1).Dot(b.Axis.WithMagnitude(1)) > 1-1e-12
}

// jointedSteps splits again each arc beside a jointed corner, its
// quarter turns from sweepSections taken together, so that the section
// at a corner holds the joint's points with the reach its surface needs
// past them.
func jointedSteps(sbls *SBezierLoopSet, steps []SweepStep) ([]SweepStep, error) {
	jointed := func(k int) bool {
		return k > 0 && k+1 < len(steps) && steps[k].Corner && jointedCorner(steps[k-1], steps[k+1])
	}
	some := false
	for k := range steps {
		some = some || jointed(k)
	}
	if !some {
		return steps, nil
	}
	// The arcs beside a jointed corner, each one step.
	var merged []SweepStep
	for k := 0; k < len(steps); k++ {
		st := steps[k]
		if st.Arc {
			e := k
			for e+1 < len(steps) && sameArc(steps[e+1], st) {
				e++
			}
			if jointed(k-1) || jointed(e+1) {
				for j := k + 1; j <= e; j++ {
					st.Angle += steps[j].Angle
				}
				merged = append(merged, st)
				k = e
				continue
			}
		}
		merged = append(merged, st)
	}
	steps = merged
	corners, err := sweepCorners(sbls, steps)
	if err != nil {
		return nil, err
	}
	var out []SweepStep
	for k, st := range steps {
		var before, after *sweepCorner
		if k > 0 {
			before = corners[k-1]
		}
		if k+1 < len(steps) {
			after = corners[k+1]
		}
		if !st.Arc || before == nil && after == nil {
			out = append(out, st)
			continue
		}
		parts, err := splitJointedArc(st.Angle, before, after)
		if err != nil {
			at := st.Center
			switch {
			case before != nil:
				at = before.at
			case after != nil:
				at = after.at
			}
			return nil, &CornerError{At: at, Err: err}
		}
		for _, a := range parts {
			p := st
			p.Angle = a
			out = append(out, p)
		}
	}
	return out, nil
}

// sweepCorners are the jointed corners of steps, by the index of each
// corner's step, nil at every other.
func sweepCorners(sbls *SBezierLoopSet, steps []SweepStep) ([]*sweepCorner, error) {
	samples := sweepSamples(sbls)
	corners := make([]*sweepCorner, len(steps))
	f := RigidIdentity
	for k, st := range steps {
		if st.Corner && k > 0 && k+1 < len(steps) && jointedCorner(steps[k-1], steps[k+1]) {
			c, err := newSweepCorner(steps[k-1], st, steps[k+1], f, samples)
			if err != nil {
				return nil, &CornerError{At: st.Center, Err: err}
			}
			corners[k] = c
		}
		f = f.Then(st.Motion())
	}
	return corners, nil
}

// splitJointedArc splits an arc turning by total into sections, the
// first holding the joint of the corner before it, if any, and the last
// the joint of the corner after.
func splitJointedArc(total float64, before, after *sweepCorner) ([]float64, error) {
	// What the joints take of the arc's ends: reach past them, and the
	// way they cut into it.
	var reachS, cutS, reachE, cutE float64
	if before != nil {
		reachS, cutS = math.Max(0, -before.bMin)+jointMargin, math.Max(0, before.bMax)
	}
	if after != nil {
		reachE, cutE = math.Max(0, after.aMax)+jointMargin, math.Max(0, -after.aMin)
	}
	if total+reachS+reachE <= jointSpan {
		return []float64{total}, nil
	}
	first, last := jointSpan-reachS, jointSpan-reachE
	if before != nil && first <= cutS || after != nil && last <= cutE {
		return nil, ErrCornerTooWide
	}
	if before != nil && after != nil && first+last >= total {
		lo, hi := math.Max(cutS, total-last), math.Min(first, total-cutE)
		if lo >= hi {
			return nil, ErrCornerTooLong
		}
		x := (lo + hi) / 2
		return []float64{x, total - x}, nil
	}
	var head, tail []float64
	rest := total
	if before != nil {
		head = []float64{first}
		rest -= first
	}
	if after != nil {
		tail = []float64{last}
		rest -= last
	}
	if rest <= 0 {
		// One joint, and the arc too short for a section past it.
		return []float64{total}, nil
	}
	n := int(rest/(math.Pi/2) + 1)
	mid := make([]float64, n)
	for i := range mid {
		mid[i] = rest / float64(n)
	}
	return append(append(head, mid...), tail...), nil
}

// CornerError is a corner of a sweep's path that can't be joined, at
// the corner's point.
type CornerError struct {
	At  geom.Vector
	Err error
}

func (e *CornerError) Error() string {
	return fmt.Sprintf("corner at (%g, %g, %g): %v", e.At.X, e.At.Y, e.At.Z, e.Err)
}

func (e *CornerError) Unwrap() error { return e.Err }

// arcThrough is the circle's arc from p about the axis through center
// to q, which lie at the same distance from it: a quadratic of weight
// the cosine of half the turn.
func arcThrough(center, axis, p, q geom.Vector, angle float64) SBezier {
	w := math.Cos(angle / 2)
	c0 := p.ClosestPointOnLine(center, axis)
	mid := p.Plus(q).ScaledBy(0.5)
	b := SBezierFrom3(p, mid.Minus(c0).ScaledBy(1/(w*w)).Plus(c0), q)
	b.Weight[1] = w
	return b
}

// turnAngle is how far p turns about the axis through center, by the
// right-hand rule, to reach q's half-plane: in [0, 2π).
func turnAngle(center, axis, p, q geom.Vector) float64 {
	a, b := p.Minus(center), q.Minus(center)
	a, b = a.Minus(axis.ScaledBy(a.Dot(axis))), b.Minus(axis.ScaledBy(b.Dot(axis)))
	angle := math.Atan2(a.Cross(b).Dot(axis), a.Dot(b))
	if angle < 0 {
		angle += 2 * math.Pi
	}
	return angle
}

// jointCurve is the joint at corner c of the profile's curve b: where
// the paths of its points cross, a polyline on both pieces' surfaces,
// at b's parameters ts (the grid of the turn's surface beside it) and
// between them where the joint bows off its chord by more than the
// tolerance.
func jointCurve(c *sweepCorner, b *SBezier, ts []float64, tol Tolerance) SCurve {
	if ts == nil {
		ts = []float64{0, 1}
	}
	at := func(t float64) geom.Vector {
		q := b.PointAt(t)
		switch t {
		case 0:
			q = b.Start()
		case 1:
			q = b.Finish()
		}
		p, _, _, _ := c.cross(q)
		return p
	}
	chord := tol.ChordTolMm()
	var sc SCurve
	var add func(t0, t1 float64, p0, p1 geom.Vector, depth int)
	add = func(t0, t1 float64, p0, p1 geom.Vector, depth int) {
		tm := (t0 + t1) / 2
		pm := at(tm)
		if depth < 8 && pm.Minus(p0.Plus(p1).ScaledBy(0.5)).Magnitude() > chord {
			add(t0, tm, p0, pm, depth+1)
			add(tm, t1, pm, p1, depth+1)
			return
		}
		sc.Points = append(sc.Points, SCurvePt{P: p1})
	}
	p0 := at(ts[0])
	sc.Points = append(sc.Points, SCurvePt{P: p0, Vertex: true})
	for k := 1; k < len(ts); k++ {
		p1 := at(ts[k])
		add(ts[k-1], ts[k], p0, p1, 0)
		p0 = p1
	}
	sc.Points[len(sc.Points)-1].Vertex = true
	return sc
}

// CheckSweepCorners says whether MakeFromSweepOf, or
// MakeFromClosedSweepOf with closed, can join the corners of steps that
// have an arc beside them: a *CornerError at the first that can't be,
// wrapping one of the ErrCorner errors. Corners between two lines are
// mitred by a plane and not checked here.
func CheckSweepCorners(sbls *SBezierLoopSet, steps []SweepStep, closed bool) error {
	sbls, steps, _, err := sweepPlan(sbls, steps, closed)
	if err != nil || len(steps) == 0 {
		return err
	}
	corners, err := sweepCorners(sbls, steps)
	if err != nil {
		return err
	}
	samples := sweepSamples(sbls)
	// Each piece with a joint at an end must hold the joints and mitres
	// at both apart, at every point of the profile.
	f := RigidIdentity
	for k, st := range steps {
		if st.Corner {
			f = f.Then(st.Motion())
			continue
		}
		var before, after *sweepCorner
		if k > 0 {
			before = corners[k-1]
		}
		if k+1 < len(steps) {
			after = corners[k+1]
		}
		if before == nil && after == nil {
			f = f.Then(st.Motion())
			continue
		}
		total := st.Angle
		if !st.Arc {
			total = st.Along.Magnitude()
		}
		for _, q := range samples {
			from, to := 0.0, total
			p := f.Apply(q)
			switch {
			case before != nil:
				_, _, from, _ = before.cross(q)
			case k > 0 && steps[k-1].Corner:
				from = mitreAlong(p, st.Along, steps[k-2].Along, steps[k-1].Center)
			}
			switch {
			case after != nil:
				_, ta, _, _ := after.cross(q)
				to += ta
			case k+1 < len(steps) && steps[k+1].Corner:
				to = mitreAlong(p, st.Along, steps[k+2].Along, steps[k+1].Center)
			}
			if to-from <= 1e-6 {
				var at geom.Vector
				if after != nil {
					at = after.at
				} else {
					at = before.at
				}
				return &CornerError{At: at, Err: ErrCornerTooLong}
			}
		}
		f = f.Then(st.Motion())
	}
	return nil
}

// mitreAlong is how far along the line piece along the point p goes to
// the plane halving its corner with the line other at the corner at.
func mitreAlong(p, along, other, at geom.Vector) float64 {
	d := along.WithMagnitude(1)
	n := d.Plus(other.WithMagnitude(1)).WithMagnitude(1)
	return at.Minus(p).Dot(n) / d.Dot(n)
}
