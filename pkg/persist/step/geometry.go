package step

import (
	"math"
	"sync"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Curve is the geometry of an edge, in millimetres.
type Curve interface {
	Eval(t float64) geom.Vector
	Deriv(t float64) geom.Vector
	// Range is the curve's parameter range, infinite for a line.
	Range() (lo, hi float64)
	// Period is the length of the range over which a closed curve
	// repeats, or zero.
	Period() float64
	// Param is the parameter of the point of the curve nearest p.
	Param(p geom.Vector) float64
}

// breaker is a curve with knots, where its sampling starts.
type breaker interface {
	Breaks() []float64
}

// The sides of a surface's domain, for Surface.Pole.
const (
	poleUMin = iota
	poleUMax
	poleVMin
	poleVMax
)

// Surface is the geometry of a face, in millimetres. Its normal is
// Su x Sv.
type Surface interface {
	Eval(u, v float64) geom.Vector
	// Derivs is the point and the two first derivatives.
	Derivs(u, v float64) (p, su, sv geom.Vector)
	// Domain is the parameter rectangle, infinite where the surface is.
	Domain() (u0, u1, v0, v1 float64)
	// Period is the length over which each parameter repeats, or zero.
	Period() (pu, pv float64)
	// Pole is the parameter of a side of the domain where the surface
	// collapses to a point, such as a cone's apex, when it does.
	Pole(side int) (float64, bool)
	// Project is the parameters of the point of the surface nearest p;
	// guess, when not nil, is a start near it. A periodic parameter
	// comes back within one period of the domain's start.
	Project(p geom.Vector, guess *[2]float64) (u, v float64)
}

func growBox(p geom.Vector, lo, hi *geom.Vector) {
	lo.X, lo.Y, lo.Z = math.Min(lo.X, p.X), math.Min(lo.Y, p.Y), math.Min(lo.Z, p.Z)
	hi.X, hi.Y, hi.Z = math.Max(hi.X, p.X), math.Max(hi.Y, p.Y), math.Max(hi.Z, p.Z)
}

// frame is an axis placement: an origin and three orthonormal axes.
type frame struct {
	o, x, y, z geom.Vector
}

// local is p in the frame's axes.
func (f frame) local(p geom.Vector) geom.Vector {
	q := p.Minus(f.o)
	return geom.Vector{X: q.Dot(f.x), Y: q.Dot(f.y), Z: q.Dot(f.z)}
}

// at is the point with coordinates (a, b, c) in the frame.
func (f frame) at(a, b, c float64) geom.Vector {
	return f.o.Plus(f.x.ScaledBy(a)).Plus(f.y.ScaledBy(b)).Plus(f.z.ScaledBy(c))
}

// angle is the angle of (a, b) in [0, 2pi).
func angle(a, b float64) float64 {
	t := math.Atan2(b, a)
	if t < 0 {
		t += 2 * math.Pi
	}
	if t >= 2*math.Pi {
		t = 0
	}
	return t
}

// ---- curves ----

type line struct {
	p, d geom.Vector
}

func (l *line) Eval(t float64) geom.Vector  { return l.p.Plus(l.d.ScaledBy(t)) }
func (l *line) Deriv(float64) geom.Vector   { return l.d }
func (l *line) Range() (lo, hi float64)     { return math.Inf(-1), math.Inf(1) }
func (l *line) Period() float64             { return 0 }
func (l *line) Param(p geom.Vector) float64 { return p.Minus(l.p).Dot(l.d) / l.d.MagSquared() }

// ellipse is also a circle, with a equal to b.
type ellipse struct {
	f    frame
	a, b float64
}

func (e *ellipse) Eval(t float64) geom.Vector {
	return e.f.at(e.a*math.Cos(t), e.b*math.Sin(t), 0)
}

func (e *ellipse) Deriv(t float64) geom.Vector {
	return e.f.x.ScaledBy(-e.a * math.Sin(t)).Plus(e.f.y.ScaledBy(e.b * math.Cos(t)))
}

func (e *ellipse) Range() (lo, hi float64) { return 0, 2 * math.Pi }
func (e *ellipse) Period() float64         { return 2 * math.Pi }

func (e *ellipse) Param(p geom.Vector) float64 {
	q := e.f.local(p)
	t := angle(q.X/e.a, q.Y/e.b)
	if e.a != e.b {
		t = refineCurveParam(e, p, t)
		t = wrap(t, 0, 2*math.Pi)
	}
	return t
}

// conic is a parabola or a branch of a hyperbola in its frame's xy
// plane: (f t^2, 2 f t), or (a cosh t, b sinh t).
type conic struct {
	f         frame
	hyperbola bool
	a, b      float64
}

func (c *conic) Eval(t float64) geom.Vector {
	if c.hyperbola {
		return c.f.at(c.a*math.Cosh(t), c.b*math.Sinh(t), 0)
	}
	return c.f.at(c.a*t*t, 2*c.a*t, 0)
}

func (c *conic) Deriv(t float64) geom.Vector {
	if c.hyperbola {
		return c.f.x.ScaledBy(c.a * math.Sinh(t)).Plus(c.f.y.ScaledBy(c.b * math.Cosh(t)))
	}
	return c.f.x.ScaledBy(2 * c.a * t).Plus(c.f.y.ScaledBy(2 * c.a))
}

func (c *conic) Range() (lo, hi float64) { return math.Inf(-1), math.Inf(1) }
func (c *conic) Period() float64         { return 0 }

func (c *conic) Param(p geom.Vector) float64 {
	q := c.f.local(p)
	var t float64
	if c.hyperbola {
		t = math.Asinh(q.Y / c.b)
	} else {
		t = q.Y / (2 * c.a)
	}
	return refineCurveParam(c, p, t)
}

// refineCurveParam improves t, the parameter of the point of c nearest
// p, by Newton steps.
func refineCurveParam(c Curve, p geom.Vector, t float64) float64 {
	lo, hi := c.Range()
	for i := 0; i < 30; i++ {
		d := c.Deriv(t)
		m := d.MagSquared()
		if m == 0 {
			break
		}
		dt := p.Minus(c.Eval(t)).Dot(d) / m
		nt := t + dt
		if c.Period() == 0 {
			nt = math.Max(lo, math.Min(hi, nt))
		}
		if math.Abs(nt-t) < 1e-14*(1+math.Abs(t)) {
			t = nt
			break
		}
		t = nt
	}
	return t
}

// projectOnCurve is the parameter of the point of a bounded curve
// nearest p: the best of the nearest few samples, each refined.
func projectOnCurve(c Curve, p geom.Vector) float64 {
	lo, hi := c.Range()
	var ts []float64
	if b, ok := c.(breaker); ok {
		br := b.Breaks()
		per := 16
		if len(br) > 250 {
			per = 2
		} else if len(br) > 64 {
			per = 4
		}
		for i := 0; i+1 < len(br); i++ {
			for k := 0; k < per; k++ {
				ts = append(ts, br[i]+(br[i+1]-br[i])*float64(k)/float64(per))
			}
		}
		ts = append(ts, hi)
	} else {
		for k := 0; k <= 32; k++ {
			ts = append(ts, lo+(hi-lo)*float64(k)/32)
		}
	}
	type cand struct{ t, d float64 }
	var best [3]cand
	for i := range best {
		best[i] = cand{lo, math.Inf(1)}
	}
	for _, t := range ts {
		d := c.Eval(t).Minus(p).MagSquared()
		for i := range best {
			if d < best[i].d {
				copy(best[i+1:], best[i:len(best)-1])
				best[i] = cand{t, d}
				break
			}
		}
	}
	t, d := best[0].t, best[0].d
	for _, b := range best {
		if math.IsInf(b.d, 1) {
			continue
		}
		rt := refineCurveParam(c, p, b.t)
		if rd := c.Eval(rt).Minus(p).MagSquared(); rd < d {
			t, d = rt, rd
		}
	}
	return t
}

// polylineCurve is a POLYLINE, as a B-spline of degree one through its
// points.
func polylineCurve(pts []geom.Vector) (Curve, error) {
	knots := make([]float64, len(pts)+2)
	for i := range knots {
		switch {
		case i == 0:
			knots[i] = 0
		case i > len(pts):
			knots[i] = float64(len(pts) - 1)
		default:
			knots[i] = float64(i - 1)
		}
	}
	return newBsplineCurve(1, knots, pts, nil)
}

// ---- surfaces ----

type plane struct{ f frame }

func (s *plane) Eval(u, v float64) geom.Vector { return s.f.at(u, v, 0) }
func (s *plane) Derivs(u, v float64) (p, su, sv geom.Vector) {
	return s.f.at(u, v, 0), s.f.x, s.f.y
}
func (s *plane) Domain() (u0, u1, v0, v1 float64) { return infDomain() }
func (s *plane) Period() (pu, pv float64)         { return 0, 0 }
func (s *plane) Pole(int) (float64, bool)         { return 0, false }
func (s *plane) Project(p geom.Vector, _ *[2]float64) (u, v float64) {
	q := s.f.local(p)
	return q.X, q.Y
}

func infDomain() (u0, u1, v0, v1 float64) {
	return math.Inf(-1), math.Inf(1), math.Inf(-1), math.Inf(1)
}

// cone is also a cylinder, with tan zero: the point at (u, v) is at
// distance r + v tan from the axis, at height v.
type cone struct {
	f      frame
	r, tan float64
}

func (s *cone) Eval(u, v float64) geom.Vector {
	rho := s.r + v*s.tan
	return s.f.at(rho*math.Cos(u), rho*math.Sin(u), v)
}

func (s *cone) Derivs(u, v float64) (p, su, sv geom.Vector) {
	c, sn := math.Cos(u), math.Sin(u)
	rho := s.r + v*s.tan
	p = s.f.at(rho*c, rho*sn, v)
	su = s.f.x.ScaledBy(-rho * sn).Plus(s.f.y.ScaledBy(rho * c))
	sv = s.f.x.ScaledBy(s.tan * c).Plus(s.f.y.ScaledBy(s.tan * sn)).Plus(s.f.z)
	return
}

func (s *cone) Domain() (u0, u1, v0, v1 float64) {
	return 0, 2 * math.Pi, math.Inf(-1), math.Inf(1)
}

func (s *cone) Period() (pu, pv float64) { return 2 * math.Pi, 0 }

// Pole is the apex, on either side: the surface goes on through it,
// its radius negative beyond.
func (s *cone) Pole(side int) (float64, bool) {
	if (side == poleVMin || side == poleVMax) && s.tan != 0 {
		return -s.r / s.tan, true
	}
	return 0, false
}

func (s *cone) Project(p geom.Vector, _ *[2]float64) (u, v float64) {
	q := s.f.local(p)
	rho := math.Hypot(q.X, q.Y)
	// The nearest point of the line (r + v tan, v) to (rho, h), or of
	// its other half, beyond the apex, to (-rho, h).
	near := func(rho float64) (v, d float64) {
		v = ((rho-s.r)*s.tan + q.Z) / (1 + s.tan*s.tan)
		return v, math.Hypot(s.r+v*s.tan-rho, v-q.Z)
	}
	v, d := near(rho)
	u = angle(q.X, q.Y)
	if s.tan != 0 {
		if v2, d2 := near(-rho); d2 < d {
			v = v2
			u = angle(-q.X, -q.Y)
		}
	}
	return u, v
}

type sphere struct {
	f frame
	r float64
}

func (s *sphere) Eval(u, v float64) geom.Vector {
	cv := math.Cos(v)
	return s.f.at(s.r*cv*math.Cos(u), s.r*cv*math.Sin(u), s.r*math.Sin(v))
}

func (s *sphere) Derivs(u, v float64) (p, su, sv geom.Vector) {
	cu, snu := math.Cos(u), math.Sin(u)
	cv, snv := math.Cos(v), math.Sin(v)
	p = s.f.at(s.r*cv*cu, s.r*cv*snu, s.r*snv)
	su = s.f.x.ScaledBy(-s.r * cv * snu).Plus(s.f.y.ScaledBy(s.r * cv * cu))
	sv = s.f.x.ScaledBy(-s.r * snv * cu).Plus(s.f.y.ScaledBy(-s.r * snv * snu)).Plus(s.f.z.ScaledBy(s.r * cv))
	return
}

func (s *sphere) Domain() (u0, u1, v0, v1 float64) { return 0, 2 * math.Pi, -math.Pi / 2, math.Pi / 2 }
func (s *sphere) Period() (pu, pv float64)         { return 2 * math.Pi, 0 }

func (s *sphere) Pole(side int) (float64, bool) {
	switch side {
	case poleVMin:
		return -math.Pi / 2, true
	case poleVMax:
		return math.Pi / 2, true
	}
	return 0, false
}

func (s *sphere) Project(p geom.Vector, _ *[2]float64) (u, v float64) {
	q := s.f.local(p)
	u = angle(q.X, q.Y)
	v = math.Atan2(q.Z, math.Hypot(q.X, q.Y))
	return u, v
}

type torus struct {
	f        frame
	major, r float64
}

func (s *torus) Eval(u, v float64) geom.Vector {
	rho := s.major + s.r*math.Cos(v)
	return s.f.at(rho*math.Cos(u), rho*math.Sin(u), s.r*math.Sin(v))
}

func (s *torus) Derivs(u, v float64) (p, su, sv geom.Vector) {
	cu, snu := math.Cos(u), math.Sin(u)
	cv, snv := math.Cos(v), math.Sin(v)
	rho := s.major + s.r*cv
	p = s.f.at(rho*cu, rho*snu, s.r*snv)
	su = s.f.x.ScaledBy(-rho * snu).Plus(s.f.y.ScaledBy(rho * cu))
	sv = s.f.x.ScaledBy(-s.r * snv * cu).Plus(s.f.y.ScaledBy(-s.r * snv * snu)).Plus(s.f.z.ScaledBy(s.r * cv))
	return
}

func (s *torus) Domain() (u0, u1, v0, v1 float64) { return 0, 2 * math.Pi, 0, 2 * math.Pi }
func (s *torus) Period() (pu, pv float64)         { return 2 * math.Pi, 2 * math.Pi }
func (s *torus) Pole(int) (float64, bool)         { return 0, false }

func (s *torus) Project(p geom.Vector, _ *[2]float64) (u, v float64) {
	q := s.f.local(p)
	rho := math.Hypot(q.X, q.Y)
	u = angle(q.X, q.Y)
	v = angle(rho-s.major, q.Z)
	// Where the tube is wider than the hole, the surface also passes
	// through the axis: a point may lie on the meridian circle of the
	// opposite half plane, at u + pi.
	if s.r > s.major {
		d1 := math.Abs(math.Hypot(rho-s.major, q.Z) - s.r)
		d2 := math.Abs(math.Hypot(-rho-s.major, q.Z) - s.r)
		if d2 < d1 {
			u = angle(-q.X, -q.Y)
			v = angle(-rho-s.major, q.Z)
		}
	}
	return u, v
}

// extrusion is a SURFACE_OF_LINEAR_EXTRUSION: the curve at u, moved
// by v times d.
type extrusion struct {
	c Curve
	d geom.Vector
}

func (s *extrusion) Eval(u, v float64) geom.Vector { return s.c.Eval(u).Plus(s.d.ScaledBy(v)) }
func (s *extrusion) Derivs(u, v float64) (p, su, sv geom.Vector) {
	return s.Eval(u, v), s.c.Deriv(u), s.d
}
func (s *extrusion) Domain() (u0, u1, v0, v1 float64) {
	u0, u1 = s.c.Range()
	return u0, u1, math.Inf(-1), math.Inf(1)
}
func (s *extrusion) Period() (pu, pv float64) { return s.c.Period(), 0 }
func (s *extrusion) Pole(int) (float64, bool) { return 0, false }

func (s *extrusion) Project(p geom.Vector, guess *[2]float64) (u, v float64) {
	if guess != nil {
		u, v = newtonOnSurface(s, p, *guess)
		if s.Eval(u, v).Minus(p).Magnitude() < 1e-4+1e-6*p.Magnitude() {
			return u, v
		}
	}
	{
		// Along d the surface is straight: project the point onto the
		// curve as if it were moved back along d.
		c0 := s.c.Eval(0)
		if lo, hi := s.c.Range(); !math.IsInf(lo, 0) && !math.IsInf(hi, 0) {
			c0 = s.c.Eval((lo + hi) / 2)
		}
		back := p.Minus(s.d.ScaledBy(p.Minus(c0).Dot(s.d) / s.d.MagSquared()))
		u = s.c.Param(back)
		v = p.Minus(s.c.Eval(u)).Dot(s.d) / s.d.MagSquared()
		guess = &[2]float64{u, v}
	}
	return newtonOnSurface(s, p, *guess)
}

// revolution is a SURFACE_OF_REVOLUTION: the curve at v, turned by u
// about the axis.
type revolution struct {
	c    Curve
	o, a geom.Vector // a is a unit vector
}

func (s *revolution) Eval(u, v float64) geom.Vector {
	return s.c.Eval(v).RotatedAbout(s.o, s.a, u)
}

func (s *revolution) Derivs(u, v float64) (p, su, sv geom.Vector) {
	p = s.Eval(u, v)
	su = s.a.Cross(p.Minus(s.o))
	sv = s.c.Deriv(v).RotatedAbout(geom.Vector{}, s.a, u)
	return
}

func (s *revolution) Domain() (u0, u1, v0, v1 float64) {
	v0, v1 = s.c.Range()
	return 0, 2 * math.Pi, v0, v1
}

func (s *revolution) Period() (pu, pv float64) { return 2 * math.Pi, s.c.Period() }

func (s *revolution) Pole(side int) (float64, bool) {
	v0, v1 := s.c.Range()
	onAxis := func(v float64) bool {
		if math.IsInf(v, 0) {
			return false
		}
		q := s.c.Eval(v).Minus(s.o)
		return q.Minus(s.a.ScaledBy(q.Dot(s.a))).Magnitude() < 1e-9*(1+q.Magnitude())
	}
	switch side {
	case poleVMin:
		return v0, onAxis(v0)
	case poleVMax:
		return v1, onAxis(v1)
	}
	return 0, false
}

// angleAbout is the angle of p about the axis, measured from the
// direction of ref.
func (s *revolution) angleAbout(p, ref geom.Vector) float64 {
	x := ref.Minus(s.o)
	x = x.Minus(s.a.ScaledBy(x.Dot(s.a)))
	if x.MagSquared() == 0 {
		x = s.a.Normal()
	}
	x = x.WithMagnitude(1)
	y := s.a.Cross(x)
	q := p.Minus(s.o)
	return angle(q.Dot(x), q.Dot(y))
}

func (s *revolution) Project(p geom.Vector, guess *[2]float64) (u, v float64) {
	if guess != nil {
		u, v = newtonOnSurface(s, p, *guess)
		if s.Eval(u, v).Minus(p).Magnitude() < 1e-4+1e-6*p.Magnitude() {
			return wrap(u, 0, 2*math.Pi), v
		}
	}
	{
		// Turn the point back into the half plane of the curve.
		lo, hi := s.c.Range()
		mid := 0.0
		if !math.IsInf(lo, 0) && !math.IsInf(hi, 0) {
			mid = (lo + hi) / 2
		}
		ref := s.c.Eval(mid)
		th := s.angleAbout(p, ref)
		v = s.c.Param(p.RotatedAbout(s.o, s.a, -th))
		u = s.angleAbout(p, s.c.Eval(v))
		guess = &[2]float64{u, v}
	}
	u, v = newtonOnSurface(s, p, *guess)
	return wrap(u, 0, 2*math.Pi), v
}

// offset is an OFFSET_SURFACE: the basis surface moved along its
// normal by d. The point of it nearest p has the parameters of the
// point of the basis nearest p.
type offset struct {
	base Surface
	d    float64

	period     sync.Once
	perU, perV float64
}

func (s *offset) normal(u, v float64) (p, n geom.Vector) {
	p, su, sv := s.base.Derivs(u, v)
	n = su.Cross(sv)
	if m := n.Magnitude(); m > 0 {
		n = n.ScaledBy(1 / m)
	}
	return p, n
}

func (s *offset) Eval(u, v float64) geom.Vector {
	p, n := s.normal(u, v)
	return p.Plus(n.ScaledBy(s.d))
}

func (s *offset) Derivs(u, v float64) (p, su, sv geom.Vector) {
	p = s.Eval(u, v)
	const h = 1e-6
	su = s.Eval(u+h, v).Minus(s.Eval(u-h, v)).ScaledBy(1 / (2 * h))
	sv = s.Eval(u, v+h).Minus(s.Eval(u, v-h)).ScaledBy(1 / (2 * h))
	return
}

func (s *offset) Domain() (u0, u1, v0, v1 float64) { return s.base.Domain() }

// Period is the base's, along a way where the offset closes up too: a
// base that closes with a crease, as a closed outline with a corner
// where it starts, has an offset that doesn't (package 27.5).
func (s *offset) Period() (pu, pv float64) {
	s.period.Do(func() { s.perU, s.perV = s.closingPeriods() })
	return s.perU, s.perV
}

func (s *offset) closingPeriods() (pu, pv float64) {
	pu, pv = s.base.Period()
	u0, u1, v0, v1 := s.base.Domain()
	closes := func(at func(k int) (geom.Vector, geom.Vector)) bool {
		for k := 0; k <= 4; k++ {
			a, b := at(k)
			if a.Minus(b).Magnitude() > 1e-6*(1+math.Abs(s.d)) {
				return false
			}
		}
		return true
	}
	finite := func(lo, hi float64) (float64, float64) {
		if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
			return -1, 1
		}
		return lo, hi
	}
	if pu > 0 {
		a, b := finite(v0, v1)
		if !closes(func(k int) (geom.Vector, geom.Vector) {
			v := a + (b-a)*float64(k)/4
			return s.Eval(u0, v), s.Eval(u0+pu, v)
		}) {
			pu = 0
		}
	}
	if pv > 0 {
		a, b := finite(u0, u1)
		if !closes(func(k int) (geom.Vector, geom.Vector) {
			u := a + (b-a)*float64(k)/4
			return s.Eval(u, v0), s.Eval(u, v0+pv)
		}) {
			pv = 0
		}
	}
	return pu, pv
}
func (s *offset) Pole(side int) (float64, bool) { return s.base.Pole(side) }
func (s *offset) Project(p geom.Vector, guess *[2]float64) (u, v float64) {
	u, v = s.base.Project(p, guess)
	// The base's nearest point is the offset's foot only where the
	// offset is less than the base's radius of curvature; near a crease
	// or a tight bend, Newton's steps on the offset itself from there,
	// and from the guess, find it (package 27.5).
	best := math.Inf(1)
	try := func(a, b float64) {
		a, b = newtonOnSurface(s, p, [2]float64{a, b})
		if d := s.Eval(a, b).Minus(p).Magnitude(); d < best {
			best, u, v = d, a, b
		}
	}
	bu0, bv0 := u, v
	try(bu0, bv0)
	if guess != nil {
		try(guess[0], guess[1])
	}
	// Where the base closes with a crease, its parameter at the crease
	// is either end; the offset's is the end whose point is p's.
	bu, bv := s.base.Period()
	ou, ov := s.Period()
	u0, u1, v0, v1 := s.base.Domain()
	// Near either end, the nearer to p of the parameter found, the same
	// a period on or back, and the ends.
	pick := func(t, lo, hi, period float64, at func(float64) geom.Vector) float64 {
		if math.Min(math.Abs(t-lo), math.Abs(hi-t)) > 1e-3*period {
			return t
		}
		best, bd := t, at(t).Minus(p).Magnitude()
		for _, c := range []float64{lo, hi, t + period, t - period} {
			if c < lo || c > hi {
				continue
			}
			if d := at(c).Minus(p).Magnitude(); d < bd {
				best, bd = c, d
			}
		}
		return best
	}
	if bu > 0 && ou == 0 {
		u = pick(u, u0, u1, bu, func(t float64) geom.Vector { return s.Eval(t, v) })
	}
	if bv > 0 && ov == 0 {
		v = pick(v, v0, v1, bv, func(t float64) geom.Vector { return s.Eval(u, t) })
	}
	return u, v
}

