package srf

import (
	"container/heap"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Projection is where a point projects onto a surface: the parameters
// of the surface's point nearest it, that point, and how far it is.
// Stationary reports that the steps stopped where the point's offset
// is square to the surface, or runs out of the domain at its edge: a
// nearest point, if not always the nearest.
type Projection struct {
	U, V       float64
	P          geom.Vector
	Dist       float64
	Stationary bool
}

// ProjectDomain is the (u, v) range a projection keeps to. A direction
// with a period wraps instead of stopping at its ends.
type ProjectDomain struct {
	U0, U1, V0, V1 float64
	PU, PV         float64
}

// Derivs evaluates a surface and its two first derivatives at (u, v).
type Derivs func(u, v float64) (p, su, sv geom.Vector)

// projectMaxSteps bounds the Gauss-Newton steps from one seed. A step
// gains a factor of about the distance over the radius of curvature, so
// a point far off a tight curve takes many.
const projectMaxSteps = 200

// projectHalvings is how many times a step is halved looking for one
// that brings the point nearer.
const projectHalvings = 12

// ProjectFrom is the point of the surface nearest p, by damped
// Gauss-Newton steps from each seed in turn, best first: the step solves
// the 2x2 normal equations, is capped to a quarter of the domain in
// each direction, and is halved until it brings the point nearer, so
// that it never climbs away from a minimum or jumps across a crease.
// Directions with a period are evaluated wrapped, others clamped to the
// domain. The nearest result is kept, and the seeds stop at the first
// stationary one within tol of p, or the first stationary one where
// tol is 0.
func ProjectFrom(f Derivs, dom ProjectDomain, p geom.Vector, tol float64, seeds ...geom.Point2d) Projection {
	best := Projection{Dist: math.Inf(1)}
	for _, s := range seeds {
		r := gaussNewton(f, dom, p, s.U, s.V)
		if r.Dist < best.Dist || r.Stationary && !best.Stationary && r.Dist <= best.Dist+geom.LengthEps {
			best = r
		}
		if best.Stationary && (tol == 0 || best.Dist <= tol) {
			break
		}
	}
	return best
}

// gaussNewton runs the damped steps from (u, v).
func gaussNewton(f Derivs, dom ProjectDomain, p geom.Vector, u, v float64) Projection {
	clampTo := func(t, lo, hi, period float64) float64 {
		if period > 0 {
			return t
		}
		return math.Max(lo, math.Min(hi, t))
	}
	u, v = clampTo(u, dom.U0, dom.U1, dom.PU), clampTo(v, dom.V0, dom.V1, dom.PV)
	capU, capV := (dom.U1-dom.U0)/4, (dom.V1-dom.V0)/4
	if dom.PU > 0 {
		capU = dom.PU / 4
	}
	if dom.PV > 0 {
		capV = dom.PV / 4
	}
	q, su, sv := f(u, v)
	d2 := p.Minus(q).MagSquared()
	stationary := false
	for range projectMaxSteps {
		r := p.Minus(q)
		a, b, c := su.Dot(su), su.Dot(sv), sv.Dot(sv)
		ru, rv := su.Dot(r), sv.Dot(r)
		// At an edge of the domain, a direction that would leave it is
		// held: what is left of the gradient decides.
		if dom.PU == 0 && (u <= dom.U0 && ru < 0 || u >= dom.U1 && ru > 0) {
			ru, a, b = 0, 1, 0
		}
		if dom.PV == 0 && (v <= dom.V0 && rv < 0 || v >= dom.V1 && rv > 0) {
			rv, c, b = 0, 1, 0
		}
		rm := math.Sqrt(d2)
		if a == 0 && c == 0 {
			// No tangents: nowhere to step.
			break
		}
		if rm == 0 || math.Abs(ru) <= 1e-12*rm*math.Sqrt(a) && math.Abs(rv) <= 1e-12*rm*math.Sqrt(c) {
			stationary = true
			break
		}
		var du, dv float64
		det := a*c - b*b
		// Gauss-Newton gains about the distance over the radius of
		// curvature a step, slow far off a tight curve: where the full
		// Hessian, with the second derivatives the offset weighs, is
		// positive definite, its Newton step is taken instead. The
		// second derivatives are differences of the tangents.
		ha, hb, hc, newton := hessian(f, dom, u, v, r, a, b, c)
		switch {
		case newton:
			du = (hc*ru - hb*rv) / (ha*hc - hb*hb)
			dv = (ha*rv - hb*ru) / (ha*hc - hb*hb)
			if dom.PU == 0 && ru == 0 && (u <= dom.U0 || u >= dom.U1) {
				du = 0
			}
			if dom.PV == 0 && rv == 0 && (v <= dom.V0 || v >= dom.V1) {
				dv = 0
			}
		case det > 1e-12*a*c && det > 0:
			du = (c*ru - b*rv) / det
			dv = (a*rv - b*ru) / det
		case a >= c && a > 0:
			// A degenerate direction, as at a pole: the other alone.
			du = ru / a
		case c > 0:
			dv = rv / c
		default:
			return Projection{U: wrap(u, dom.U0, dom.PU), V: wrap(v, dom.V0, dom.PV), P: q, Dist: rm}
		}
		if math.Abs(du) > capU {
			du = math.Copysign(capU, du)
		}
		if math.Abs(dv) > capV {
			dv = math.Copysign(capV, dv)
		}
		improved := false
		var nu, nv, nd2 float64
		var nq, nsu, nsv geom.Vector
		for h, step := 0, 1.0; h <= projectHalvings; h, step = h+1, step/2 {
			nu, nv = clampTo(u+step*du, dom.U0, dom.U1, dom.PU), clampTo(v+step*dv, dom.V0, dom.V1, dom.PV)
			nq, nsu, nsv = f(nu, nv)
			// Near the foot the distance is flat to second order, and
			// a step along the surface that brings the foot nearer
			// changes it less than rounding: a step no further within
			// rounding is taken, and the gradient says when to stop.
			if nd2 = p.Minus(nq).MagSquared(); nd2 <= d2*(1+1e-14) {
				improved = true
				break
			}
		}
		if !improved {
			// No step nearer: as near as the arithmetic gets.
			stationary = true
			break
		}
		moved := math.Abs(nu-u) <= 1e-15*(1+math.Abs(u)) && math.Abs(nv-v) <= 1e-15*(1+math.Abs(v))
		u, v, q, su, sv, d2 = nu, nv, nq, nsu, nsv, nd2
		if moved {
			stationary = true
			break
		}
	}
	return Projection{U: wrap(u, dom.U0, dom.PU), V: wrap(v, dom.V0, dom.PV), P: q, Dist: math.Sqrt(d2), Stationary: stationary}
}

// hessian is the Hessian of half the squared distance at (u, v), where
// the offset is r and the tangents' products a, b, c, and whether it
// is positive definite. The second derivatives are central differences
// of the tangents.
func hessian(f Derivs, dom ProjectDomain, u, v float64, r geom.Vector, a, b, c float64) (ha, hb, hc float64, ok bool) {
	step := func(t, t0, t1, period float64) float64 {
		span := t1 - t0
		if period > 0 {
			span = period
		}
		if math.IsInf(span, 0) || span <= 0 {
			// An unbounded direction, as along a cylinder's axis.
			span = 1 + math.Abs(t)
		}
		return 1e-5 * span
	}
	hu, hv := step(u, dom.U0, dom.U1, dom.PU), step(v, dom.V0, dom.V1, dom.PV)
	_, suP, svP := f(u+hu, v)
	_, suM, svM := f(u-hu, v)
	_, _, svVP := f(u, v+hv)
	_, _, svVM := f(u, v-hv)
	suu := suP.Minus(suM).ScaledBy(1 / (2 * hu))
	suv := svP.Minus(svM).ScaledBy(1 / (2 * hu))
	svv := svVP.Minus(svVM).ScaledBy(1 / (2 * hv))
	ha, hb, hc = a-r.Dot(suu), b-r.Dot(suv), c-r.Dot(svv)
	return ha, hb, hc, ha > 0 && hc > 0 && ha*hc-hb*hb > 1e-12*ha*hc
}

// wrap brings t into the period from t0.
func wrap(t, t0, period float64) float64 {
	if period <= 0 {
		return t
	}
	t = math.Mod(t-t0, period)
	if t < 0 {
		t += period
	}
	return t0 + t
}

// derivs evaluates the patch for the projection.
func (s *SSurface) derivs(u, v float64) (p, su, sv geom.Vector) {
	s.TangentsAt(u, v, &su, &sv, true)
	return s.PointAt(u, v), su, sv
}

// Project is the point of the patch nearest p, within its (u, v)
// square carried on by ext past each side. It starts from the kernel's
// own guess (ClosestPointToUV), whose first-order steps stop short of
// a point off the surface (2 mm off a cylinder, 6 µm short), and
// steps on from there (ProjectFrom). Where those steps find no
// stationary point, it tries three seeds a direction, then searches the
// patch by halves (projectBranchAndBound).
func (s *SSurface) Project(p geom.Vector, ext float64) Projection {
	dom := ProjectDomain{U0: -ext, U1: 1 + ext, V0: -ext, V1: 1 + ext}
	var u, v float64
	s.ClosestPointToUV(p, &u, &v, false)
	r := ProjectFrom(s.derivs, dom, p, 0, geom.Point2d{U: u, V: v})
	if r.Stationary {
		return r
	}
	var seeds []geom.Point2d
	for _, su := range []float64{1.0 / 6, 0.5, 5.0 / 6} {
		for _, sv := range []float64{1.0 / 6, 0.5, 5.0 / 6} {
			seeds = append(seeds, geom.Point2d{U: su, V: sv})
		}
	}
	if q := ProjectFrom(s.derivs, dom, p, 0, seeds...); q.Dist < r.Dist || q.Stationary && !r.Stationary {
		r = q
	}
	if r.Stationary {
		return r
	}
	if q := s.projectBranchAndBound(p, dom); q.Dist < r.Dist || q.Stationary && !r.Stationary {
		r = q
	}
	return r
}

// bbPatch is a piece of the patch the search holds, with the least
// distance from the point its control points' box allows.
type bbPatch struct {
	u0, u1, v0, v1 float64
	lower          float64
}

type bbQueue []bbPatch

func (q bbQueue) Len() int           { return len(q) }
func (q bbQueue) Less(i, j int) bool { return q[i].lower < q[j].lower }
func (q bbQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *bbQueue) Push(x any)        { *q = append(*q, x.(bbPatch)) }
func (q *bbQueue) Pop() any {
	old := *q
	x := old[len(old)-1]
	*q = old[:len(old)-1]
	return x
}

// projectBranchAndBound searches the patch for the piece nearest p: a
// rational patch with positive weights lies in its control points'
// box, so a piece whose box is further than the nearest point found
// so far is dropped, and the rest are halved, nearest first, until
// pieces are small; the nearest piece's middle seeds the last steps.
func (s *SSurface) projectBranchAndBound(p geom.Vector, dom ProjectDomain) Projection {
	lower := func(u0, u1, v0, v1 float64) float64 {
		sub := s.Sub(u0, u1, v0, v1)
		lo := sub.Ctrl[0][0]
		hi := lo
		for i := 0; i <= sub.DegM; i++ {
			for j := 0; j <= sub.DegN; j++ {
				sub.Ctrl[i][j].MakeMaxMin(&hi, &lo)
			}
		}
		out := geom.Vector{
			X: math.Max(0, math.Max(lo.X-p.X, p.X-hi.X)),
			Y: math.Max(0, math.Max(lo.Y-p.Y, p.Y-hi.Y)),
			Z: math.Max(0, math.Max(lo.Z-p.Z, p.Z-hi.Z)),
		}
		return out.Magnitude()
	}
	upper := math.Inf(1)
	var seed geom.Point2d
	q := &bbQueue{{0, 1, 0, 1, lower(0, 1, 0, 1)}}
	for n := 0; q.Len() > 0 && n < 2000; n++ {
		b := heap.Pop(q).(bbPatch)
		if b.lower >= upper {
			break
		}
		um, vm := (b.u0+b.u1)/2, (b.v0+b.v1)/2
		if d := s.PointAt(um, vm).Minus(p).Magnitude(); d < upper {
			upper, seed = d, geom.Point2d{U: um, V: vm}
		}
		if b.u1-b.u0 < 1.0/1024 {
			continue
		}
		for _, c := range [4][4]float64{{b.u0, um, b.v0, vm}, {um, b.u1, b.v0, vm}, {b.u0, um, vm, b.v1}, {um, b.u1, vm, b.v1}} {
			if l := lower(c[0], c[1], c[2], c[3]); l < upper {
				heap.Push(q, bbPatch{c[0], c[1], c[2], c[3], l})
			}
		}
	}
	return ProjectFrom(s.derivs, dom, p, 0, seed)
}
