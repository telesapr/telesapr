package topo

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// A curved next face that the end section lies wholly past, carried on
// as its own polynomial, whose edge with side j's face is curved, as a
// ring-shaped band's top on a cone (a torus, whose edge with the band's
// side is a circle): the tangent plane of rollingrunoff_tangent.go
// would leave the edge, and the face could not share it. The top is
// carried on instead by its tangents across the edge, square to it:
// the surface ruled by them, E(t) + s T(t), E the edge from w to its
// far end and T the top's unit tangent there pointing past side j. It
// meets the top along the whole edge, smoothly, and the end runs onto
// it as onto any curved next face: a new face of the top's sheet,
// beside the top as the tangent plane's face is, its outline the edge
// from w to p2, the fillet's cut and the curve where the end face meets
// it from p1 back to w.
//
// E is the edge's own exact curve, so that the new face shares it with
// the top; T is fitted, its control vectors such that the ruling at
// each of degree+1 points along E is the top's tangent there, and so is
// square to the edge and in the top's tangent plane only at those
// points, near it between: the face meets the top smoothly within that.

// rbRuled is the surface a curved next face is carried on by along a
// curved edge, s, and the edge, e, from w on, that it is ruled from.
type rbRuled struct {
	s srf.SSurface
	e srf.SBezier
}

// along is the edge from w to p, which lies on it: its own curve, as
// the top's trim holds it.
func (rd *rbRuled) along(p geom.Vector) srf.SBezier {
	tp, _ := rd.e.ClosestPointTo(p, false)
	sb := rd.e.Sub(0, tp)
	sb.Ctrl[0], sb.Ctrl[sb.Degree] = rd.e.Start(), p
	return sb
}

// ruledLength is how far the rulings reach past the edge, in the
// blend's sizes: past where the end section and the fillet's cut cross
// the surface, which runOffExt carries on past them once more.
const ruledLength = 4.0

// ruledIfPast replaces ro's curved next face by the surface its
// tangents across its curved edge with side j's face rule, where the
// end section sec lies wholly past the face carried on but crosses
// that surface, with v at the end's vertex under it.
func (rb *rollingBall) ruledIfPast(sec srf.SBezier, v geom.Vector, ro *rbRunOff) {
	rd, ok := rb.ruledAlong(ro)
	if !ok {
		return
	}
	cand := &rbRunOff{ns: &rd.s, sign: 1}
	if cand.h(v) > 0 {
		cand.sign = -1
	}
	if !crossesOut(cand, ro.j, sec) || cand.h(v) >= 0 {
		return
	}
	ro.ns, ro.sign, ro.tangent, ro.ruled = cand.ns, cand.sign, true, rd
}

// ruledAlong is the surface ro's curved next face is carried on by
// along its edge with side j's face, as the package comment above
// says; false where the edge has no exact curve or the top's tangent
// across it can't be found.
func (rb *rollingBall) ruledAlong(ro *rbRunOff) (*rbRuled, bool) {
	t := rb.t
	ed := &t.Edges[ro.sideN]
	c := t.shell.FindCurve(ed.Curve)
	if c == nil || !c.IsExact || len(ed.Points) < 2 {
		return nil, false
	}
	w := t.Vertices[ro.w].P
	far := ed.Points[0]
	if far.Equals(w) {
		far = ed.Points[len(ed.Points)-1]
	}
	ex := c.Exact
	t0, _ := ex.ClosestPointTo(w, false)
	t1, _ := ex.ClosestPointTo(far, false)
	if math.Abs(t1-t0) < 1e-9 {
		return nil, false
	}
	e := ex.Sub(math.Min(t0, t1), math.Max(t0, t1))
	if t0 > t1 {
		e.Reverse()
	}
	e.Ctrl[0], e.Ctrl[e.Degree] = w, far
	// Into the top from w, along its edge with the end face: the
	// tangents point the other way.
	in := t.Edges[ro.endN].Points
	ref := in[1].Minus(w)
	if !in[0].Equals(w) {
		ref = in[len(in)-2].Minus(w)
	}
	ref = ref.ScaledBy(-1)
	n := e.Degree
	tans := make([]geom.Vector, n+1)
	for k := range n + 1 {
		x := float64(k) / float64(n)
		p := e.PointAt(x)
		u, vv, _ := nearestOnWithin(ro.ns, p, 0)
		d := e.TangentAt(x).Cross(ro.ns.NormalAt(u, vv))
		if d.Magnitude() < 1e-12 {
			return nil, false
		}
		d = d.WithMagnitude(1)
		if d.Dot(ref) < 0 {
			d = d.ScaledBy(-1)
		}
		tans[k], ref = d, d
	}
	ctrl, ok := ruledControls(&e, tans)
	if !ok {
		return nil, false
	}
	l := ruledLength * rb.r
	rd := &rbRuled{e: e}
	s := &rd.s
	s.DegM, s.DegN, s.Tol = n, 1, t.shell.Tol
	for i := range n + 1 {
		s.Ctrl[i][0], s.Ctrl[i][1] = e.Ctrl[i], e.Ctrl[i].Plus(ctrl[i].ScaledBy(l))
		s.Weight[i][0], s.Weight[i][1] = e.Weight[i], e.Weight[i]
	}
	return rd, true
}

// ruledControls is the control vectors V of a field along e, weighted
// as e is, that is tans[k] at k/n: sum w_i B_i V_i / sum w_i B_i there,
// solved by elimination.
func ruledControls(e *srf.SBezier, tans []geom.Vector) ([]geom.Vector, bool) {
	n := e.Degree
	a := make([][]float64, n+1)
	for k := range n + 1 {
		x := float64(k) / float64(n)
		a[k] = make([]float64, n+1)
		den := 0.0
		for i := range n + 1 {
			a[k][i] = e.Weight[i] * bernstein(n, i, x)
			den += a[k][i]
		}
		for i := range n + 1 {
			a[k][i] /= den
		}
	}
	b := append([]geom.Vector(nil), tans...)
	for col := range n + 1 {
		piv := col
		for r := col + 1; r <= n; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[piv][col]) {
				piv = r
			}
		}
		if math.Abs(a[piv][col]) < 1e-12 {
			return nil, false
		}
		a[col], a[piv] = a[piv], a[col]
		b[col], b[piv] = b[piv], b[col]
		for r := range n + 1 {
			if r == col {
				continue
			}
			f := a[r][col] / a[col][col]
			for i := col; i <= n; i++ {
				a[r][i] -= f * a[col][i]
			}
			b[r] = b[r].Minus(b[col].ScaledBy(f))
		}
	}
	out := make([]geom.Vector, n+1)
	for i := range n + 1 {
		out[i] = b[i].ScaledBy(1 / a[i][i])
	}
	return out, true
}