// ---- projection onto a general surface ----

// newtonOnSurface improves (u, v), the parameters of the point of s
// nearest p, by Gauss-Newton steps, kept in the domain where the
// surface doesn't repeat.
func newtonOnSurface(s Surface, p geom.Vector, uv [2]float64) (u, v float64) {
	u, v = uv[0], uv[1]
	u0, u1, v0, v1 := s.Domain()
	pu, pv := s.Period()
	clamp := func(t, lo, hi, period float64) float64 {
		if period > 0 {
			return t
		}
		return math.Max(lo, math.Min(hi, t))
	}
	for i := 0; i < 40; i++ {
		q, su, sv := s.Derivs(u, v)
		r := p.Minus(q)
		a, b, c := su.Dot(su), su.Dot(sv), sv.Dot(sv)
		ru, rv := su.Dot(r), sv.Dot(r)
		det := a*c - b*b
		var du, dv float64
		switch {
		case det > 1e-12*(a*c) && det > 0:
			du = (c*ru - b*rv) / det
			dv = (a*rv - b*ru) / det
		case a > c && a > 0:
			du = ru / a
		case c > 0:
			dv = rv / c
		default:
			return wrap(u, u0, pu), wrap(v, v0, pv)
		}
		// Keep steps sane on strongly curved patches.
		if pu > 0 && math.Abs(du) > pu/4 {
			du = math.Copysign(pu/4, du)
		}
		if pv > 0 && math.Abs(dv) > pv/4 {
			dv = math.Copysign(pv/4, dv)
		}
		// Take the step, or a fraction of it, only if it brings the
		// point nearer: near a crease of the surface a full step can
		// jump to another part of it.
		d0 := r.MagSquared()
		var nu, nv float64
		improved := false
		for f := 1.0; f > 1e-3; f /= 2 {
			nu, nv = clamp(u+f*du, u0, u1, pu), clamp(v+f*dv, v0, v1, pv)
			if s.Eval(nu, nv).Minus(p).MagSquared() <= d0 {
				improved = true
				break
			}
		}
		if !improved {
			break
		}
		done := math.Abs(nu-u) <= 1e-13*(1+math.Abs(u)) && math.Abs(nv-v) <= 1e-13*(1+math.Abs(v))
		u, v = nu, nv
		if done {
			break
		}
	}
	return wrap(u, u0, pu), wrap(v, v0, pv)
}

