package step

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// basis is a B-spline basis: a degree and a knot vector with every
// knot repeated by its multiplicity. Its parameter range is
// [knots[deg], knots[len-1-deg]].
type basis struct {
	deg   int
	knots []float64
}

// newBasis checks a degree, its knot vector and the number of control
// points it has to serve.
func newBasis(deg int, knots []float64, n int) (basis, error) {
	if deg < 1 || deg > 25 {
		return basis{}, fmt.Errorf("degree %d is out of range", deg)
	}
	if n < deg+1 {
		return basis{}, fmt.Errorf("%d control points are too few for degree %d", n, deg)
	}
	if len(knots) != n+deg+1 {
		return basis{}, fmt.Errorf("%d knots don't fit %d control points of degree %d", len(knots), n, deg)
	}
	for i := 1; i < len(knots); i++ {
		if !(knots[i] >= knots[i-1]) || math.IsInf(knots[i], 0) {
			return basis{}, fmt.Errorf("the knots decrease")
		}
	}
	b := basis{deg: deg, knots: knots}
	if lo, hi := b.rng(); !(hi > lo) {
		return basis{}, fmt.Errorf("the knots span no range")
	}
	return b, nil
}

// expandKnots repeats each knot by its multiplicity.
func expandKnots(mults []int, knots []float64) ([]float64, error) {
	if len(mults) != len(knots) {
		return nil, fmt.Errorf("%d multiplicities for %d knots", len(mults), len(knots))
	}
	total := 0
	for _, m := range mults {
		if m < 1 || m > 64 {
			return nil, fmt.Errorf("knot multiplicity %d is out of range", m)
		}
		total += m
	}
	if total > 1<<20 {
		return nil, fmt.Errorf("too many knots")
	}
	out := make([]float64, 0, total)
	for i, m := range mults {
		for range m {
			out = append(out, knots[i])
		}
	}
	return out, nil
}

// uniformKnots is the knot vector of a uniform (or, clamped, a quasi
// uniform) B-spline with n control points, and of a Bezier curve when
// n is deg+1 and it is clamped.
func uniformKnots(deg, n int, clamped bool) []float64 {
	k := make([]float64, n+deg+1)
	for i := range k {
		switch {
		case !clamped:
			k[i] = float64(i - deg)
		case i <= deg:
			k[i] = 0
		case i >= n:
			k[i] = float64(n - deg)
		default:
			k[i] = float64(i - deg)
		}
	}
	return k
}

func (b basis) rng() (lo, hi float64) {
	return b.knots[b.deg], b.knots[len(b.knots)-1-b.deg]
}

// spans is the number of knot spans of non-zero length in the range.
func (b basis) spans() int {
	n := 0
	for i := b.deg; i < len(b.knots)-1-b.deg; i++ {
		if b.knots[i+1] > b.knots[i] {
			n++
		}
	}
	return n
}

// breaks are the distinct knots inside the range, ends included.
func (b basis) breaks() []float64 {
	var out []float64
	for i := b.deg; i <= len(b.knots)-1-b.deg; i++ {
		if len(out) == 0 || b.knots[i] > out[len(out)-1] {
			out = append(out, b.knots[i])
		}
	}
	return out
}

// span is the index i with knots[i] <= t < knots[i+1] inside the
// range, t clamped into it (The NURBS Book, A2.1).
func (b basis) span(t float64) int {
	n := len(b.knots) - b.deg - 1 // number of control points
	lo, hi := b.rng()
	if !(t > lo) {
		t = lo
	}
	if t >= hi {
		// The last span of non-zero length.
		i := n - 1
		for i > b.deg && b.knots[i] == b.knots[i+1] {
			i--
		}
		return i
	}
	// Largest i in [deg, n-1] with knots[i] <= t.
	i := sort.Search(n-b.deg, func(k int) bool { return b.knots[b.deg+k+1] > t })
	return b.deg + i
}

