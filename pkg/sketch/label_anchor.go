package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/res"
)

// labelAnchor is where the window centres c's label reading text, at
// view v; ok is false for a kind of constraint whose label place this
// does not know.
func (sk *Sketch) labelAnchor(v *View, c *core.Constraint, text string) (geom.Vector, bool) {
	switch c.Type {
	case core.ConstraintPtPtDistance, core.ConstraintProjPtDistance,
		core.ConstraintPtLineDistance, core.ConstraintPtPlaneDistance, core.ConstraintPtFaceDistance,
		core.ConstraintLengthRatio, core.ConstraintLengthDifference,
		core.ConstraintArcLineLenRatio, core.ConstraintArcLineDifference:
		a, b, ok := sk.labelEnds(c)
		if !ok {
			return geom.Vector{}, false
		}
		return a.Plus(b).ScaledBy(0.5).Plus(c.Disp.Offset), true
	case core.ConstraintDiameter, core.ConstraintArcArcLenRatio, core.ConstraintArcArcDifference:
		circle := sk.Entity(c.EntityA)
		if circle == nil {
			return geom.Vector{}, false
		}
		center := sk.MustEntity(circle.Point[0]).PointGetNum(sk)
		n := sk.MustEntity(circle.Normal).NormalGetNum(sk).RotationN().WithMagnitude(1)
		ref := center.Plus(c.Disp.Offset)
		return ref.Minus(n.ScaledBy(n.Dot(ref) - n.Dot(center))), true
	case core.ConstraintAngle:
		return sk.angleLabelAnchor(v, c, text)
	case core.ConstraintComment:
		if c.PtA.V == 0 {
			return c.Disp.Offset, true
		}
		pt := sk.Entity(c.PtA)
		if pt == nil {
			return geom.Vector{}, false
		}
		return pt.PointGetNum(sk).Plus(c.Disp.Offset), true
	}
	return geom.Vector{}, false
}

// angleLabelAnchor is where the window centres an angle's label: off
// the corner by the label's offset, but never nearer it than the arc's
// least radius and half the label's diagonal; between the lines' ends
// where they are skew.
func (sk *Sketch) angleLabelAnchor(v *View, c *core.Constraint, text string) (geom.Vector, bool) {
	a0, da, b0, db, ok := sk.angleLines(c)
	if !ok {
		return geom.Vector{}, false
	}
	pi, skew := geom.AtIntersectionOfLinesSkew(a0, a0.Plus(da), b0, b0.Plus(db))
	if skew {
		return a0.Plus(b0).ScaledBy(0.5).Plus(c.Disp.Offset), true
	}
	pixels := 1 / v.Scale
	th := sk.StyleTextHeight(c.StyleOrDefault(), v.Scale)
	vf := res.BuiltinVectorFont()
	w := (vf.Width(th, text) + 8) * pixels
	h := (vf.GetCapHeight(th) + 6) * pixels
	textR := math.Hypot(w, h) / 2
	rm := c.Disp.Offset
	return pi.Plus(rm.WithMagnitude(max(rm.Magnitude(), 15*pixels+textR))), true
}

// angleLines is the two lines an angle is between, a0+t*da and
// b0+t*db, the first turned about where the angle is the supplement,
// in the constraint's workplane as the window draws them; ok is false
// where they are parallel and meet nowhere.
func (sk *Sketch) angleLines(c *core.Constraint) (a0, da, b0, db geom.Vector, ok bool) {
	a, b := sk.Entity(c.EntityA), sk.Entity(c.EntityB)
	if a == nil || b == nil {
		return a0, da, b0, db, false
	}
	a0, da = a.VectorGetStartPoint(sk), a.VectorGetNum(sk)
	b0, db = b.VectorGetStartPoint(sk), b.VectorGetNum(sk)
	if c.Other {
		a0 = a0.Plus(da)
		da = da.ScaledBy(-1)
	}
	if !core.IsFreeIn3D(c.Workplane) {
		a0, b0 = sk.intoWorkplane(a0, c.Workplane), sk.intoWorkplane(b0, c.Workplane)
		da = core.ProjectVectorInto(sk, da, c.Workplane)
		db = core.ProjectVectorInto(sk, db, c.Workplane)
	}
	if da.Cross(db).Magnitude() < geom.LengthEps*max(da.Magnitude(), db.Magnitude()) {
		return a0, da, b0, db, false
	}
	return a0, da, b0, db, true
}