// sampled is a surface with a grid of samples for finding where to
// start a projection.
type sampled interface {
	Surface
	samples() *surfaceGrid
}

// surfaceGrid is a grid of points of a surface with finite domain.
type surfaceGrid struct {
	us, vs []float64
	pts    []geom.Vector
}

func newSurfaceGrid(s Surface) *surfaceGrid {
	g := &surfaceGrid{}
	u0, u1, v0, v1 := s.Domain()
	var bu, bv []float64
	if b, ok := s.(interface{ Breaks() (us, vs []float64) }); ok {
		bu, bv = b.Breaks()
	}
	// At least two samples per knot span, so a surface that winds many
	// times is sampled on every turn; at most 512 along a direction.
	fill := func(br []float64, lo, hi float64) []float64 {
		if len(br) < 2 {
			br = []float64{lo, hi}
		}
		spans := len(br) - 1
		per := max(2, 16/spans)
		for spans*per > 512 && per > 1 {
			per--
		}
		step := 1
		if spans > 512 {
			step = (spans + 511) / 512
		}
		var out []float64
		for i := 0; i+step < len(br); i += step {
			for k := 0; k < per; k++ {
				out = append(out, br[i]+(br[i+step]-br[i])*float64(k)/float64(per))
			}
		}
		return append(out, hi)
	}
	g.us = fill(bu, u0, u1)
	g.vs = fill(bv, v0, v1)
	g.pts = make([]geom.Vector, 0, len(g.us)*len(g.vs))
	for _, u := range g.us {
		for _, v := range g.vs {
			g.pts = append(g.pts, s.Eval(u, v))
		}
	}
	return g
}

