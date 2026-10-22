package api

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/persist/link"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// drawIn makes wp the workplane drawn in, if it isn't already.
func (w *scriptWriter) drawIn(wp core.HEntity) {
	if wp == w.wp {
		return
	}
	if core.IsFreeIn3D(wp) {
		w.line("doc.Sketch.Anywhere3D()")
	} else {
		w.line("doc.Sketch.InWorkplane(%s)", w.ref(wp))
	}
	w.wp = wp
}

// writeRequests draws the group's requests where they are solved to.
func (w *scriptWriter) writeRequests(g *sketch.Group) {
	sk := w.sk
	for i := range sk.Requests {
		r := &sk.Requests[i]
		if r.Group != g.H {
			continue
		}
		call := w.requestCall(r)
		if call == "" {
			continue
		}
		w.drawIn(r.Workplane)
		v := fmt.Sprintf("r%d", r.H.V)
		w.reqVar[r.H] = v
		w.line("%s := %s", v, call)
		if r.Type == sketch.RequestCircle && core.IsFreeIn3D(r.Workplane) {
			if n := sk.Entity(r.H.Entity(32)); n != nil {
				if q := n.NormalGetNum(sk); q != (geom.Quaternion{W: 1}) {
					u, vv := uvOf(q)
					w.line("%s.SetOrientation(%s, %s)", v, vecText(u), vecText(vv))
				}
			}
		}
		if r.Type == sketch.RequestTTFText && r.ExtraPoints != 0 {
			w.line("%s.SetKerning(true)", v)
		}
		switch {
		case r.Type == sketch.RequestImage && !r.Construction:
			// An image is drawn as construction.
			w.line("%s.SetConstruction(false)", v)
		case r.Type != sketch.RequestImage && r.Construction:
			w.line("%s.SetConstruction(true)", v)
		}
		if r.Style.V != 0 {
			if st, ok := w.styleRef(r.Style); ok {
				w.line("%s.Request().SetStyle(%s)", v, st)
			}
		}
	}
}

// requestCall is the call that draws a request like r, "" for one no
// call draws, said in the script.
func (w *scriptWriter) requestCall(r *sketch.Request) string {
	sk := w.sk
	at := func(i int) string {
		if e := sk.Entity(r.H.Entity(i)); e != nil && e.IsPoint() {
			return vecText(e.PointGetNum(sk))
		}
		return "api.V(0, 0, 0)"
	}
	pos := func(i int) geom.Vector {
		if e := sk.Entity(r.H.Entity(i)); e != nil && e.IsPoint() {
			return e.PointGetNum(sk)
		}
		return geom.Vector{}
	}
	switch r.Type {
	case sketch.RequestDatumPoint:
		return "doc.Sketch.Point(" + at(0) + ")"
	case sketch.RequestLineSegment:
		return "doc.Sketch.Line(" + at(1) + ", " + at(2) + ")"
	case sketch.RequestCircle:
		radius := 0.0
		if e := sk.Entity(r.H.Entity(0)); e != nil {
			radius = math.Abs(e.CircleGetRadiusNum(sk))
		}
		return "doc.Sketch.Circle(" + at(1) + ", " + num(radius) + ")"
	case sketch.RequestArcOfCircle:
		if core.IsFreeIn3D(r.Workplane) {
			// Drawn in 3d, as a DXF import draws an arc, about its own
			// normal.
			u, v := w.normalUV(r)
			return "doc.Sketch.ArcIn3D(" + at(1) + ", " + at(2) + ", " + at(3) + ", " + vecText(u) + ", " + vecText(v) + ")"
		}
		return "doc.Sketch.Arc(" + at(1) + ", " + at(2) + ", " + at(3) + ")"
	case sketch.RequestCubic:
		if r.ExtraPoints != 0 {
			return "doc.Sketch.Spline(" + w.points(r, 4+r.ExtraPoints) + ")"
		}
		return "doc.Sketch.Cubic(" + at(1) + ", " + at(2) + ", " + at(3) + ", " + at(4) + ")"
	case sketch.RequestCubicPeriodic:
		return "doc.Sketch.PeriodicSpline(" + w.points(r, 3+r.ExtraPoints) + ")"
	case sketch.RequestImage:
		path := link.Path(w.file, r.File)
		height := pos(1).Minus(pos(2)).Magnitude()
		if _, err := os.Stat(path); err != nil {
			// Its frame and path, with no pixels, as the drawing has it.
			w.notes = append(w.notes, fmt.Sprintf("r%d, an image whose file %s isn't there: its frame is written", r.H.V, r.File))
			width := pos(3).Minus(pos(2)).Magnitude()
			return fmt.Sprintf("doc.Sketch.ImageFrame(%q, %s, %s, %s)", path, at(2), num(height), num(width))
		}
		return fmt.Sprintf("doc.Sketch.Image(%q, %s, %s)", path, at(2), num(height))
	case sketch.RequestWorkplane:
		if !core.IsFreeIn3D(r.Workplane) {
			// Drawn in a workplane, as a DXF import draws one, and so
			// parallel to it.
			return "doc.Sketch.NewWorkplaneIn(" + at(1) + ")"
		}
		u, v := w.normalUV(r)
		return "doc.Sketch.NewWorkplane(" + at(1) + ", " + vecText(snapVec(u)) + ", " + vecText(snapVec(v)) + ")"
	case sketch.RequestTTFText:
		height := pos(1).Minus(pos(2)).Magnitude()
		return fmt.Sprintf("doc.Sketch.Text(%q, %q, %s, %s)", r.Str, r.Font, at(2), num(height))
	}
	w.notWritten("r%d, a request of a kind no call draws (%d)", r.H.V, int(r.Type))
	return ""
}

