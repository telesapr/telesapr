package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// An extrusion's depth and a revolve's or helix's angle are solver
// parameters of the group, free to be dragged or held by a dimension. A
// held extent (Group.Extent) adds an equation of the group in place of
// that dimension, so that a parameter or an expression drives it as it
// drives a fillet's radius: the freedom goes, as a dimension would take
// it, and a drag can't move the end.
//
// Both are measured end to end: an extrusion from its bottom to its top
// along the normal of its workplane, a revolve from its first copy to
// its last. That is twice the translation parameters (one-sided, 0 to
// 2; two-sided, -1 to 1), and four times the angle parameter, whose
// quaternion turns by twice it at each of two steps.

// HasExtent reports whether the group's extent can be held: an
// extrusion of a sketch in a workplane, a revolve or a helix.
func (g *Group) HasExtent() bool {
	switch g.Type {
	case GroupExtrude:
		return !core.IsFreeIn3D(g.Predef.EntityB)
	case GroupRevolve, GroupHelix:
		return true
	}
	return false
}

// checkExtent says what is wrong with v as the group's held extent.
func checkExtent(g *Group, v float64) error {
	switch {
	case g.Type == GroupExtrude && !g.HasExtent():
		return errors.New("an extrusion of a sketch in 3d has no normal to hold a depth along; dimension a point of it instead")
	case !g.HasExtent():
		return errors.New("only an extrusion has a depth, and only a revolve or a helix an angle")
	case v == 0 && g.Type == GroupExtrude:
		return errors.New("a depth of zero makes no solid")
	case v == 0:
		return errors.New("an angle of zero makes no solid")
	case g.Type == GroupRevolve && math.Abs(v) > 360:
		return fmt.Errorf("a revolve turns by 360 degrees at most, not %g", v)
	}
	return nil
}

// MeasuredExtent is the group's extent as its parameters have it now,
// held or not: an extrusion's depth in millimetres, signed along the
// normal of its workplane, or a revolve's or helix's angle in degrees.
func (g *Group) MeasuredExtent(sk *Sketch) float64 {
	switch g.Type {
	case GroupExtrude:
		if !g.HasExtent() {
			return 0
		}
		n := sk.MustEntity(g.Predef.EntityB).NormalEntity(sk).NormalN(sk)
		var p geom.Vector
		for i, c := range []*float64{&p.X, &p.Y, &p.Z} {
			if q := sk.Param(g.H.Param(i)); q != nil {
				*c = q.Val
			}
		}
		return 2 * n.Dot(p)
	case GroupRevolve, GroupHelix:
		if p := sk.Param(g.H.Param(3)); p != nil {
			return 4 * p.Val * 180 / math.Pi
		}
	}
	return 0
}

// extentEquation appends the equation that holds a held extent, with
// index i of the group's equations.
func (g *Group) extentEquation(sk *Sketch, addEq func(*core.Expr, int), i int) {
	if g.Extent == 0 || !g.HasExtent() {
		return
	}
	ep := func(i int) *core.Expr { return core.NewParam(g.H.Param(i)) }
	ec := core.NewConstant
	if g.Type == GroupExtrude {
		n := sk.MustEntity(g.Predef.EntityB).NormalEntity(sk).NormalExprsN(sk)
		p := core.ExprVector{X: ep(0), Y: ep(1), Z: ep(2)}
		addEq(n.Dot(p).Times(ec(2)).Minus(ec(g.Extent)), i)
		return
	}
	addEq(ep(3).Times(ec(4)).Minus(ec(g.Extent*math.Pi/180)), i)
}

// SetGroupExtentHeld holds a group's extent where its parameters have it
// now, or, with held false, frees it again to be dragged or dimensioned.
// Its expression goes either way.
func (sk *Sketch) SetGroupExtentHeld(hg core.HGroup, held bool) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	v := 0.0
	if held {
		v = g.MeasuredExtent(sk)
		if err := checkExtent(g, v); err != nil {
			return err
		}
	}
	g.Extent, g.ExprExtent = v, ""
	sk.MarkGroupDirty(hg, false)
	return nil
}

// SetGroupExtent holds a group's extent at v, an extrusion's depth in
// millimetres or a revolve's or helix's angle in degrees, as a plain
// number; SetGroupText with GroupExtent takes an expression.
func (sk *Sketch) SetGroupExtent(hg core.HGroup, v float64) error {
	g := sk.Group(hg)
	if g == nil {
		return fmt.Errorf("no group %08x", hg.V)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return errors.New("not a number")
	}
	if err := checkExtent(g, v); err != nil {
		return err
	}
	g.Extent, g.ExprExtent = v, ""
	sk.MarkGroupDirty(hg, false)
	return nil
}
