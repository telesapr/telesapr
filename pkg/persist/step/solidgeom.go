package step

import (
	"fmt"
	"math"
	"sort"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The exact geometry of a solid (roadmap D3): each edge's curve as
// Beziers, and each face's surface as Bezier patches, one for each cell
// of a grid of its parameters, whose lines are where the surface's
// pieces meet: a B-spline's knots, and for a surface that turns about
// an axis a quarter turn at most, so a rational quadratic holds it.

// errUnsupported is geometry the solid importer can't hold exactly.
type errUnsupported string

func (e errUnsupported) Error() string { return string(e) }

// srfCurve is a STEP B-spline curve as the kernel's.
func (c *bsplineCurve) srfCurve() *srf.BSplineCurve {
	out := &srf.BSplineCurve{Deg: c.b.deg, Knots: append([]float64(nil), c.b.knots...)}
	for _, h := range c.ctrl {
		out.Ctrl = append(out.Ctrl, geom.Vector{X: h.x / h.w, Y: h.y / h.w, Z: h.z / h.w})
		out.Weight = append(out.Weight, h.w)
	}
	return out
}

// lowDegreeTol is how far, in millimetres, a curve or surface of a
// degree above srf.MaxDegree may move when the solid importer writes it
// at degree srf.MaxDegree (package 27.2): a tenth of a file's
// tolerance.
const lowDegreeTol = fileTol / 10

// kernel is the curve as the kernel's, at degree srf.MaxDegree within
// lowDegreeTol when its own is above.
func (c *bsplineCurve) kernel() (*srf.BSplineCurve, error) {
	c.low.once.Do(func() {
		bc := c.srfCurve()
		c.low.c, c.low.err = bc.LowDegree(lowDegreeTol)
		if c.low.err != nil {
			c.low.err = errUnsupported(fmt.Sprintf("a curve of degree %d (%v)", bc.Deg, c.low.err))
		}
	})
	return c.low.c, c.low.err
}

// srfSurface is a STEP B-spline surface as the kernel's.
func (s *bsplineSurface) srfSurface() *srf.BSplineSurface {
	out := &srf.BSplineSurface{DegU: s.bu.deg, DegV: s.bv.deg,
		KnotsU: append([]float64(nil), s.bu.knots...), KnotsV: append([]float64(nil), s.bv.knots...)}
	for _, row := range s.ctrl {
		var pts []geom.Vector
		var ws []float64
		for _, h := range row {
			pts = append(pts, geom.Vector{X: h.x / h.w, Y: h.y / h.w, Z: h.z / h.w})
			ws = append(ws, h.w)
		}
		out.Ctrl = append(out.Ctrl, pts)
		out.Weight = append(out.Weight, ws)
	}
	return out
}

// arcBezier is the rational quadratic of the arc of an ellipse in frame
// f with semi-axes a and b from angle ta to tb, at most a quarter turn
// apart.
func arcBezier(f frame, a, b, ta, tb float64) srf.SBezier {
	m, h := (ta+tb)/2, (tb-ta)/2
	sb := srf.SBezierFrom3(f.at(a*math.Cos(ta), b*math.Sin(ta), 0),
		f.at(a*math.Cos(m)/math.Cos(h), b*math.Sin(m)/math.Cos(h), 0),
		f.at(a*math.Cos(tb), b*math.Sin(tb), 0))
	sb.Weight[1] = math.Cos(h)
	return sb
}

// quarterSplits are the points that cut [t0, t1] (either way round)
// into pieces of at most a quarter turn, equal, ends included.
func quarterSplits(t0, t1 float64) []float64 {
	n := max(1, int(math.Ceil(math.Abs(t1-t0)/(math.Pi/2)-1e-9)))
	out := make([]float64, n+1)
	for i := range out {
		out[i] = t0 + (t1-t0)*float64(i)/float64(n)
	}
	return out
}

// curveBreaks are the parameters inside (lo, hi) where a curve's
// pieces meet: its knots, or for a conic the quarter turns from lo.
func curveBreaks(c Curve, lo, hi float64) []float64 {
	var out []float64
	switch cc := c.(type) {
	case *bsplineCurve:
		bc, err := cc.kernel()
		if err != nil {
			// curvePiece says so.
			bc = cc.srfCurve()
		}
		per := cc.Period()
		shifts := []float64{0}
		if per > 0 {
			r0, _ := cc.Range()
			for m := math.Floor((lo - r0) / per); m*per+r0 <= hi; m++ {
				shifts = append(shifts, m*per)
			}
		}
		seen := map[float64]bool{}
		for _, sh := range shifts {
			for _, k := range bc.Breaks() {
				if k+sh > lo && k+sh < hi && !seen[k+sh] {
					seen[k+sh] = true
					out = append(out, k+sh)
				}
			}
		}
		sort.Float64s(out)
	case *ellipse:
		q := quarterSplits(lo, hi)
		out = q[1 : len(q)-1]
	case *conic:
		if cc.hyperbola {
			// Pieces of a hyperbola no longer than one in its parameter,
			// each one rational quadratic, cut at whole numbers, so that
			// an edge and the extrusion of the same curve are cut alike.
			for k := math.Floor(lo) + 1; k < hi; k++ {
				out = append(out, k)
			}
		}
	}
	return out
}

// conicPiece is a parabola's or hyperbola's arc from ta to tb as one
// Bezier, exactly (package 27.5): a parabola's quadratic, through the
// point where the tangents at the ends meet; a hyperbola's rational
// quadratic, its middle point there too, of weight the hyperbolic
// cosine of half the arc's parameter.
func conicPiece(c *conic, ta, tb float64) srf.SBezier {
	if !c.hyperbola {
		p0, p2 := c.Eval(ta), c.Eval(tb)
		p1 := p0.Plus(c.Deriv(ta).ScaledBy((tb - ta) / 2))
		return srf.SBezierFrom3(p0, p1, p2)
	}
	m, h := (ta+tb)/2, (tb-ta)/2
	ch := math.Cosh(h)
	p1 := c.f.at(c.a*math.Cosh(m)/ch, c.b*math.Sinh(m)/ch, 0)
	sb := srf.SBezierFrom3(c.Eval(ta), p1, c.Eval(tb))
	sb.Weight[1] = ch
	return sb
}

// curvePiece is the Bezier of curve c from parameter ta to tb, which
// must lie within one piece of it (curveBreaks), or reach past the end
// of a B-spline's, which extends it.
func curvePiece(c Curve, ta, tb float64) (srf.SBezier, error) {
	switch cc := c.(type) {
	case *line:
		return srf.SBezierFrom2(cc.Eval(ta), cc.Eval(tb)), nil
	case *ellipse:
		if math.Abs(tb-ta) > math.Pi/2+1e-9 {
			return srf.SBezier{}, fmt.Errorf("an arc of %g radians in one piece", tb-ta)
		}
		return arcBezier(cc.f, cc.a, cc.b, ta, tb), nil
	case *conic:
		return conicPiece(cc, ta, tb), nil
	case *bsplineCurve:
		bc, err := cc.kernel()
		if err != nil {
			return srf.SBezier{}, err
		}
		pieces, err := bc.Beziers()
		if err != nil {
			return srf.SBezier{}, err
		}
		br := bc.Breaks()
		if per := cc.Period(); per > 0 {
			// A closed curve's parameter, brought into its range.
			r0, r1 := cc.Range()
			m := (ta + tb) / 2
			sh := math.Floor((m-r0)/per) * per
			if m-sh > r1 {
				sh += per
			}
			ta, tb = ta-sh, tb-sh
		}
		mid := (ta + tb) / 2
		k := 0
		for k+1 < len(pieces) && mid > br[k+1] {
			k++
		}
		lo, hi := br[k], br[k+1]
		return pieces[k].Sub((ta-lo)/(hi-lo), (tb-lo)/(hi-lo)), nil
	}
	return srf.SBezier{}, errUnsupported(fmt.Sprintf("a curve of kind %T", c))
}

// edgeParams are the parameters of an edge's curve at its start and
// end, along the curve the way the edge runs, as sampleEdge finds them.
func edgeParams(e *Edge) (t0, t1 float64, err error) {
	c := e.Curve
	t0, t1 = c.Param(e.Start), c.Param(e.End)
	lo, hi := c.Range()
	per := c.Period()
	switch {
	case per > 0 && e.Closed:
		if e.Sense {
			t1 = t0 + per
		} else {
			t1 = t0 - per
		}
	case per > 0:
		eps := 1e-12 * per
		if e.Sense {
			for t1 <= t0+eps {
				t1 += per
			}
			for t1-t0 > per {
				t1 -= per
			}
		} else {
			for t1 >= t0-eps {
				t1 -= per
			}
			for t0-t1 > per {
				t1 += per
			}
		}
	case e.Closed:
		if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
			return 0, 0, fmt.Errorf("edge #%d is an open curve that starts where it ends", e.ID)
		}
		t0, t1 = lo, hi
		if !e.Sense {
			t0, t1 = hi, lo
		}
	}
	if t0 == t1 {
		return 0, 0, fmt.Errorf("edge #%d has no length along its curve", e.ID)
	}
	return t0, t1, nil
}