// normalUV are the directions of the request's normal, which a
// workplane and an arc in 3d have as their entity 32.
func (w *scriptWriter) normalUV(r *sketch.Request) (u, v geom.Vector) {
	if n := w.sk.Entity(r.H.Entity(32)); n != nil {
		return uvOf(n.NormalGetNum(w.sk))
	}
	return geom.Vector{X: 1}, geom.Vector{Y: 1}
}

// points are the first n points of the request, as a call takes them.
func (w *scriptWriter) points(r *sketch.Request, n int) string {
	parts := make([]string, n)
	for i := range n {
		p := geom.Vector{}
		if e := w.sk.Entity(r.H.Entity(1 + i)); e != nil && e.IsPoint() {
			p = e.PointGetNum(w.sk)
		}
		parts[i] = vecText(p)
	}
	return strings.Join(parts, ", ")
}

// writeConstraints writes the group's constraints.
func (w *scriptWriter) writeConstraints(g *sketch.Group) {
	sk := w.sk
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Group != g.H {
			continue
		}
		w.writeConstraint(c)
	}
}

// writeConstraint writes one constraint, by type, then what it holds.
func (w *scriptWriter) writeConstraint(c *core.Constraint) {
	w.drawIn(c.Workplane)
	v := fmt.Sprintf("c%d", c.H.V)
	if c.Type == core.ConstraintComment {
		switch first, ok := w.written(c); {
		case ok:
			// The same words at the same place, twice, as a DXF import
			// writes them.
			w.line("%s := doc.Constrain.Again(%s)", v, first)
		case c.PtA.V != 0:
			w.line("%s := doc.Constrain.CommentOn(%s, %q)", v, w.ref(c.PtA), c.Comment)
			w.line("%s.SetLabelOffset(%s)", v, vecText(c.Disp.Offset))
		default:
			w.line("%s := doc.Constrain.Comment(%s, %q)", v, vecText(c.Disp.Offset), c.Comment)
		}
		w.conVar[c.H] = v
		if st, ok := w.styleRef(c.Disp.Style); ok && c.Disp.Style.V != 0 {
			w.line("%s.SetStyle(%s)", v, st)
		}
		return
	}
	if first, ok := w.written(c); ok {
		// An equal constraint is written already: the drawing has it
		// twice, as a DXF import makes it.
		w.line("%s := doc.Constrain.Again(%s)", v, first)
		w.conVar[c.H] = v
		w.writeConstraintSettings(c, v)
		return
	}
	name, ok := constraintTypeNames[c.Type]
	if !ok {
		w.notWritten("constraint %08x of type %d", c.H.V, int(c.Type))
		return
	}
	other := c.Other
	if c.Type == core.ConstraintDiameter {
		// Shown as a radius, set after.
		other = false
	}
	if c.EntityC.V != 0 || c.EntityD.V != 0 {
		w.line("%s := doc.Constrain.RawFour(core.%s, %s, %s, %s, %s, %s, %s, %v, %v)", v, name,
			w.ref(c.PtA), w.ref(c.PtB), w.ref(c.EntityA), w.ref(c.EntityB), w.ref(c.EntityC), w.ref(c.EntityD), other, c.Other2)
	} else {
		w.line("%s := doc.Constrain.Raw(core.%s, %s, %s, %s, %s, %v, %v)", v, name,
			w.ref(c.PtA), w.ref(c.PtB), w.ref(c.EntityA), w.ref(c.EntityB), other, c.Other2)
	}
	w.conVar[c.H] = v
	w.writeConstraintSettings(c, v)
}

// writeConstraintSettings writes what is set on constraint c, written
// as v, after it is made: its value, label, limit, reference and name.
func (w *scriptWriter) writeConstraintSettings(c *core.Constraint, v string) {
	if c.HasLabel() {
		if c.Type == core.ConstraintDiameter && c.Other {
			w.line("%s.SetShowAsRadius(true)", v)
		}
		switch {
		case c.Expr != "":
			// As typed in the label: a size, its side and a radius kept
			// by the constraint.
			w.line("must(%s.SetExpr(%q))", v, sketch.ConstraintText(c))
		default:
			w.line("%s.Set(%s)", v, num(constraintSetValue(c)))
		}
		if c.Disp.Offset != (geom.Vector{}) {
			w.line("%s.SetLabelOffset(%s)", v, vecText(c.Disp.Offset))
		}
	}
	if c.Limit != core.LimitNone {
		w.line("%s.SetLimit(core.Limit(%d))", v, int(c.Limit))
	}
	if c.Reference {
		w.line("%s.SetReference(true)", v)
	}
	if c.Name != "" {
		w.line("%s.SetName(%q)", v, c.Name)
	}
}

