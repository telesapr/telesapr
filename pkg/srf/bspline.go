package srf

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/telesapr/telesapr/pkg/geom"
)

// B-spline curves and surfaces with knots, rational or not, of any
// degree: what STEP files and smooth lofts and blends give. The kernel
// works on Bezier patches, so a B-spline is cut at its knots into
// patches (BSplineSurface.Patches) of degree up to MaxDegree, which a
// shell takes as the patches of one face (topo.AddBSplineFace).
//
// The knot vectors hold every knot as often as its multiplicity, and
// have the number of control points plus the degree plus one entries;
// the parameter runs over [knots[deg], knots[len-1-deg]]. Ends that are
// not clamped are allowed.

// BSplineCurve is a B-spline curve.
type BSplineCurve struct {
	Deg    int
	Knots  []float64
	Ctrl   []geom.Vector
	Weight []float64
}

// BSplineSurface is a tensor product B-spline surface: Ctrl[i][j] is
// the control point i along u and j along v.
type BSplineSurface struct {
	DegU, DegV     int
	KnotsU, KnotsV []float64
	Ctrl           [][]geom.Vector
	Weight         [][]float64
}

// checkKnots checks a degree, its knot vector and the number of
// control points it serves.
func checkKnots(deg int, knots []float64, n int) error {
	switch {
	case deg < 1:
		return fmt.Errorf("srf: degree %d is less than 1", deg)
	case n < deg+1:
		return fmt.Errorf("srf: %d control points are too few for degree %d", n, deg)
	case len(knots) != n+deg+1:
		return fmt.Errorf("srf: %d knots don't fit %d control points of degree %d", len(knots), n, deg)
	}
	for i, k := range knots {
		if math.IsNaN(k) || math.IsInf(k, 0) {
			return errors.New("srf: a knot is not a number")
		}
		if i > 0 && k < knots[i-1] {
			return errors.New("srf: the knots decrease")
		}
		if m := multiplicity(knots, i); m > deg+1 {
			return fmt.Errorf("srf: a knot of multiplicity %d, more than the degree %d plus one", m, deg)
		}
	}
	if !(knots[len(knots)-1-deg] > knots[deg]) {
		return errors.New("srf: the knots span no range")
	}
	return nil
}

// multiplicity is how many knots equal knots[i].
func multiplicity(knots []float64, i int) int {
	m := 0
	for _, k := range knots {
		if k == knots[i] {
			m++
		}
	}
	return m
}

func checkWeights(w []float64) error {
	for _, x := range w {
		if !(x > 0) || math.IsInf(x, 0) {
			return fmt.Errorf("srf: a weight of %g; weights must be more than zero", x)
		}
	}
	return nil
}

// Check reports what is wrong with the curve, or nil.
func (c *BSplineCurve) Check() error {
	if err := checkKnots(c.Deg, c.Knots, len(c.Ctrl)); err != nil {
		return err
	}
	if len(c.Weight) != len(c.Ctrl) {
		return fmt.Errorf("srf: %d weights for %d control points", len(c.Weight), len(c.Ctrl))
	}
	return checkWeights(c.Weight)
}

// Check reports what is wrong with the surface, or nil.
func (s *BSplineSurface) Check() error {
	if len(s.Ctrl) == 0 {
		return errors.New("srf: a surface with no control points")
	}
	if err := checkKnots(s.DegU, s.KnotsU, len(s.Ctrl)); err != nil {
		return fmt.Errorf("along u: %w", err)
	}
	if err := checkKnots(s.DegV, s.KnotsV, len(s.Ctrl[0])); err != nil {
		return fmt.Errorf("along v: %w", err)
	}
	if len(s.Weight) != len(s.Ctrl) {
		return fmt.Errorf("srf: %d rows of weights for %d rows of control points", len(s.Weight), len(s.Ctrl))
	}
	for i := range s.Ctrl {
		if len(s.Ctrl[i]) != len(s.Ctrl[0]) || len(s.Weight[i]) != len(s.Ctrl[i]) {
			return fmt.Errorf("srf: row %d of the control net is %d long, its weights %d, the first row %d",
				i, len(s.Ctrl[i]), len(s.Weight[i]), len(s.Ctrl[0]))
		}
		if err := checkWeights(s.Weight[i]); err != nil {
			return err
		}
	}
	return nil
}

// Domain is the curve's parameter range.
func (c *BSplineCurve) Domain() (lo, hi float64) {
	return c.Knots[c.Deg], c.Knots[len(c.Knots)-1-c.Deg]
}

// Domain is the surface's parameter ranges.
func (s *BSplineSurface) Domain() (u0, u1, v0, v1 float64) {
	return s.KnotsU[s.DegU], s.KnotsU[len(s.KnotsU)-1-s.DegU], s.KnotsV[s.DegV], s.KnotsV[len(s.KnotsV)-1-s.DegV]
}