// ders fills ders[k][j] with the k-th derivative of the basis function
// span-deg+j at t, for k up to nd (A2.3). ders must be (nd+1) by deg+1.
func (b basis) ders(span int, t float64, nd int, ders [][]float64) {
	p := b.deg
	U := b.knots
	ndu := make([][]float64, p+1)
	for i := range ndu {
		ndu[i] = make([]float64, p+1)
	}
	left := make([]float64, p+1)
	right := make([]float64, p+1)
	ndu[0][0] = 1
	for j := 1; j <= p; j++ {
		left[j] = t - U[span+1-j]
		right[j] = U[span+j] - t
		saved := 0.0
		for r := 0; r < j; r++ {
			ndu[j][r] = right[r+1] + left[j-r]
			var temp float64
			if ndu[j][r] != 0 {
				temp = ndu[r][j-1] / ndu[j][r]
			}
			ndu[r][j] = saved + right[r+1]*temp
			saved = left[j-r] * temp
		}
		ndu[j][j] = saved
	}
	for j := 0; j <= p; j++ {
		ders[0][j] = ndu[j][p]
	}
	if nd == 0 {
		return
	}
	a := [2][]float64{make([]float64, p+1), make([]float64, p+1)}
	for r := 0; r <= p; r++ {
		s1, s2 := 0, 1
		a[0][0] = 1
		for k := 1; k <= nd; k++ {
			d := 0.0
			rk, pk := r-k, p-k
			if r >= k {
				if ndu[pk+1][rk] != 0 {
					a[s2][0] = a[s1][0] / ndu[pk+1][rk]
				} else {
					a[s2][0] = 0
				}
				d = a[s2][0] * ndu[rk][pk]
			}
			j1, j2 := 1, k-1
			if rk < -1 {
				j1 = -rk
			}
			if r-1 > pk {
				j2 = p - r
			}
			for j := j1; j <= j2; j++ {
				if ndu[pk+1][rk+j] != 0 {
					a[s2][j] = (a[s1][j] - a[s1][j-1]) / ndu[pk+1][rk+j]
				} else {
					a[s2][j] = 0
				}
				d += a[s2][j] * ndu[rk+j][pk]
			}
			if r <= pk {
				if ndu[pk+1][r] != 0 {
					a[s2][k] = -a[s1][k-1] / ndu[pk+1][r]
				} else {
					a[s2][k] = 0
				}
				d += a[s2][k] * ndu[r][pk]
			}
			ders[k][r] = d
			s1, s2 = s2, s1
		}
	}
	r := float64(p)
	for k := 1; k <= nd; k++ {
		for j := 0; j <= p; j++ {
			ders[k][j] *= r
		}
		r *= float64(p - k)
	}
}

func newDers(nd, p int) [][]float64 {
	d := make([][]float64, nd+1)
	for i := range d {
		d[i] = make([]float64, p+1)
	}
	return d
}

// hpoint is a control point in homogeneous form: the point times its
// weight, and the weight.
type hpoint struct {
	x, y, z, w float64
}

func homogeneous(p geom.Vector, w float64) hpoint {
	return hpoint{p.X * w, p.Y * w, p.Z * w, w}
}

// bsplineCurve is a (rational) B-spline curve.
type bsplineCurve struct {
	b    basis
	ctrl []hpoint
	// closed is set when the curve ends where it starts.
	closed bool
	// low is the curve as the kernel's, of degree at most srf.MaxDegree,
	// made on first use for the solid importer.
	low lowCurve
}

type lowCurve struct {
	once sync.Once
	c    *srf.BSplineCurve
	err  error
}

func newBsplineCurve(deg int, knots []float64, pts []geom.Vector, weights []float64) (*bsplineCurve, error) {
	b, err := newBasis(deg, knots, len(pts))
	if err != nil {
		return nil, err
	}
	if weights != nil && len(weights) != len(pts) {
		return nil, fmt.Errorf("%d weights for %d control points", len(weights), len(pts))
	}
	c := &bsplineCurve{b: b, ctrl: make([]hpoint, len(pts))}
	for i, p := range pts {
		w := 1.0
		if weights != nil {
			w = weights[i]
			if !(w > 0) {
				return nil, fmt.Errorf("weight %g is not positive", w)
			}
		}
		c.ctrl[i] = homogeneous(p, w)
	}
	lo, hi := b.rng()
	c.closed = c.Eval(lo).Minus(c.Eval(hi)).Magnitude() < 1e-9*(1+c.size())
	return c, nil
}

// size is the extent of the control polygon, for tolerances.
func (c *bsplineCurve) size() float64 {
	var lo, hi geom.Vector
	for i, h := range c.ctrl {
		p := geom.Vector{X: h.x / h.w, Y: h.y / h.w, Z: h.z / h.w}
		if i == 0 {
			lo, hi = p, p
		}
		growBox(p, &lo, &hi)
	}
	return hi.Minus(lo).Magnitude()
}

