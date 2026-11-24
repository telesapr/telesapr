package export

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// lineOrCircleOf tells whether sb is a line, or an arc of a circle of
// centre c, normal n and radius r, as the file writes it.
func lineOrCircleOf(sb *srf.SBezier) (line bool, c, n geom.Vector, r float64, ok bool) {
	a, b := sb.Start(), sb.Finish()
	if sb.Degree == 1 {
		return true, c, n, 0, b.Minus(a).Magnitude() >= geom.LengthEps
	}
	if sb.Degree != 2 {
		return false, c, n, 0, false
	}
	c, n, r, ok = circleThrough(a, sb.PointAt(0.5), b)
	if !ok || r < geom.LengthEps {
		return false, c, n, 0, false
	}
	tol := 1e-7*r + geom.LengthEps/10
	for _, t := range []float64{0.125, 0.25, 0.375, 0.625, 0.75, 0.875} {
		p := sb.PointAt(t)
		if math.Abs(p.Minus(c).Magnitude()-r) > tol || math.Abs(p.Minus(c).Dot(n)) > tol {
			return false, c, n, 0, false
		}
	}
	return false, c, n, r, true
}

// writeLineOrCircle writes sb as curve id when it is a line or an arc of
// a circle, as a LINE or a CIRCLE whose parameter runs from sb's start
// to its finish, so the edge along it is the same sense; the entities it
// needs besides take ids from next on. It returns the id after the last
// it wrote; false, writing nothing, for any other curve.
func (w *stepFileWriter) writeLineOrCircle(id, next int, sb *srf.SBezier) (int, bool) {
	line, c, n, r, ok := lineOrCircleOf(sb)
	if !ok {
		return 0, false
	}
	a := sb.Start()
	if line {
		d := sb.Finish().Minus(a)
		w.printf("#%d=DIRECTION('',(%s));\n", next, stepDirection(d))
		w.printf("#%d=VECTOR('',#%d,%s);\n", next+1, next, cfmtReal(d.Magnitude()))
		w.printf("#%d=LINE('',#%d,#%d);\n", id, w.InsertPoint(id+1), next+1)
		return next + 2, true
	}
	w.id = next
	place := w.writePlacement(c, n, a.Minus(c))
	w.printf("#%d=CIRCLE('',#%d,%s);\n", id, place, cfmtReal(r))
	return w.id, true
}