// findSpan is the index i of the knot span [knots[i], knots[i+1]) that
// holds t, clamped to the domain (Piegl and Tiller, A2.1).
func findSpan(deg int, knots []float64, t float64) int {
	n := len(knots) - deg - 2 // the last control point's index
	if t >= knots[n+1] {
		// The last span of non-zero length.
		i := n
		for i > deg && knots[i] == knots[i+1] {
			i--
		}
		return i
	}
	if t <= knots[deg] {
		i := deg
		for i < n && knots[i] == knots[i+1] {
			i++
		}
		return i
	}
	lo, hi := deg, n+1
	mid := (lo + hi) / 2
	for t < knots[mid] || t >= knots[mid+1] {
		if t < knots[mid] {
			hi = mid
		} else {
			lo = mid
		}
		mid = (lo + hi) / 2
	}
	return mid
}

// basisFuns are the deg+1 basis functions that are not zero at t, in
// span i (Piegl and Tiller, A2.2).
func basisFuns(i int, t float64, deg int, knots []float64, out []float64) {
	left := make([]float64, deg+1)
	right := make([]float64, deg+1)
	out[0] = 1
	for j := 1; j <= deg; j++ {
		left[j] = t - knots[i+1-j]
		right[j] = knots[i+j] - t
		saved := 0.0
		for r := 0; r < j; r++ {
			tmp := out[r] / (right[r+1] + left[j-r])
			out[r] = saved + right[r+1]*tmp
			saved = left[j-r] * tmp
		}
		out[j] = saved
	}
}

// PointAt is the curve's point at t.
func (c *BSplineCurve) PointAt(t float64) geom.Vector {
	span := findSpan(c.Deg, c.Knots, t)
	n := make([]float64, c.Deg+1)
	basisFuns(span, t, c.Deg, c.Knots, n)
	var p geom.Vector
	w := 0.0
	for k := 0; k <= c.Deg; k++ {
		i := span - c.Deg + k
		b := n[k] * c.Weight[i]
		p = p.Plus(c.Ctrl[i].ScaledBy(b))
		w += b
	}
	return p.ScaledBy(1 / w)
}

// PointAt is the surface's point at (u, v).
func (s *BSplineSurface) PointAt(u, v float64) geom.Vector {
	su := findSpan(s.DegU, s.KnotsU, u)
	sv := findSpan(s.DegV, s.KnotsV, v)
	nu := make([]float64, s.DegU+1)
	nv := make([]float64, s.DegV+1)
	basisFuns(su, u, s.DegU, s.KnotsU, nu)
	basisFuns(sv, v, s.DegV, s.KnotsV, nv)
	var p geom.Vector
	w := 0.0
	for a := 0; a <= s.DegU; a++ {
		i := su - s.DegU + a
		for b := 0; b <= s.DegV; b++ {
			j := sv - s.DegV + b
			f := nu[a] * nv[b] * s.Weight[i][j]
			p = p.Plus(s.Ctrl[i][j].ScaledBy(f))
			w += f
		}
	}
	return p.ScaledBy(1 / w)
}

// insertKnot inserts t once into a knot vector serving homogeneous
// control points pts, by Boehm's algorithm (Piegl and Tiller, A5.1),
// giving the new knots and points. t must lie in the domain, and be
// less than deg+1 times a knot already.
func insertKnot(deg int, knots []float64, pts []geom.Vector4, t float64) ([]float64, []geom.Vector4) {
	// The last knot not above t, t lying in [knots[k], knots[k+1]]; no
	// later than the domain's end, whose span is the last.
	k := sort.Search(len(knots), func(i int) bool { return knots[i] > t }) - 1
	k = min(k, len(knots)-2-deg)
	nk := make([]float64, 0, len(knots)+1)
	nk = append(nk, knots[:k+1]...)
	nk = append(nk, t)
	nk = append(nk, knots[k+1:]...)
	np := make([]geom.Vector4, len(pts)+1)
	for i := 0; i <= k-deg; i++ {
		np[i] = pts[i]
	}
	for i := k - deg + 1; i <= k; i++ {
		a := 0.0
		if d := knots[i+deg] - knots[i]; d != 0 {
			a = (t - knots[i]) / d
		}
		np[i] = pts[i-1].ScaledBy(1 - a).Plus(pts[i].ScaledBy(a))
	}
	for i := k + 1; i <= len(pts); i++ {
		np[i] = pts[i-1]
	}
	return nk, np
}

// breaks are the distinct knots inside the domain and at its ends.
func breaks(deg int, knots []float64) []float64 {
	lo, hi := knots[deg], knots[len(knots)-1-deg]
	var out []float64
	for _, k := range knots {
		if k >= lo && k <= hi && (len(out) == 0 || k > out[len(out)-1]) {
			out = append(out, k)
		}
	}
	return out
}