func (c *bsplineCurve) derivs(t float64) (p, d geom.Vector) {
	if c.closed {
		lo, hi := c.b.rng()
		t = wrap(t, lo, hi-lo)
	}
	span := c.b.span(t)
	ders := newDers(1, c.b.deg)
	c.b.ders(span, t, 1, ders)
	var a, ad hpoint
	for j := 0; j <= c.b.deg; j++ {
		h := c.ctrl[span-c.b.deg+j]
		n0, n1 := ders[0][j], ders[1][j]
		a.x += n0 * h.x
		a.y += n0 * h.y
		a.z += n0 * h.z
		a.w += n0 * h.w
		ad.x += n1 * h.x
		ad.y += n1 * h.y
		ad.z += n1 * h.z
		ad.w += n1 * h.w
	}
	p = geom.Vector{X: a.x / a.w, Y: a.y / a.w, Z: a.z / a.w}
	d = geom.Vector{X: (ad.x - ad.w*p.X) / a.w, Y: (ad.y - ad.w*p.Y) / a.w, Z: (ad.z - ad.w*p.Z) / a.w}
	return p, d
}

func (c *bsplineCurve) Eval(t float64) geom.Vector {
	p, _ := c.derivs(t)
	return p
}

func (c *bsplineCurve) Deriv(t float64) geom.Vector {
	_, d := c.derivs(t)
	return d
}

func (c *bsplineCurve) Range() (lo, hi float64) { return c.b.rng() }

func (c *bsplineCurve) Period() float64 {
	if c.closed {
		lo, hi := c.b.rng()
		return hi - lo
	}
	return 0
}

func (c *bsplineCurve) Breaks() []float64 { return c.b.breaks() }

func (c *bsplineCurve) Param(p geom.Vector) float64 { return projectOnCurve(c, p) }

// bsplineSurface is a (rational) B-spline surface. ctrl[i][j] is the
// control point i along u and j along v.
type bsplineSurface struct {
	bu, bv basis
	ctrl   [][]hpoint
	// periodU and periodV are the ranges in which the surface closes
	// on itself, or zero.
	periodU, periodV float64
	// poles say which sides of the domain collapse to a point:
	// u = lo, u = hi, v = lo, v = hi.
	poles [4]bool
	// grid is made on first use; faces that share the surface may be
	// tessellated side by side.
	gridOnce sync.Once
	grid     *surfaceGrid
}

func newBsplineSurface(du, dv int, ku, kv []float64, pts [][]geom.Vector, weights [][]float64) (*bsplineSurface, error) {
	if len(pts) == 0 || len(pts[0]) == 0 {
		return nil, fmt.Errorf("no control points")
	}
	nu, nv := len(pts), len(pts[0])
	for _, row := range pts {
		if len(row) != nv {
			return nil, fmt.Errorf("the rows of control points differ in length")
		}
	}
	if nu*nv > 1<<20 {
		return nil, fmt.Errorf("too many control points")
	}
	bu, err := newBasis(du, ku, nu)
	if err != nil {
		return nil, fmt.Errorf("along u: %w", err)
	}
	bv, err := newBasis(dv, kv, nv)
	if err != nil {
		return nil, fmt.Errorf("along v: %w", err)
	}
	if weights != nil {
		if len(weights) != nu {
			return nil, fmt.Errorf("the weights don't match the control points")
		}
		for _, row := range weights {
			if len(row) != nv {
				return nil, fmt.Errorf("the weights don't match the control points")
			}
		}
	}
	s := &bsplineSurface{bu: bu, bv: bv, ctrl: make([][]hpoint, nu)}
	for i := range pts {
		s.ctrl[i] = make([]hpoint, nv)
		for j, p := range pts[i] {
			w := 1.0
			if weights != nil {
				w = weights[i][j]
				if !(w > 0) {
					return nil, fmt.Errorf("weight %g is not positive", w)
				}
			}
			s.ctrl[i][j] = homogeneous(p, w)
		}
	}
	s.analyse()
	return s, nil
}

// analyse finds the directions in which the surface closes, and the
// sides that collapse to a point.
func (s *bsplineSurface) analyse() {
	u0, u1, v0, v1 := s.Domain()
	size := s.size()
	tol := 1e-7 * (1 + size)
	const n = 7
	closedU, closedV := true, true
	for k := 0; k <= n; k++ {
		f := float64(k) / n
		v := v0 + (v1-v0)*f
		if s.Eval(u0, v).Minus(s.Eval(u1, v)).Magnitude() > tol {
			closedU = false
		}
		u := u0 + (u1-u0)*f
		if s.Eval(u, v0).Minus(s.Eval(u, v1)).Magnitude() > tol {
			closedV = false
		}
	}
	if closedU {
		s.periodU = u1 - u0
	}
	if closedV {
		s.periodV = v1 - v0
	}
	// A side is a pole when all its points are one.
	side := func(fix func(f float64) geom.Vector) bool {
		p0 := fix(0)
		for k := 1; k <= n; k++ {
			if fix(float64(k)/n).Minus(p0).Magnitude() > tol {
				return false
			}
		}
		return true
	}
	s.poles[0] = side(func(f float64) geom.Vector { return s.Eval(u0, v0+(v1-v0)*f) })
	s.poles[1] = side(func(f float64) geom.Vector { return s.Eval(u1, v0+(v1-v0)*f) })
	s.poles[2] = side(func(f float64) geom.Vector { return s.Eval(u0+(u1-u0)*f, v0) })
	s.poles[3] = side(func(f float64) geom.Vector { return s.Eval(u0+(u1-u0)*f, v1) })
}

