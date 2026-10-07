package srf

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// Skinning (roadmap E8): a surface through curves of one degree and one
// knot vector, the sections, interpolated across them by a B-spline of
// degree three, or less when there are fewer than four sections. The
// surface goes through each section exactly; between them it is smooth.
// Rational sections are interpolated in homogeneous coordinates, so a
// circle stays a circle where a section has one.

// ChainCurve is a chain of Bezier curves joined end to end as one
// clamped B-spline curve of degree deg, each piece raised to it: piece
// i runs over [i/n, (i+1)/n], its joints knots deg times over. A chain
// whose end meets its start within LengthEps is closed exactly, the
// last control point made the first.
func ChainCurve(chain []SBezier, deg int) (BSplineCurve, error) {
	if len(chain) == 0 {
		return BSplineCurve{}, errors.New("srf: a chain of no curves")
	}
	n := len(chain)
	c := BSplineCurve{Deg: deg}
	for k := 0; k <= deg; k++ {
		c.Knots = append(c.Knots, 0)
	}
	for i := 1; i < n; i++ {
		for range deg {
			c.Knots = append(c.Knots, float64(i)/float64(n))
		}
	}
	for k := 0; k <= deg; k++ {
		c.Knots = append(c.Knots, 1)
	}
	for i := range chain {
		b, err := chain[i].ElevatedTo(deg)
		if err != nil {
			return BSplineCurve{}, err
		}
		if b.Degree != deg {
			return BSplineCurve{}, fmt.Errorf("srf: a curve of degree %d in a chain of degree %d", b.Degree, deg)
		}
		if i > 0 && !b.Ctrl[0].Equals(c.Ctrl[len(c.Ctrl)-1]) {
			return BSplineCurve{}, fmt.Errorf("srf: the chain breaks at %v", b.Ctrl[0])
		}
		// Each piece's ends are the curve's points there, so a weight
		// other than the last piece's at a joint is scaled to it: the
		// homogeneous points of a rational Bezier can be scaled as a
		// whole.
		s := 1.0
		if i > 0 {
			s = c.Weight[len(c.Weight)-1] / b.Weight[0]
		}
		first := 0
		if i > 0 {
			first = 1
		}
		for k := first; k <= deg; k++ {
			c.Ctrl = append(c.Ctrl, b.Ctrl[k])
			c.Weight = append(c.Weight, b.Weight[k]*s)
		}
	}
	if last := len(c.Ctrl) - 1; c.Ctrl[last].Equals(c.Ctrl[0]) {
		c.Ctrl[last] = c.Ctrl[0]
	}
	return c, c.Check()
}

// Skin is the surface through sections, curves of one degree and knot
// vector: u runs along them, v across, from the first section at v = 0
// to the last at v = 1. The v of each section comes back too, by the
// lengths between the sections' control points. Two sections make a
// ruled surface, three one of degree two across, more a cubic.
func Skin(sections []BSplineCurve) (*BSplineSurface, []float64, error) {
	k := len(sections)
	if k < 2 {
		return nil, nil, errors.New("srf: a skin needs two sections at least")
	}
	first := sections[0]
	for i, c := range sections {
		if c.Deg != first.Deg || len(c.Knots) != len(first.Knots) || len(c.Ctrl) != len(first.Ctrl) {
			return nil, nil, fmt.Errorf("srf: section %d has another degree or knot vector than the first", i)
		}
		for j := range c.Knots {
			if c.Knots[j] != first.Knots[j] {
				return nil, nil, fmt.Errorf("srf: section %d has another knot vector than the first", i)
			}
		}
	}
	// Chord lengths across, the mean over the control points.
	ts := make([]float64, k)
	for s := 1; s < k; s++ {
		d := 0.0
		for i := range first.Ctrl {
			d += sections[s].Ctrl[i].Minus(sections[s-1].Ctrl[i]).Magnitude()
		}
		d /= float64(len(first.Ctrl))
		if d < geom.LengthEps {
			return nil, nil, fmt.Errorf("srf: sections %d and %d lie on each other", s-1, s)
		}
		ts[s] = ts[s-1] + d
	}
	for s := range ts {
		ts[s] /= ts[k-1]
	}
	ts[k-1] = 1
	// One solve for every column of control points, in homogeneous
	// coordinates.
	rhs := make([][]geom.Vector4, len(first.Ctrl))
	for i := range rhs {
		rhs[i] = make([]geom.Vector4, k)
		for s := range sections {
			rhs[i][s] = homogeneous(sections[s].Ctrl[i], sections[s].Weight[i])
		}
	}
	deg, knots, err := interpolate(ts, rhs)
	if err != nil {
		return nil, nil, err
	}
	sf := &BSplineSurface{DegU: first.Deg, DegV: deg,
		KnotsU: append([]float64(nil), first.Knots...), KnotsV: knots}
	for i := range rhs {
		row := make([]geom.Vector, k)
		w := make([]float64, k)
		for j, q := range rhs[i] {
			if q.W <= 0 {
				return nil, nil, errors.New("srf: the sections' weights interpolate to below zero")
			}
			row[j], w[j] = q.PerspectiveProject(), q.W
		}
		sf.Ctrl = append(sf.Ctrl, row)
		sf.Weight = append(sf.Weight, w)
	}
	// Each section's own points where they were, exactly: the solve
	// gives them at the ends, through the clamped knots, within
	// rounding.
	for i := range sf.Ctrl {
		sf.Ctrl[i][0], sf.Weight[i][0] = first.Ctrl[i], first.Weight[i]
		last := sections[k-1]
		sf.Ctrl[i][k-1], sf.Weight[i][k-1] = last.Ctrl[i], last.Weight[i]
	}
	return sf, ts, sf.Check()
}