// edgeBeziers are an edge's curve as Beziers from its start to its end,
// those ends its vertices exactly.
func edgeBeziers(e *Edge) ([]srf.SBezier, error) {
	if _, ok := e.Curve.(*line); ok {
		if e.Closed {
			return nil, fmt.Errorf("edge #%d is a line that starts where it ends", e.ID)
		}
		return []srf.SBezier{srf.SBezierFrom2(e.Start, e.End)}, nil
	}
	t0, t1, err := edgeParams(e)
	if err != nil {
		return nil, err
	}
	lo, hi := math.Min(t0, t1), math.Max(t0, t1)
	cuts := append(append([]float64{lo}, curveBreaks(e.Curve, lo, hi)...), hi)
	var out []srf.SBezier
	for i := 0; i+1 < len(cuts); i++ {
		if cuts[i+1]-cuts[i] <= 1e-12*(1+math.Abs(cuts[i])) {
			continue
		}
		sb, err := curvePiece(e.Curve, cuts[i], cuts[i+1])
		if err != nil {
			return nil, err
		}
		out = append(out, sb)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("edge #%d has no length along its curve", e.ID)
	}
	if t1 < t0 {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
		for i := range out {
			out[i].Reverse()
		}
	}
	// The ends are the vertices, and each joint one point.
	out[0].Ctrl[0] = e.Start
	out[len(out)-1].Ctrl[out[len(out)-1].Degree] = e.End
	for i := 1; i < len(out); i++ {
		out[i].Ctrl[0] = out[i-1].Ctrl[out[i-1].Degree]
	}
	return out, nil
}

