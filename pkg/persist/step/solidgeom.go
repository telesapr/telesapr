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
			for _, k := range cc.Breaks() {
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
	}
	return out
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
	case *bsplineCurve:
		bc := cc.srfCurve()
		if bc.Deg > srf.MaxDegree {
			return srf.SBezier{}, errUnsupported(fmt.Sprintf("a curve of degree %d", bc.Deg))
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
	case *bsplineSurface:
		bs := ss.srfSurface()
		if bs.DegU > srf.MaxDegree || bs.DegV > srf.MaxDegree {
			return nil, errUnsupported(fmt.Sprintf("a surface of degree %d by %d", bs.DegU, bs.DegV))
		}
		ps, err := bs.Patches()
		if err != nil {
			return nil, err
		}
		bu, bv := bs.Breaks()
		inside := func(br []float64) func(lo, hi float64) []float64 {
			return func(lo, hi float64) []float64 {
				var out []float64
				for _, k := range br {
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