// interpolate turns each column of rhs, values at the parameters ts
// (from 0 to 1, rising), into the control points of the B-spline of
// degree min(3, len(ts)-1) through them, in place, with averaged knots
// (Piegl and Tiller, 9.8), each inner one the mean of deg parameters,
// which keeps the system well posed.
func interpolate(ts []float64, rhs [][]geom.Vector4) (int, []float64, error) {
	k := len(ts)
	deg := min(3, k-1)
	knots := make([]float64, 0, k+deg+1)
	for j := 0; j <= deg; j++ {
		knots = append(knots, 0)
	}
	for j := 1; j <= k-1-deg; j++ {
		s := 0.0
		for i := j; i < j+deg; i++ {
			s += ts[i]
		}
		knots = append(knots, s/float64(deg))
	}
	for j := 0; j <= deg; j++ {
		knots = append(knots, 1)
	}
	a := make([][]float64, k)
	nb := make([]float64, deg+1)
	for r := range a {
		a[r] = make([]float64, k)
		span := findSpan(deg, knots, ts[r])
		basisFuns(span, ts[r], deg, knots, nb)
		for c := 0; c <= deg; c++ {
			a[r][span-deg+c] = nb[c]
		}
	}
	if err := solveDense(a, rhs); err != nil {
		return 0, nil, err
	}
	return deg, knots, nil
}

// solveDense solves a x = b for each right-hand side column in rhs, in
// place, by Gaussian elimination with partial pivoting. a is changed.
func solveDense(a [][]float64, rhs [][]geom.Vector4) error {
	n := len(a)
	for c := range n {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		if math.Abs(a[p][c]) < 1e-12 {
			return errors.New("srf: the interpolation across the sections is singular")
		}
		a[c], a[p] = a[p], a[c]
		for i := range rhs {
			rhs[i][c], rhs[i][p] = rhs[i][p], rhs[i][c]
		}
		for r := c + 1; r < n; r++ {
			f := a[r][c] / a[c][c]
			if f == 0 {
				continue
			}
			for j := c; j < n; j++ {
				a[r][j] -= f * a[c][j]
			}
			for i := range rhs {
				rhs[i][r] = rhs[i][r].Minus(rhs[i][c].ScaledBy(f))
			}
		}
	}
	for c := n - 1; c >= 0; c-- {
		for i := range rhs {
			q := rhs[i][c]
			for j := c + 1; j < n; j++ {
				q = q.Minus(rhs[i][j].ScaledBy(a[c][j]))
			}
			rhs[i][c] = q.ScaledBy(1 / a[c][c])
		}
	}
	return nil
}

// SkinWithEnds is the cubic surface through sections, curves of one
// degree and knot vector, their weights their own, at the parameters ts across, from 0
// to 1, with the derivatives across at the first and the last given for
// each control point (Piegl and Tiller, 9.2.2): the interior knots are
// the sections' own parameters, and two more control points meet the
// derivatives. Surfaces that end and start with one section and
// derivatives along one another meet tangent there.
func SkinWithEnds(sections []BSplineCurve, ts []float64, start, end []geom.Vector) (*BSplineSurface, error) {
	if len(sections) < 2 || len(ts) != len(sections) {
		return nil, errors.New("srf: a skin needs two sections at least, each with its parameter")
	}
	first := sections[0]
	if len(start) != len(first.Ctrl) || len(end) != len(first.Ctrl) {
		return nil, errors.New("srf: a derivative for each control point at each end")
	}
	last := sections[len(sections)-1]
	for i, c := range sections {
		if len(c.Weight) != len(first.Weight) {
			return nil, fmt.Errorf("srf: section %d has another degree or knot vector than the first", i)
		}
	}
	// Each end section with its own weights; the weight's derivative
	// across is 0 at the ends, so there the homogeneous derivative is the
	// weight times the point's.
	hs, he := make([]geom.Vector4, len(start)), make([]geom.Vector4, len(end))
	for i := range start {
		hs[i] = geom.Vector4FromWeighted(first.Weight[i], start[i])
		he[i] = geom.Vector4FromWeighted(last.Weight[i], end[i])
		hs[i].W, he[i].W = 0, 0
	}
	return SkinThrough(sections, ts, hs, he)
}