// cellPatch makes the patch of a surface over one cell of its grid,
// [ua, ub] by [va, vb] in the surface's own parameters, its u and v
// running with them, so its normal is the surface's.
type cellPatch func(ua, ub, va, vb float64) (srf.SSurface, error)

// revolvedPatch is the patch of the surface that profile, the curve
// between va and vb as one Bezier, sweeps turning about the axis
// through o along unit a from angle ua to ub, at most a quarter turn:
// each control point of the profile a rational quadratic arc.
func revolvedPatch(profile srf.SBezier, o, a geom.Vector, ua, ub float64) srf.SSurface {
	m, h := (ua+ub)/2, (ub-ua)/2
	var s srf.SSurface
	s.DegM, s.DegN = 2, profile.Degree
	for j := 0; j <= profile.Degree; j++ {
		q := profile.Ctrl[j]
		foot := o.Plus(a.ScaledBy(q.Minus(o).Dot(a)))
		mid := q.RotatedAbout(o, a, m)
		s.Ctrl[0][j] = q.RotatedAbout(o, a, ua)
		s.Ctrl[1][j] = foot.Plus(mid.Minus(foot).ScaledBy(1 / math.Cos(h)))
		s.Ctrl[2][j] = q.RotatedAbout(o, a, ub)
		w := profile.Weight[j]
		s.Weight[0][j], s.Weight[1][j], s.Weight[2][j] = w, w*math.Cos(h), w
	}
	return s
}

// surfaceGridOf is how a surface is cut into cells: its breaks along u
// and v inside a range, and the patch of each cell.
type surfaceCells struct {
	// breaksU and breaksV give the lines inside (lo, hi) of the
	// surface's own pieces.
	breaksU, breaksV func(lo, hi float64) []float64
	patch            cellPatch
}