// labelEnds is the two ends of the line a dimension is drawn along: a
// distance's two points, or a point and the nearest point of its line,
// workplane or face, projected into the constraint's workplane as the
// window draws them; the line segment a ratio or a difference puts its
// label by.
func (sk *Sketch) labelEnds(c *core.Constraint) (a, b geom.Vector, ok bool) {
	switch c.Type {
	case core.ConstraintLengthRatio, core.ConstraintLengthDifference:
		return sk.segmentEnds(c.EntityB, c.Workplane)
	case core.ConstraintArcLineLenRatio, core.ConstraintArcLineDifference:
		return sk.segmentEnds(c.EntityA, core.FreeIn3D)
	}
	pa := sk.Entity(c.PtA)
	if pa == nil {
		return a, b, false
	}
	a = pa.PointGetNum(sk)
	switch c.Type {
	case core.ConstraintPtPtDistance, core.ConstraintProjPtDistance:
		pb := sk.Entity(c.PtB)
		if pb == nil {
			return a, b, false
		}
		b = pb.PointGetNum(sk)
		if c.Type == core.ConstraintPtPtDistance {
			a, b = sk.intoWorkplane(a, c.Workplane), sk.intoWorkplane(b, c.Workplane)
		}
	case core.ConstraintPtLineDistance:
		l := sk.Entity(c.EntityA)
		if l == nil {
			return a, b, false
		}
		la := sk.intoWorkplane(sk.MustEntity(l.Point[0]).PointGetNum(sk), c.Workplane)
		lb := sk.intoWorkplane(sk.MustEntity(l.Point[1]).PointGetNum(sk), c.Workplane)
		a = sk.intoWorkplane(a, c.Workplane)
		b = a.ClosestPointOnLine(la, lb.Minus(la))
	case core.ConstraintPtPlaneDistance, core.ConstraintPtFaceDistance:
		e := sk.Entity(c.EntityA)
		if e == nil {
			return a, b, false
		}
		var n, p geom.Vector
		if c.Type == core.ConstraintPtPlaneDistance {
			n = e.NormalEntity(sk).NormalN(sk)
			p = e.WorkplaneGetOffset(sk)
		} else {
			n = e.FaceGetNormalNum(sk)
			p = e.FaceGetPointNum(sk)
		}
		b = a.Plus(n.WithMagnitude(p.Minus(a).Dot(n)))
	}
	return a, b, true
}

// segmentEnds is line segment l's ends, projected into workplane wp.
func (sk *Sketch) segmentEnds(l, wp core.HEntity) (a, b geom.Vector, ok bool) {
	e := sk.Entity(l)
	if e == nil {
		return a, b, false
	}
	a = sk.intoWorkplane(sk.MustEntity(e.Point[0]).PointGetNum(sk), wp)
	b = sk.intoWorkplane(sk.MustEntity(e.Point[1]).PointGetNum(sk), wp)
	return a, b, true
}

// intoWorkplane projects p into workplane wp, or leaves it in 3D.
func (sk *Sketch) intoWorkplane(p geom.Vector, wp core.HEntity) geom.Vector {
	if core.IsFreeIn3D(wp) {
		return p
	}
	w := sk.Entity(wp)
	if w == nil {
		return p
	}
	p0 := w.WorkplaneGetOffset(sk)
	return p0.Plus(core.ProjectVectorInto(sk, p.Minus(p0), wp))
}

// labelShiftDir is the way c's label is moved to clear another: out
// along an angle's bisector, on the side the label is on; across the
// line a distance measures along, in the view, away from it where the
// label is off it already; up the screen for a diameter, or where the
// line is seen end on.
func (sk *Sketch) labelShiftDir(c *core.Constraint, v *View) geom.Vector {
	up := v.ProjUp.WithMagnitude(1)
	var dir geom.Vector
	switch c.Type {
	case core.ConstraintDiameter, core.ConstraintArcArcLenRatio, core.ConstraintArcArcDifference:
		return up
	case core.ConstraintAngle:
		_, da, _, db, ok := sk.angleLines(c)
		if !ok {
			return up
		}
		dir = da.WithMagnitude(1).Plus(db.WithMagnitude(1))
		if dir.Magnitude() < geom.LengthEps {
			return up
		}
	default:
		a, b, ok := sk.labelEnds(c)
		if !ok {
			return up
		}
		n := v.ProjRight.Cross(v.ProjUp)
		dir = n.Cross(a.Minus(b))
		if dir.Magnitude() < geom.LengthEps && c.Type == core.ConstraintPtLineDistance {
			// The point is on the line: along the line.
			l := sk.MustEntity(c.EntityA)
			dir = sk.MustEntity(l.Point[1]).PointGetNum(sk).Minus(sk.MustEntity(l.Point[0]).PointGetNum(sk))
		}
		if dir.Magnitude() < geom.LengthEps {
			return up
		}
	}
	if c.Disp.Offset.Dot(dir) < 0 {
		dir = dir.ScaledBy(-1)
	}
	return dir.WithMagnitude(1)
}
