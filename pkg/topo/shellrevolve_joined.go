package topo

import (
	"errors"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// Lathes about one axis joined to each other make a solid of
// revolution too: whole turns of profiles combined are the whole turn
// of the profiles combined in a plane through the axis. Each profile
// is turned about the axis into the first's half-plane; every curve is
// split where it meets another's, and a piece is kept where the
// combined region is on one side of it only, as the parts' insides
// decide by the Booleans that joined them. The profile is so made of
// the lathes' own lines and arcs. A Boolean of prisms of the profiles
// would do it in the kernel, but lathes joined usually touch along a
// face (a flange against a cup's side), which a Boolean leaves.

// RevolvedPart is one of the whole turns a solid of revolution is
// joined from: its profile, closed loops of lines and arcs in a plane
// through the axis, and how it is combined with the solid of the parts
// before it.
type RevolvedPart struct {
	Profile *srf.SBezierLoopSetSet
	Combine srf.SurfaceCombineAs
}

// JoinedRevolution is the revolution, a whole turn about the axis (pt,
// axis), that parts make combined in order, at tol. It is
// ErrNotRevolution when a profile does not lie in a plane through the
// axis on one side of it.
func JoinedRevolution(pt, axis geom.Vector, parts []RevolvedPart, tol srf.Tolerance) (Revolution, error) {
	axis = axis.WithMagnitude(1)
	if len(parts) == 0 {
		return Revolution{}, ErrNotRevolution
	}
	var radial0, n0 geom.Vector
	curves := make([][]srf.SBezier, len(parts)) // each part's, in the first's half-plane
	var size float64
	for i, p := range parts {
		radial, err := profileRadial(p.Profile, pt, axis)
		if err != nil {
			return Revolution{}, err
		}
		if i == 0 {
			radial0, n0 = radial, axis.Cross(radial).WithMagnitude(1)
		}
		// The angle that turns this profile's half-plane onto the first's.
		a := math.Atan2(radial.Cross(radial0).Dot(axis), radial.Dot(radial0))
		for _, set := range p.Profile.Sets {
			for _, l := range append([]srf.SBezierLoop{set.Outer}, set.Holes...) {
				for _, sb := range l.Curves {
					for k := 0; k <= sb.Degree; k++ {
						sb.Ctrl[k] = sb.Ctrl[k].RotatedAbout(pt, axis, a)
					}
					if sb.Degree > 2 {
						return Revolution{}, shellErr(sb.PointAt(0.5), "a curve of a lathe's profile is neither a line nor an arc; "+
							"a shell hollows revolves of lines and arcs")
					}
					curves[i] = append(curves[i], sb)
					size = max(size, sb.Start().Minus(pt).Magnitude(), sb.Finish().Minus(pt).Magnitude())
				}
			}
		}
	}
	eps := max(geom.LengthEps, 1e-9*size)
	step := max(100*geom.LengthEps, 1e-5*size)
	inside := func(q geom.Vector) bool {
		in := false
		for i, p := range parts {
			c := crossings(curves[i], q, n0, eps)%2 == 1
			switch {
			case i == 0:
				in = c
			case p.Combine == srf.SurfaceUnion:
				in = in || c
			case p.Combine == srf.SurfaceDiff:
				in = in && !c
			default:
				in = in && c
			}
		}
		return in
	}

	var all []srf.SBezier
	for _, cs := range curves {
		all = append(all, cs...)
	}
	var kept srf.SBezierList
	var mids []geom.Vector
	for i := range all {
		for _, sb := range splitAtOthers(all, i, n0, eps) {
			m := sb.PointAt(0.5)
			side := n0.Cross(sb.TangentAt(0.5)).WithMagnitude(step)
			if inside(m.Plus(side)) == inside(m.Minus(side)) {
				continue
			}
			// A piece two parts share is kept once.
			if slices.ContainsFunc(mids, func(o geom.Vector) bool { return o.Minus(m).Magnitude() < 100*eps }) {
				continue
			}
			mids = append(mids, m)
			// Loops are found among curves of one AuxA, and the parts'
			// sketches may give theirs different ones.
			sb.AuxA = 0
			kept.Curves = append(kept.Curves, sb)
		}
	}

	kept.Curves = joinLines(joinEnds(kept.Curves, eps), eps)

	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	allClosed, allCoplanar := false, false
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&kept, &poly, nil, tol, &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	if !allClosed || !allCoplanar || len(sets.Sets) == 0 {
		return Revolution{}, errors.New("the lathes' profiles joined do not close")
	}
	return Revolution{Profile: &sets, Pt: pt, Axis: axis, Start: 0, End: 2 * math.Pi}, nil
}

// profileRadial is the unit direction from the axis (pt, axis) to the
// half-plane through it the profile lies in.
func profileRadial(profile *srf.SBezierLoopSetSet, pt, axis geom.Vector) (geom.Vector, error) {
	if profile == nil || len(profile.Sets) == 0 {
		return geom.Vector{}, ErrNotRevolution
	}
	n := profile.Sets[0].Normal.WithMagnitude(1)
	if math.Abs(n.Dot(axis)) > 1e-9 {
		return geom.Vector{}, ErrNotRevolution
	}
	radial := n.Cross(axis).WithMagnitude(1)
	side := 0.0
	for _, set := range profile.Sets {
		for _, l := range append([]srf.SBezierLoop{set.Outer}, set.Holes...) {
			for _, sb := range l.Curves {
				for k := 0; k <= sb.Degree; k++ {
					d := sb.Ctrl[k].Minus(pt)
					if math.Abs(d.Dot(n)) > geom.LengthEps {
						return geom.Vector{}, ErrNotRevolution
					}
					s := d.Dot(radial)
					if math.Abs(s) <= geom.LengthEps {
						continue
					}
					if side != 0 && math.Signbit(s) != math.Signbit(side) {
						return geom.Vector{}, ErrNotRevolution
					}
					side = s
				}
			}
		}
	}
	if side == 0 {
		return geom.Vector{}, ErrNotRevolution
	}
	if side < 0 {
		radial = radial.ScaledBy(-1)
	}
	return radial, nil
}

// onCurve is the parameter of p on sb, a line or an arc, and whether p
// lies on it within eps.
func onCurve(sb *srf.SBezier, p geom.Vector, eps float64) (float64, bool) {
	t, _ := sb.ClosestPointTo(p, false)
	return t, t >= -1e-9 && t <= 1+1e-9 && sb.PointAt(t).Minus(p).Magnitude() < eps
}

// carrier is the line or circle sb lies on, in the plane of normal n:
// a line through a along d, or a circle about c of radius r.
type carrier struct {
	arc  bool
	a, d geom.Vector
	c    geom.Vector
	r    float64
}

func carrierOf(sb *srf.SBezier, n geom.Vector) carrier {
	a, m, b := sb.Start(), sb.PointAt(0.5), sb.Finish()
	if sb.Degree == 1 || m.DistanceToLine(a, b.Minus(a)) < geom.LengthEps {
		return carrier{a: a, d: b.Minus(a).WithMagnitude(1)}
	}
	c, ok := circleThrough(a, m, b, n)
	if !ok {
		return carrier{a: a, d: b.Minus(a).WithMagnitude(1)}
	}
	return carrier{arc: true, c: c, r: a.Minus(c).Magnitude()}
}

// meet is where the carriers p and q meet, in the plane of normal n.
func meet(p, q carrier, n geom.Vector) []geom.Vector {
	switch {
	case !p.arc && !q.arc:
		den := p.d.Cross(q.d).Dot(n)
		if math.Abs(den) < 1e-12 {
			return nil
		}
		s := q.a.Minus(p.a).Cross(q.d).Dot(n) / den
		return []geom.Vector{p.a.Plus(p.d.ScaledBy(s))}
	case p.arc && !q.arc:
		return meet(q, p, n)
	case !p.arc:
		// The line's points a + s d at r from c.
		f := p.a.Minus(q.c)
		b := f.Dot(p.d)
		disc := b*b - (f.Dot(f) - q.r*q.r)
		if disc < 0 {
			return nil
		}
		h := math.Sqrt(disc)
		return []geom.Vector{p.a.Plus(p.d.ScaledBy(-b - h)), p.a.Plus(p.d.ScaledBy(-b + h))}
	default:
		dc := q.c.Minus(p.c)
		d := dc.Magnitude()
		if d < 1e-12 || d > p.r+q.r || d < math.Abs(p.r-q.r) {
			return nil
		}
		x := (d*d + p.r*p.r - q.r*q.r) / (2 * d)
		y := math.Sqrt(max(0, p.r*p.r-x*x))
		u := dc.ScaledBy(1 / d)
		w := n.Cross(u)
		mid := p.c.Plus(u.ScaledBy(x))
		return []geom.Vector{mid.Plus(w.ScaledBy(y)), mid.Minus(w.ScaledBy(y))}
	}
}

// splitAtOthers is curve i of all split where the others meet it or
// end on it.
func splitAtOthers(all []srf.SBezier, i int, n geom.Vector, eps float64) []srf.SBezier {
	sb := all[i]
	ci := carrierOf(&sb, n)
	var ts []float64
	add := func(p geom.Vector) {
		if t, ok := onCurve(&sb, p, eps); ok && t > 1e-7 && t < 1-1e-7 {
			ts = append(ts, t)
		}
	}
	for j := range all {
		if j == i {
			continue
		}
		o := &all[j]
		add(o.Start())
		add(o.Finish())
		for _, p := range meet(ci, carrierOf(o, n), n) {
			if _, ok := onCurve(o, p, eps); ok {
				add(p)
			}
		}
	}
	slices.Sort(ts)
	var out []srf.SBezier
	rest, from := sb, 0.0
	for _, t := range ts {
		if t-from < 1e-7 {
			continue
		}
		var bef, aft srf.SBezier
		rest.SplitAt((t-from)/(1-from), &bef, &aft)
		out = append(out, standardWeights(bef))
		rest, from = aft, t
	}
	return append(out, standardWeights(rest))
}

// standardWeights is sb, a line or an arc, with its end weights 1: the
// same curve, as a split arc leaves it otherwise, and as a revolution
// of it takes it.
func standardWeights(sb srf.SBezier) srf.SBezier {
	if sb.Degree != 2 || (sb.Weight[0] == 1 && sb.Weight[2] == 1) {
		return sb
	}
	sb.Weight[1] /= math.Sqrt(sb.Weight[0] * sb.Weight[2])
	sb.Weight[0], sb.Weight[2] = 1, 1
	return sb
}

// crossings is how many times a ray from q, in the plane of normal n,
// crosses curves.
func crossings(curves []srf.SBezier, q, n geom.Vector, eps float64) int {
	// An odd direction, so the ray passes no vertex of a drawn profile.
	dir := n.NormalDir(0).RotatedAroundAxis(n, 0.3141).WithMagnitude(1)
	ray := carrier{a: q, d: dir}
	k := 0
	for i := range curves {
		for _, p := range meet(ray, carrierOf(&curves[i], n), n) {
			if p.Minus(q).Dot(dir) <= 0 {
				continue
			}
			if _, ok := onCurve(&curves[i], p, eps); ok {
				k++
			}
		}
	}
	return k
}

// joinLines joins lines of curves that run on from each other along
// one line, where no other curve ends: the parts' lines a joined
// profile is cut from meet end to end there, as across a flange's foot
// and a cup's, and the shell takes one line as one face.
func joinLines(curves []srf.SBezier, eps float64) []srf.SBezier {
	same := func(p, q geom.Vector) bool { return p.Minus(q).Magnitude() < 100*eps }
	ends := func(p geom.Vector) int {
		k := 0
		for i := range curves {
			if same(curves[i].Start(), p) || same(curves[i].Finish(), p) {
				k++
			}
		}
		return k
	}
	for joined := true; joined; {
		joined = false
		for i := range curves {
			for j := range curves {
				a, b := curves[i], curves[j]
				if i == j || a.Degree != 1 || b.Degree != 1 {
					continue
				}
				// a then b, or a then b reversed.
				var far geom.Vector
				switch {
				case same(a.Finish(), b.Start()):
					far = b.Finish()
				case same(a.Finish(), b.Finish()):
					far = b.Start()
				default:
					continue
				}
				if ends(a.Finish()) != 2 || far.DistanceToLine(a.Start(), a.Finish().Minus(a.Start())) > eps ||
					far.Minus(a.Finish()).Dot(a.Finish().Minus(a.Start())) <= 0 {
					continue
				}
				curves[i] = srf.SBezierFrom2(a.Start(), far)
				curves = slices.Delete(curves, j, j+1)
				joined = true
				break
			}
			if joined {
				break
			}
		}
	}
	return curves
}

// joinEnds makes the ends of curves that are one point within eps the
// same point: splits leave them a few ulps apart, and loops are found
// by ends that match.
func joinEnds(curves []srf.SBezier, eps float64) []srf.SBezier {
	var pts []geom.Vector
	snap := func(p *geom.Vector) {
		for _, q := range pts {
			if q.Minus(*p).Magnitude() < 100*eps {
				*p = q
				return
			}
		}
		pts = append(pts, *p)
	}
	for i := range curves {
		snap(&curves[i].Ctrl[0])
		snap(&curves[i].Ctrl[curves[i].Degree])
	}
	return curves
}