// cellsOf is the grid of a face's surface, or an error for a surface
// the solid importer can't hold exactly.
func cellsOf(s Surface) (*surfaceCells, error) {
	quarters := func(lo, hi float64) []float64 {
		q := quarterSplits(lo, hi)
		return q[1 : len(q)-1]
	}
	none := func(lo, hi float64) []float64 { return nil }
	switch ss := s.(type) {
	case *cone:
		f := ss.f
		return &surfaceCells{breaksU: quarters, breaksV: none,
			patch: func(ua, ub, va, vb float64) (srf.SSurface, error) {
				prof := srf.SBezierFrom2(f.at(ss.r+va*ss.tan, 0, va), f.at(ss.r+vb*ss.tan, 0, vb))
				return revolvedPatch(prof, f.o, f.z, ua, ub), nil
			}}, nil
	case *sphere:
		f := ss.f
		return &surfaceCells{breaksU: quarters, breaksV: quarters,
			patch: func(ua, ub, va, vb float64) (srf.SSurface, error) {
				g := frame{o: f.o, x: f.x, y: f.z, z: f.x.Cross(f.z)}
				prof := arcBezier(g, ss.r, ss.r, va, vb)
				return revolvedPatch(prof, f.o, f.z, ua, ub), nil
			}}, nil
	case *torus:
		f := ss.f
		return &surfaceCells{breaksU: quarters, breaksV: quarters,
			patch: func(ua, ub, va, vb float64) (srf.SSurface, error) {
				g := frame{o: f.o.Plus(f.x.ScaledBy(ss.major)), x: f.x, y: f.z, z: f.x.Cross(f.z)}
				prof := arcBezier(g, ss.r, ss.r, va, vb)
				return revolvedPatch(prof, f.o, f.z, ua, ub), nil
			}}, nil
	case *revolution:
		return &surfaceCells{breaksU: quarters,
			breaksV: func(lo, hi float64) []float64 { return curveBreaks(ss.c, lo, hi) },
			patch: func(ua, ub, va, vb float64) (srf.SSurface, error) {
				prof, err := curvePiece(ss.c, va, vb)
				if err != nil {
					return srf.SSurface{}, err
				}
				return revolvedPatch(prof, ss.o, ss.a, ua, ub), nil
			}}, nil
	case *extrusion:
		return &surfaceCells{breaksU: func(lo, hi float64) []float64 { return curveBreaks(ss.c, lo, hi) },
			breaksV: none,
			patch: func(ua, ub, va, vb float64) (srf.SSurface, error) {
				c, err := curvePiece(ss.c, ua, ub)
				if err != nil {
					return srf.SSurface{}, err
				}
				var p srf.SSurface
				p.DegM, p.DegN = c.Degree, 1
				for i := 0; i <= c.Degree; i++ {
					p.Ctrl[i][0] = c.Ctrl[i].Plus(ss.d.ScaledBy(va))
					p.Ctrl[i][1] = c.Ctrl[i].Plus(ss.d.ScaledBy(vb))
					p.Weight[i][0], p.Weight[i][1] = c.Weight[i], c.Weight[i]
				}
				return p, nil
			}}, nil
	case *offset:
		if eq, ok := ss.analytic(); ok {
			return cellsOf(eq)
		}
		return offsetCells(ss)
	case *bsplineSurface:
		bs, err := ss.srfSurface().LowDegree(lowDegreeTol)
		if err != nil {
			return nil, errUnsupported(fmt.Sprintf("a surface of degree %d by %d (%v)", ss.bu.deg, ss.bv.deg, err))
		}
		ps, err := bs.Patches()
		if err != nil {
			return nil, err
		}
		bu, bv := bs.Breaks()
		// The lines are the knots inside the domain: past its ends the
		// first and last pieces reach on.
		inside := func(br []float64) func(lo, hi float64) []float64 {
			return func(lo, hi float64) []float64 {
				var out []float64
				for _, k := range br[1 : len(br)-1] {
					if k > lo && k < hi {
						out = append(out, k)
					}
				}
				return out
			}
		}
		span := func(br []float64, t float64) int {
			k := 0
			for k+2 < len(br) && t > br[k+1] {
				k++
			}
			return k
		}
		return &surfaceCells{breaksU: inside(bu), breaksV: inside(bv),
			patch: func(ua, ub, va, vb float64) (srf.SSurface, error) {
				a, b := span(bu, (ua+ub)/2), span(bv, (va+vb)/2)
				p := ps[a][b]
				ul, uh, vl, vh := bu[a], bu[a+1], bv[b], bv[b+1]
				return p.Sub((ua-ul)/(uh-ul), (ub-ul)/(uh-ul), (va-vl)/(vh-vl), (vb-vl)/(vh-vl)), nil
			}}, nil
	}
	return nil, errUnsupported(fmt.Sprintf("a surface of kind %T", s))
}