// written is the variable of a constraint written before c that equals
// it, by the rule the sketch keeps one of a kind by: the same kind,
// entities and value, and for a comment the same place.
func (w *scriptWriter) written(c *core.Constraint) (string, bool) {
	for i := range w.sk.Constraints {
		cc := &w.sk.Constraints[i]
		if cc.H == c.H {
			break
		}
		v, ok := w.conVar[cc.H]
		if ok && c.ConstraintBase.Equals(&cc.ConstraintBase) && //nolint:staticcheck // QF1008: the base is named on purpose
			(c.Type != core.ConstraintComment || c.Disp.Offset == cc.Disp.Offset) {
			return v, true
		}
	}
	return "", false
}

// constraintSetValue is the value Constraint.Set takes to give c the
// value it has.
func constraintSetValue(c *core.Constraint) float64 {
	switch c.Type {
	case core.ConstraintAxisAngle:
		return c.ValA
	case core.ConstraintDiameter:
		if c.Other {
			return math.Abs(c.ValA) / 2
		}
		return math.Abs(c.ValA)
	}
	return math.Abs(c.ValA)
}

// constraintTypeNames are the core package's names of the constraint
// types, as a script writes them.
var constraintTypeNames = map[core.ConstraintType]string{
	core.ConstraintPointsCoincident:  "ConstraintPointsCoincident",
	core.ConstraintPtPtDistance:      "ConstraintPtPtDistance",
	core.ConstraintPtPlaneDistance:   "ConstraintPtPlaneDistance",
	core.ConstraintPtLineDistance:    "ConstraintPtLineDistance",
	core.ConstraintPtFaceDistance:    "ConstraintPtFaceDistance",
	core.ConstraintProjPtDistance:    "ConstraintProjPtDistance",
	core.ConstraintPtInPlane:         "ConstraintPtInPlane",
	core.ConstraintPtOnLine:          "ConstraintPtOnLine",
	core.ConstraintPtOnFace:          "ConstraintPtOnFace",
	core.ConstraintPtOnSegment:       "ConstraintPtOnSegment",
	core.ConstraintEqualLengthLines:  "ConstraintEqualLengthLines",
	core.ConstraintLengthRatio:       "ConstraintLengthRatio",
	core.ConstraintEqLenPtLineD:      "ConstraintEqLenPtLineD",
	core.ConstraintEqPtLnDistances:   "ConstraintEqPtLnDistances",
	core.ConstraintEqualAngle:        "ConstraintEqualAngle",
	core.ConstraintEqualLineArcLen:   "ConstraintEqualLineArcLen",
	core.ConstraintLengthDifference:  "ConstraintLengthDifference",
	core.ConstraintSymmetric:         "ConstraintSymmetric",
	core.ConstraintSymmetricHoriz:    "ConstraintSymmetricHoriz",
	core.ConstraintSymmetricVert:     "ConstraintSymmetricVert",
	core.ConstraintSymmetricLine:     "ConstraintSymmetricLine",
	core.ConstraintAtMidpoint:        "ConstraintAtMidpoint",
	core.ConstraintHorizontal:        "ConstraintHorizontal",
	core.ConstraintVertical:          "ConstraintVertical",
	core.ConstraintDiameter:          "ConstraintDiameter",
	core.ConstraintPtOnCircle:        "ConstraintPtOnCircle",
	core.ConstraintSameOrientation:   "ConstraintSameOrientation",
	core.ConstraintAngle:             "ConstraintAngle",
	core.ConstraintParallel:          "ConstraintParallel",
	core.ConstraintPerpendicular:     "ConstraintPerpendicular",
	core.ConstraintArcLineTangent:    "ConstraintArcLineTangent",
	core.ConstraintCubicLineTangent:  "ConstraintCubicLineTangent",
	core.ConstraintCurveCurveTangent: "ConstraintCurveCurveTangent",
	core.ConstraintEqualRadius:       "ConstraintEqualRadius",
	core.ConstraintAxisAngle:         "ConstraintAxisAngle",
	core.ConstraintWhereDragged:      "ConstraintWhereDragged",
	core.ConstraintArcArcLenRatio:    "ConstraintArcArcLenRatio",
	core.ConstraintArcLineLenRatio:   "ConstraintArcLineLenRatio",
	core.ConstraintArcArcDifference:  "ConstraintArcArcDifference",
	core.ConstraintArcLineDifference: "ConstraintArcLineDifference",
}