// SkinThrough is SkinWithEnds for sections whose weights may differ,
// as arcs of different angles do: the derivatives across at the ends
// are given in homogeneous coordinates, (w p)' and w', for each control
// point. Surfaces that end and start with one section and the same
// homogeneous derivatives, each scaled by its own parameter's length,
// meet tangent there.
func SkinThrough(sections []BSplineCurve, ts []float64, start, end []geom.Vector4) (*BSplineSurface, error) {
	k := len(sections)
	if k < 2 || len(ts) != k {
		return nil, errors.New("srf: a skin needs two sections at least, each with its parameter")
	}
	first := sections[0]
	if len(start) != len(first.Ctrl) || len(end) != len(first.Ctrl) {
		return nil, errors.New("srf: a derivative for each control point at each end")
	}
	for i, c := range sections {
		if c.Deg != first.Deg || len(c.Knots) != len(first.Knots) || len(c.Ctrl) != len(first.Ctrl) {
			return nil, fmt.Errorf("srf: section %d has another degree or knot vector than the first", i)
		}
		for j := range c.Knots {
			if c.Knots[j] != first.Knots[j] {
				return nil, fmt.Errorf("srf: section %d has another knot vector than the first", i)
			}
		}
		if i > 0 && !(ts[i] > ts[i-1]) {
			return nil, errors.New("srf: the sections' parameters must rise")
		}
	}
	if ts[0] != 0 || ts[k-1] != 1 {
		return nil, errors.New("srf: the sections' parameters run from 0 to 1")
	}
	const deg = 3
	knots := []float64{0, 0, 0, 0}
	knots = append(knots, ts[1:k-1]...)
	knots = append(knots, 1, 1, 1, 1)
	n := k + 2
	a := make([][]float64, n)
	for r := range a {
		a[r] = make([]float64, n)
	}
	nb := make([]float64, deg+1)
	row := 0
	put := func(t float64) {
		span := findSpan(deg, knots, t)
		basisFuns(span, t, deg, knots, nb)
		for c := 0; c <= deg; c++ {
			a[row][span-deg+c] = nb[c]
		}
		row++
	}
	// The rows: the first section, the derivative at the start, the
	// sections between, the derivative at the end, the last section.
	put(0)
	d0 := deg / knots[deg+1]
	a[row][0], a[row][1] = -d0, d0
	row++
	for s := 1; s < k-1; s++ {
		put(ts[s])
	}
	d1 := deg / (1 - knots[len(knots)-deg-2])
	a[row][n-2], a[row][n-1] = -d1, d1
	row++
	put(1)
	rhs := make([][]geom.Vector4, len(first.Ctrl))
	for i := range rhs {
		col := make([]geom.Vector4, 0, n)
		col = append(col, homogeneous(sections[0].Ctrl[i], sections[0].Weight[i]), start[i])
		for s := 1; s < k-1; s++ {
			col = append(col, homogeneous(sections[s].Ctrl[i], sections[s].Weight[i]))
		}
		col = append(col, end[i], homogeneous(sections[k-1].Ctrl[i], sections[k-1].Weight[i]))
		rhs[i] = col
	}
	if err := solveDense(a, rhs); err != nil {
		return nil, err
	}
	sf := &BSplineSurface{DegU: first.Deg, DegV: deg,
		KnotsU: append([]float64(nil), first.Knots...), KnotsV: knots}
	for i := range rhs {
		r := make([]geom.Vector, n)
		w := make([]float64, n)
		for j, q := range rhs[i] {
			if q.W <= 0 {
				return nil, errors.New("srf: the sections' weights interpolate to below zero")
			}
			r[j], w[j] = q.PerspectiveProject(), q.W
		}
		// The end sections exactly, as Skin keeps them.
		r[0], w[0] = sections[0].Ctrl[i], sections[0].Weight[i]
		r[n-1], w[n-1] = sections[k-1].Ctrl[i], sections[k-1].Weight[i]
		sf.Ctrl = append(sf.Ctrl, r)
		sf.Weight = append(sf.Weight, w)
	}
	return sf, sf.Check()
}