// nearest is the parameters of the k grid points nearest p, nearest
// first.
func (g *surfaceGrid) nearest(p geom.Vector, k int) [][2]float64 {
	type cand struct {
		i int
		d float64
	}
	best := make([]cand, 0, k+1)
	for i, q := range g.pts {
		d := q.Minus(p).MagSquared()
		if len(best) == k && d >= best[k-1].d {
			continue
		}
		j := len(best)
		best = append(best, cand{i, d})
		for j > 0 && best[j-1].d > d {
			best[j] = best[j-1]
			j--
		}
		best[j] = cand{i, d}
		if len(best) > k {
			best = best[:k]
		}
	}
	out := make([][2]float64, len(best))
	for n, c := range best {
		out[n] = [2]float64{g.us[c.i/len(g.vs)], g.vs[c.i%len(g.vs)]}
	}
	return out
}

// projectOnSurface is the parameters of the point of s nearest p,
// starting from guess when it gives a near point, else from the
// nearest sample.
func projectOnSurface(s sampled, p geom.Vector, guess *[2]float64) (u, v float64) {
	bestD := math.Inf(1)
	if guess != nil {
		u, v = newtonOnSurface(s, p, *guess)
		bestD = s.Eval(u, v).Minus(p).Magnitude()
		if bestD < 1e-4+1e-6*p.Magnitude() {
			return u, v
		}
	}
	for _, start := range s.samples().nearest(p, 4) {
		cu, cv := newtonOnSurface(s, p, start)
		if d := s.Eval(cu, cv).Minus(p).Magnitude(); d < bestD-1e-12 {
			u, v, bestD = cu, cv, d
		}
	}
	return u, v
}
