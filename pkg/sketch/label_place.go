package sketch

import (
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/res"
)

// A new dimension's label is put where the menu puts it, and two
// dimensions on the same thing (a minimum and a maximum on one line, a
// distance from a circle's centre and its diameter) put theirs at the
// same place, one over the other, until the user drags one away. So a
// new label that would land on a label its group shows already is
// moved along the dimension's normal, one label height at a time, until
// it is clear. The boxes are those the window draws, from the vector
// font, at the sketch's view scale; the new label's text is taken as a
// nominal one, so that the place does not hang on the value it is made
// with: the window makes a dimension at what it measures and a script
// at the value it gives, and both put the label at the same place.

// labelNominalText stands for the text of a label not yet set.
const labelNominalText = "min 000.00"

// labelMarginPixels is the gap kept round a label, as the window keeps
// round it when it trims the dimension's lines.
const labelMarginPixels = 4

// labelMaxSteps bounds how far a label is moved; past it the label is
// left where the menu puts it.
const labelMaxSteps = 24

// labelBox is a label's box on the screen, in pixels: its centre and
// its half sizes, margin included.
type labelBox struct {
	u, v, hw, hh float64
}

func (b labelBox) overlaps(o labelBox) bool {
	return math.Abs(b.u-o.u) < b.hw+o.hw && math.Abs(b.v-o.v) < b.hh+o.hh
}

// ClearNewLabel moves the label of c, a dimension about to be added to
// its group, along the dimension's normal until its box is clear of
// the labels the group has, as the view shows them. A kind of
// constraint whose label place this does not know (an angle, a ratio,
// a comment) is left as it is, and so is a label that stays on another
// after labelMaxSteps.
func (sk *Sketch) ClearNewLabel(c *core.Constraint) {
	if !c.HasLabel() || c.Type == core.ConstraintComment {
		return
	}
	v := sk.view()
	if v.Scale <= 0 {
		return
	}
	if _, ok := sk.labelAnchor(c); !ok {
		return
	}
	var others []labelBox
	for i := range sk.Constraints {
		o := &sk.Constraints[i]
		if o.Group != c.Group || !o.HasLabel() {
			continue
		}
		if a, ok := sk.labelAnchor(o); ok {
			others = append(others, sk.labelBoxAt(v, a, o, labelText(o)))
		}
	}
	isClear := func(off geom.Vector) bool {
		moved := *c
		moved.Disp.Offset = c.Disp.Offset.Plus(off)
		at, _ := sk.labelAnchor(&moved)
		b := sk.labelBoxAt(v, at, c, labelNominalText)
		return !slices.ContainsFunc(others, b.overlaps)
	}
	if isClear(geom.Vector{}) {
		return
	}
	dir := sk.labelShiftDir(c, v)
	th := sk.StyleTextHeight(c.StyleOrDefault(), v.Scale)
	step := dir.WithMagnitude((th + 2*labelMarginPixels) / v.Scale)
	for k := 1; k <= labelMaxSteps; k++ {
		off := step.ScaledBy(float64(k))
		if isClear(off) {
			c.Disp.Offset = c.Disp.Offset.Plus(off)
			return
		}
	}
}

// labelBoxAt is the box of a label of c reading text, centred at p.
func (sk *Sketch) labelBoxAt(v *View, p geom.Vector, c *core.Constraint, text string) labelBox {
	th := sk.StyleTextHeight(c.StyleOrDefault(), v.Scale)
	w := res.BuiltinVectorFont().Width(th, text)
	q := p.Plus(v.Offset)
	return labelBox{
		u:  q.Dot(v.ProjRight) * v.Scale,
		v:  q.Dot(v.ProjUp) * v.Scale,
		hw: w/2 + labelMarginPixels,
		hh: th/2 + labelMarginPixels,
	}
}

// labelText is near enough the text the window shows on c's label for
// its width: the window's own text takes the user's units.
func labelText(c *core.Constraint) string {
	var s string
	switch c.Type {
	case core.ConstraintAngle, core.ConstraintAxisAngle:
		s = fmt.Sprintf("%.1f°", c.ValA)
	case core.ConstraintLengthRatio, core.ConstraintArcArcLenRatio, core.ConstraintArcLineLenRatio:
		s = fmt.Sprintf("%.3f:1", c.ValA)
	case core.ConstraintComment:
		return c.Comment
	case core.ConstraintDiameter:
		if c.Other {
			s = fmt.Sprintf("R%.2f", c.ValA/2)
		} else {
			s = fmt.Sprintf("⌀%.2f", c.ValA)
		}
	default:
		s = fmt.Sprintf("%.2f", math.Abs(c.ValA))
	}
	if c.Limit != core.LimitNone {
		s = c.Limit.String() + " " + s
	}
	if c.Reference {
		s += " REF"
	}
	if c.Name != "" {
		s = c.Name + " = " + s
	}
	return s
}

// labelAnchor is where the window centres c's label, for the kinds of
// constraint whose label sits off what they measure.
func (sk *Sketch) labelAnchor(c *core.Constraint) (geom.Vector, bool) {
	switch c.Type {
	case core.ConstraintPtPtDistance, core.ConstraintProjPtDistance:
		a, b, ok := sk.labelEnds(c)
		if !ok {
			return geom.Vector{}, false
		}
		return a.Plus(b).ScaledBy(0.5).Plus(c.Disp.Offset), true
	case core.ConstraintPtLineDistance, core.ConstraintPtPlaneDistance, core.ConstraintPtFaceDistance:
		pt, closest, ok := sk.labelEnds(c)
		if !ok {
			return geom.Vector{}, false
		}
		return pt.Plus(closest).ScaledBy(0.5).Plus(c.Disp.Offset), true
	case core.ConstraintDiameter:
		circle := sk.Entity(c.EntityA)
		if circle == nil {
			return geom.Vector{}, false
		}
		center := sk.MustEntity(circle.Point[0]).PointGetNum(sk)
		n := sk.MustEntity(circle.Normal).NormalGetNum(sk).RotationN().WithMagnitude(1)
		ref := center.Plus(c.Disp.Offset)
		return ref.Minus(n.ScaledBy(n.Dot(ref) - n.Dot(center))), true
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

// labelEnds is the two ends of the line a distance is drawn along: its
// two points, or a point and the nearest point of its line, workplane
// or face, projected into the constraint's workplane as the window
// draws them.
func (sk *Sketch) labelEnds(c *core.Constraint) (a, b geom.Vector, ok bool) {
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

// labelShiftDir is the way c's label is moved to clear another: across
// the line the dimension measures along, in the view, away from it
// where the label is off it already; up the screen for a diameter, or
// where the line is seen end on.
func (sk *Sketch) labelShiftDir(c *core.Constraint, v *View) geom.Vector {
	up := v.ProjUp.WithMagnitude(1)
	if c.Type == core.ConstraintDiameter {
		return up
	}
	a, b, ok := sk.labelEnds(c)
	if !ok {
		return up
	}
	n := v.ProjRight.Cross(v.ProjUp)
	dir := n.Cross(a.Minus(b))
	if dir.Magnitude() < geom.LengthEps {
		if c.Type == core.ConstraintPtLineDistance {
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