// bezierSegments cuts a B-spline of homogeneous points into Bezier
// segments, one per span of the domain: every break is inserted until
// it is deg times a knot, and segment i's points are the deg+1 that
// span i weights.
func bezierSegments(deg int, knots []float64, pts []geom.Vector4) [][]geom.Vector4 {
	bs := breaks(deg, knots)
	for _, b := range bs {
		count := 0
		for _, k := range knots {
			if k == b {
				count++
			}
		}
		for ; count < deg; count++ {
			knots, pts = insertKnot(deg, knots, pts, b)
		}
	}
	var out [][]geom.Vector4
	for s := 0; s+1 < len(bs); s++ {
		// The span [bs[s], bs[s+1]): the last knot equal to bs[s].
		i := sort.SearchFloat64s(knots, bs[s+1]) - 1
		out = append(out, append([]geom.Vector4(nil), pts[i-deg:i+1]...))
	}
	return out
}

func homogeneous(p geom.Vector, w float64) geom.Vector4 { return geom.Vector4FromWeighted(w, p) }

// Breaks are the knots that cut the curve into Bezier pieces, the
// domain's ends included.
func (c *BSplineCurve) Breaks() []float64 { return breaks(c.Deg, c.Knots) }

// Breaks are the knots that cut the surface into patches, along u and
// along v, the domain's ends included.
func (s *BSplineSurface) Breaks() (us, vs []float64) {
	return breaks(s.DegU, s.KnotsU), breaks(s.DegV, s.KnotsV)
}

// Beziers cuts the curve at its knots into Bezier curves, each running
// over one span of the domain from 0 to 1.
func (c *BSplineCurve) Beziers() ([]SBezier, error) {
	if err := c.Check(); err != nil {
		return nil, err
	}
	if c.Deg > MaxDegree {
		return nil, fmt.Errorf("srf: a curve of degree %d; pieces go up to %d", c.Deg, MaxDegree)
	}
	pts := make([]geom.Vector4, len(c.Ctrl))
	for i := range pts {
		pts[i] = homogeneous(c.Ctrl[i], c.Weight[i])
	}
	var out []SBezier
	for _, seg := range bezierSegments(c.Deg, append([]float64(nil), c.Knots...), pts) {
		out = append(out, SBezierFromV4(seg...))
	}
	return out, nil
}

// Patches cuts the surface at its knots into Bezier patches:
// patches[a][b] runs over span a of the breaks along u and span b
// along v, its own parameters from 0 to 1 over them.
func (s *BSplineSurface) Patches() ([][]SSurface, error) {
	if err := s.Check(); err != nil {
		return nil, err
	}
	if s.DegU > MaxDegree || s.DegV > MaxDegree {
		return nil, fmt.Errorf("srf: a surface of degree %d by %d; patches go up to %d", s.DegU, s.DegV, MaxDegree)
	}
	// Along v first, row by row: rows[i][b] are the points of span b
	// of row i.
	nu := len(s.Ctrl)
	rows := make([][][]geom.Vector4, nu)
	for i := range s.Ctrl {
		pts := make([]geom.Vector4, len(s.Ctrl[i]))
		for j := range pts {
			pts[j] = homogeneous(s.Ctrl[i][j], s.Weight[i][j])
		}
		rows[i] = bezierSegments(s.DegV, append([]float64(nil), s.KnotsV...), pts)
	}
	nb := len(rows[0])
	// Then along u, column by column of each span along v.
	var out [][]SSurface
	for b := 0; b < nb; b++ {
		for k := 0; k <= s.DegV; k++ {
			col := make([]geom.Vector4, nu)
			for i := 0; i < nu; i++ {
				col[i] = rows[i][b][k]
			}
			segs := bezierSegments(s.DegU, append([]float64(nil), s.KnotsU...), col)
			if b == 0 && k == 0 {
				out = make([][]SSurface, len(segs))
				for a := range out {
					out[a] = make([]SSurface, nb)
				}
			}
			for a, seg := range segs {
				p := &out[a][b]
				p.DegM, p.DegN = s.DegU, s.DegV
				for m, q := range seg {
					p.Weight[m][k] = q.W
					p.Ctrl[m][k] = q.PerspectiveProject()
				}
			}
		}
	}
	return out, nil
}

// EdgeCurve is the patch's boundary curve along one side: v = 0 (side
// 0), u = 1 (1), v = 1 (2) or u = 0 (3), running with u or v.
func (s *SSurface) EdgeCurve(side int) SBezier {
	var b SBezier
	switch side {
	case 0, 2:
		j := 0
		if side == 2 {
			j = s.DegN
		}
		b.Degree = s.DegM
		for i := 0; i <= s.DegM; i++ {
			b.Ctrl[i], b.Weight[i] = s.Ctrl[i][j], s.Weight[i][j]
		}
	default:
		i := 0
		if side == 1 {
			i = s.DegM
		}
		b.Degree = s.DegN
		for j := 0; j <= s.DegN; j++ {
			b.Ctrl[j], b.Weight[j] = s.Ctrl[i][j], s.Weight[i][j]
		}
	}
	return b
}
