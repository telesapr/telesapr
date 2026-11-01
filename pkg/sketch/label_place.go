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
// distance from a circle's centre and its diameter, two angles at one
// corner) put theirs at the same place, one over the other, until the
// user drags one away; a diameter's label and an angle's sit on the
// circle's centre and the corner, over the point drawn there. So a new
// label that would land on a label its group shows already, or on a
// point the view shows, is moved, one label height at a time, until it
// is clear: a distance's across what it measures, an angle's out along
// its bisector, a diameter's up the screen. The boxes are those the
// window draws, from the vector font, at the sketch's view scale; the
// new label's text is taken as a nominal one, so that the place does
// not hang on the value it is made with: the window makes a dimension
// at what it measures and a script at the value it gives, and both put
// the label at the same place.

// labelNominalText stands for the text of a label not yet set.
const labelNominalText = "min 000.00"

// labelMarginPixels is the gap kept round a label, as the window keeps
// round it when it trims the dimension's lines.
const labelMarginPixels = 4

// pointHalfPixels is half the side of the square the window draws a
// point as.
const pointHalfPixels = 3.5

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
// its group, until its box is clear of the labels the group has and of
// the points the view shows: a distance's along the dimension's normal,
// an angle's along its bisector, a diameter's up the screen. A kind of
// constraint whose label place this does not know (a comment) is left
// as it is, and so is a label that stays on another after
// labelMaxSteps.
func (sk *Sketch) ClearNewLabel(c *core.Constraint) {
	if !c.HasLabel() || c.Type == core.ConstraintComment {
		return
	}
	v := sk.view()
	if v.Scale <= 0 {
		return
	}
	if _, ok := sk.labelAnchor(v, c, labelNominalText); !ok {
		return
	}
	others := sk.labelObstacles(v, c.Group)
	isClear := func(off geom.Vector) bool {
		moved := *c
		moved.Disp.Offset = c.Disp.Offset.Plus(off)
		at, _ := sk.labelAnchor(v, &moved, labelNominalText)
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

// labelObstacles is the boxes a new label of group g keeps clear of:
// the labels g has, and the points the view shows.
func (sk *Sketch) labelObstacles(v *View, g core.HGroup) []labelBox {
	var boxes []labelBox
	for i := range sk.Constraints {
		o := &sk.Constraints[i]
		if o.Group != g || !o.HasLabel() {
			continue
		}
		text := labelText(o)
		if a, ok := sk.labelAnchor(v, o, text); ok {
			boxes = append(boxes, sk.labelBoxAt(v, a, o, text))
		}
	}
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if !e.IsPoint() || !sk.EntityIsVisible(e) {
			continue
		}
		u, w := v.screenPixels(e.PointGetNum(sk))
		boxes = append(boxes, labelBox{u: u, v: w, hw: pointHalfPixels, hh: pointHalfPixels})
	}
	return boxes
}

// screenPixels is where p is seen on the screen, in pixels from its
// centre, as the labels are measured: without perspective.
func (v *View) screenPixels(p geom.Vector) (u, w float64) {
	q := p.Plus(v.Offset)
	return q.Dot(v.ProjRight) * v.Scale, q.Dot(v.ProjUp) * v.Scale
}

// labelBoxAt is the box of a label of c reading text, centred at p.
func (sk *Sketch) labelBoxAt(v *View, p geom.Vector, c *core.Constraint, text string) labelBox {
	th := sk.StyleTextHeight(c.StyleOrDefault(), v.Scale)
	w := res.BuiltinVectorFont().Width(th, text)
	u, h := v.screenPixels(p)
	return labelBox{
		u:  u,
		v:  h,
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