func (s *bsplineSurface) size() float64 {
	var lo, hi geom.Vector
	first := true
	for _, row := range s.ctrl {
		for _, h := range row {
			p := geom.Vector{X: h.x / h.w, Y: h.y / h.w, Z: h.z / h.w}
			if first {
				lo, hi, first = p, p, false
			}
			growBox(p, &lo, &hi)
		}
	}
	return hi.Minus(lo).Magnitude()
}

func (s *bsplineSurface) Domain() (u0, u1, v0, v1 float64) {
	u0, u1 = s.bu.rng()
	v0, v1 = s.bv.rng()
	return
}

func (s *bsplineSurface) Period() (pu, pv float64) { return s.periodU, s.periodV }

// wrap brings a parameter of a closed direction back into the domain.
func wrap(t, lo, period float64) float64 {
	if period <= 0 {
		return t
	}
	if t < lo || t > lo+period {
		t = lo + math.Mod(t-lo, period)
		if t < lo {
			t += period
		}
	}
	return t
}

func (s *bsplineSurface) Derivs(u, v float64) (p, su, sv geom.Vector) {
	u0, _, v0, _ := s.Domain()
	u = wrap(u, u0, s.periodU)
	v = wrap(v, v0, s.periodV)
	pu, pv := s.bu.deg, s.bv.deg
	spu, spv := s.bu.span(u), s.bv.span(v)
	nu, nv := newDers(1, pu), newDers(1, pv)
	s.bu.ders(spu, u, 1, nu)
	s.bv.ders(spv, v, 1, nv)
	var a, au, av hpoint
	for i := 0; i <= pu; i++ {
		row := s.ctrl[spu-pu+i]
		var t0, t1 hpoint
		for j := 0; j <= pv; j++ {
			h := row[spv-pv+j]
			b0, b1 := nv[0][j], nv[1][j]
			t0.x += b0 * h.x
			t0.y += b0 * h.y
			t0.z += b0 * h.z
			t0.w += b0 * h.w
			t1.x += b1 * h.x
			t1.y += b1 * h.y
			t1.z += b1 * h.z
			t1.w += b1 * h.w
		}
		n0, n1 := nu[0][i], nu[1][i]
		a.x += n0 * t0.x
		a.y += n0 * t0.y
		a.z += n0 * t0.z
		a.w += n0 * t0.w
		au.x += n1 * t0.x
		au.y += n1 * t0.y
		au.z += n1 * t0.z
		au.w += n1 * t0.w
		av.x += n0 * t1.x
		av.y += n0 * t1.y
		av.z += n0 * t1.z
		av.w += n0 * t1.w
	}
	p = geom.Vector{X: a.x / a.w, Y: a.y / a.w, Z: a.z / a.w}
	su = geom.Vector{X: (au.x - au.w*p.X) / a.w, Y: (au.y - au.w*p.Y) / a.w, Z: (au.z - au.w*p.Z) / a.w}
	sv = geom.Vector{X: (av.x - av.w*p.X) / a.w, Y: (av.y - av.w*p.Y) / a.w, Z: (av.z - av.w*p.Z) / a.w}
	return p, su, sv
}

func (s *bsplineSurface) Eval(u, v float64) geom.Vector {
	p, _, _ := s.Derivs(u, v)
	return p
}

func (s *bsplineSurface) Pole(side int) (float64, bool) {
	u0, u1, v0, v1 := s.Domain()
	switch side {
	case poleUMin:
		return u0, s.poles[0]
	case poleUMax:
		return u1, s.poles[1]
	case poleVMin:
		return v0, s.poles[2]
	case poleVMax:
		return v1, s.poles[3]
	}
	return 0, false
}

func (s *bsplineSurface) Breaks() (us, vs []float64) { return s.bu.breaks(), s.bv.breaks() }

func (s *bsplineSurface) Project(p geom.Vector, guess *[2]float64) (u, v float64) {
	return projectOnSurface(s, p, guess)
}

func (s *bsplineSurface) samples() *surfaceGrid {
	s.gridOnce.Do(func() { s.grid = newSurfaceGrid(s) })
	return s.grid
}