// analytic is an offset of a plane, a cone or cylinder, a sphere or a
// torus as the surface of the same kind it is (package 27.5), its
// parameters perhaps shifted, which the importer finds again by
// projecting; ok is false for another base, or an offset through the
// base's axis or centre.
func (s *offset) analytic() (Surface, bool) {
	// The side the offset goes: the base's normal, su x sv, at a point,
	// against the way out from its axis or centre.
	side := func(out geom.Vector) float64 {
		_, su, sv := s.base.Derivs(0.3, 0.2)
		if su.Cross(sv).Dot(out) < 0 {
			return -1
		}
		return 1
	}
	switch b := s.base.(type) {
	case *plane:
		n := b.f.x.Cross(b.f.y).WithMagnitude(1)
		f := b.f
		f.o = f.o.Plus(n.ScaledBy(s.d))
		return &plane{f: f}, true
	case *cone:
		beta := math.Atan(b.tan)
		p := b.Eval(0.3, 0.2)
		axial := b.f.z
		radial := p.Minus(b.f.o).Minus(axial.ScaledBy(p.Minus(b.f.o).Dot(axial))).WithMagnitude(1)
		out := radial.ScaledBy(math.Cos(beta)).Minus(axial.ScaledBy(math.Sin(beta)))
		r := b.r + side(out)*s.d/math.Cos(beta)
		if !(math.Abs(r) > 0) || b.tan == 0 && r < 0 {
			return nil, false
		}
		return &cone{f: b.f, r: r, tan: b.tan}, true
	case *sphere:
		r := b.r + side(b.Eval(0.3, 0.2).Minus(b.f.o))*s.d
		if !(r > 0) {
			return nil, false
		}
		return &sphere{f: b.f, r: r}, true
	case *torus:
		p := b.Eval(0.3, 0.2)
		tube := b.f.o.Plus(b.f.x.ScaledBy(b.major * math.Cos(0.3))).Plus(b.f.y.ScaledBy(b.major * math.Sin(0.3)))
		r := b.r + side(p.Minus(tube))*s.d
		if !(r > 0) {
			return nil, false
		}
		return &torus{f: b.f, major: b.major, r: r}, true
	}
	return nil, false
}

// offsetCells is the grid of an offset of a surface of another kind:
// its base's lines, and between them more where a curve of degree
// srf.MaxDegree through the offset's points at Chebyshev-Lobatto points
// strays from it along a line across, by halving; on each cell a patch
// of that degree through such points, when it lies within lowDegreeTol
// of the offset; else the face is one the importer can't hold.
func offsetCells(s *offset) (*surfaceCells, error) {
	base, err := cellsOf(s.base)
	if err != nil {
		return nil, err
	}
	u0, u1, v0, v1 := s.base.Domain()
	across := func(lo, hi float64) []float64 {
		if math.IsInf(lo, 0) || math.IsInf(hi, 0) {
			lo, hi = -1, 1
		}
		return []float64{lo, lo + (hi-lo)/4, (lo + hi) / 2, hi - (hi-lo)/4, hi}
	}
	vs, us := across(v0, v1), across(u0, u1)
	nodes := make([]float64, srf.MaxDegree+1)
	for k := range nodes {
		nodes[k] = (1 - math.Cos(float64(k)*math.Pi/srf.MaxDegree)) / 2
	}
	// fits reports whether the offset along one way, from a to b, at
	// each of the others, is a curve of degree MaxDegree within a tenth
	// of the tolerance.
	fits := func(a, b float64, at func(t, o float64) geom.Vector, others []float64) bool {
		for _, o := range others {
			var pts []geom.Vector
			for _, x := range nodes {
				pts = append(pts, at(a+(b-a)*x, o))
			}
			c := srf.InterpolatedCurve(pts)
			for k := 0; k <= 16; k++ {
				x := float64(k) / 16
				if c.PointAt(x).Minus(at(a+(b-a)*x, o)).Magnitude() > lowDegreeTol/10 {
					return false
				}
			}
		}
		return true
	}
	refine := func(br func(lo, hi float64) []float64, at func(t, o float64) geom.Vector, others []float64) func(lo, hi float64) []float64 {
		return func(lo, hi float64) []float64 {
			lines := append(append([]float64{lo}, br(lo, hi)...), hi)
			var out []float64
			var split func(a, b float64, depth int)
			split = func(a, b float64, depth int) {
				if depth < 6 && !fits(a, b, at, others) {
					m := (a + b) / 2
					split(a, m, depth+1)
					out = append(out, m)
					split(m, b, depth+1)
				}
			}
			for i := 0; i+1 < len(lines); i++ {
				if i > 0 {
					out = append(out, lines[i])
				}
				split(lines[i], lines[i+1], 0)
			}
			return out
		}
	}
	return &surfaceCells{
		breaksU: refine(base.breaksU, func(t, o float64) geom.Vector { return s.Eval(t, o) }, vs),
		breaksV: refine(base.breaksV, func(t, o float64) geom.Vector { return s.Eval(o, t) }, us),
		patch: func(ua, ub, va, vb float64) (srf.SSurface, error) {
			p, err := srf.InterpolatedPatch(srf.MaxDegree, srf.MaxDegree, func(x, y float64) geom.Vector {
				return s.Eval(ua+(ub-ua)*x, va+(vb-va)*y)
			}, lowDegreeTol)
			if err != nil {
				return srf.SSurface{}, errUnsupported("an offset surface that no patch of degree 5 follows")
			}
			return p, nil
		}}, nil
}
