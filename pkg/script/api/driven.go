package api

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// DrivenMate is what HingeDriven or ConcentricDriven made: the
// parameter that drives the joint, and the mate's constraints, the one
// the parameter drives last.
type DrivenMate struct {
	// Param names the parameter that drives the joint: a hinge's angle,
	// in degrees from where the part was, or a concentric mate's
	// travel along the axis, in the document's unit from where it was.
	// It starts at 0.
	Param       string
	Constraints []Constraint
	// at is the driven dimension's value where the part was, in its own
	// units (degrees, mm): Param's 0.
	at float64
}

// HingeDriven puts line b on line a as Hinge does, and drives b's turn
// about a by a new parameter, "angle" (or with a number after for a
// second), in degrees from where the part is, so that a motion study of
// it turns the part, and the parts of a mechanism joined to it with
// it. Of two circles or arcs, a pin in its hole, it puts them on one
// axis as Concentric does and drives the turn about the first one's
// axis; the second part may still slide along the axis, which a Flush
// of its face on the first part's holds. The turn is held by the
// half-plane through the axis in which the point of b's part farthest
// from it lies (an axis-angle constraint), which counts whole turns and
// either way. A driven joint has no freedom left: dragging the part
// moves nothing, the parameter does. The mate is made in 3d.
func (m AssemblyMenu) HingeDriven(a, b Entity) DrivenMate {
	d := m.d
	var circles bool
	d.read(func(sk *sketch.Sketch) {
		if !core.IsFreeIn3D(sk.ActiveWorkplane()) {
			fail("a driven mate is made in 3d; leave the workplane (Sketch.Anywhere3D)")
		}
		ta, tb := a.get(sk).Type, b.get(sk).Type
		round := func(t core.EntityType) bool { return t == core.EntityCircle || t == core.EntityArcOfCircle }
		switch {
		case ta == core.EntityLineSegment && tb == core.EntityLineSegment:
		case round(ta) && round(tb):
			circles = true
		default:
			fail("a driven hinge is of two line segments, or of two circles or arcs, and %s and %s aren't",
				a.describe(sk), b.describe(sk))
		}
	})
	var mate DrivenMate
	// The axis: a point on it, and the entity it turns about.
	var pa, axis Entity
	if circles {
		mate.Constraints = m.Concentric(a, b)
		pa, axis = a.Center(), a.Normal()
	} else {
		mate.Constraints = m.Hinge(a, b)
		pa, axis = a.Start(), a
	}
	d.Edit.Regen()
	var p Entity
	d.read(func(sk *sketch.Sketch) {
		mate.Param = freeName(sk, "angle")
		o := pa.get(sk).PointGetNum(sk)
		var dir Vec
		if circles {
			dir = axis.get(sk).NormalN(sk)
		} else {
			la := a.get(sk)
			dir = sk.MustEntity(la.Point[1]).PointGetNum(sk).Minus(o).WithMagnitude(1)
		}
		p = farthestOf(d, sk, b.get(sk).Group, func(q Vec) float64 {
			off := q.Minus(o)
			return off.Minus(dir.ScaledBy(off.Dot(dir))).Magnitude()
		})
	})
	turn := d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintAxisAngle
		c.Workplane = core.NoEntity // free in 3d
		c.PtA, c.EntityA, c.PtB = pa.H, axis.H, p.H
		c.ValA = c.AxisAngleNum(sk)
		mate.at = c.ValA
	})
	d.drive(&mate, turn, 1)
	return mate
}

// ConcentricDriven puts two circles or arcs on one axis as Concentric
// does, and drives the second's travel along the first's normal by a
// new parameter, "travel" (or with a number after), in the document's
// unit from where it is, so that a motion study of it slides the part.
// The part may still turn about the axis; hold that by another mate, as
// a flat face on a flat face. The mate is made in 3d.
func (m AssemblyMenu) ConcentricDriven(a, b Entity) DrivenMate {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		if !core.IsFreeIn3D(sk.ActiveWorkplane()) {
			fail("a driven mate is made in 3d; leave the workplane (Sketch.Anywhere3D)")
		}
	})
	mate := DrivenMate{Constraints: m.Concentric(a, b)}
	d.read(func(sk *sketch.Sketch) { mate.Param = freeName(sk, "travel") })
	travel := d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintProjPtDistance
		c.Workplane = core.NoEntity // free in 3d
		c.PtA, c.PtB, c.EntityA = a.Center().H, b.Center().H, a.Normal().H
		n := sk.MustEntity(c.EntityA).NormalN(sk)
		c.ValA = sk.MustEntity(c.PtB).PointGetNum(sk).Minus(sk.MustEntity(c.PtA).PointGetNum(sk)).Dot(n)
		mate.at = c.ValA
	})
	var mmPer float64
	d.read(func(sk *sketch.Sketch) { mmPer = sk.Units.MmPer() })
	d.drive(&mate, travel, mmPer)
	return mate
}

// drive gives mate's driving dimension c the expression of its new
// parameter plus where it is, the parameter at 0, scale mm to its unit
// (1 for an angle), and appends c to the mate's constraints.
func (d *Doc) drive(mate *DrivenMate, c Constraint, scale float64) {
	if err := d.Params.Set(mate.Param, "0"); err != nil {
		fail("%v", err)
	}
	// Signed as written: SetExpr would take a magnitude and keep the
	// side the dimension is on, which driving it the other way changes.
	d.change(func(sk *sketch.Sketch) {
		sk.Constraint(c.H).Expr = plus(mate.Param, mate.at/scale)
		sk.EvaluateParameters()
		for _, ee := range sk.ExprErrors {
			if ee.Constraint == c.H {
				fail("%v", ee)
			}
		}
	})
	mate.Constraints = append(mate.Constraints, c)
}

// farthestOf is the point of group hg that dist puts farthest; the
// mate fails when none is off by more than a length's tolerance.
func farthestOf(d *Doc, sk *sketch.Sketch, hg core.HGroup, dist func(Vec) float64) Entity {
	var p Entity
	far := 0.0
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if e.Group != hg || !e.IsPoint() {
			continue
		}
		if r := dist(e.PointGetNum(sk)); r > far+core.LengthEps {
			far, p = r, Entity{d, e.H}
		}
	}
	if far < core.LengthEps {
		fail("%s has no point off the axis to count its turn by", sk.GroupName(hg))
	}
	return p
}

// SetRange keeps the driven joint between lo and hi of its parameter,
// as two bounds beside the dimension it drives (a minimum and a
// maximum): a value of the parameter past them fails to solve, and a
// motion study marks that step unsolved. lo must be less than hi. It
// returns the two bounds.
func (dm DrivenMate) SetRange(lo, hi float64) []Constraint {
	if len(dm.Constraints) == 0 {
		fail("a driven mate made by HingeDriven or ConcentricDriven sets a range")
	}
	if math.IsNaN(lo) || math.IsNaN(hi) || lo >= hi {
		fail("a range runs from a least to a greatest value, not %g to %g", lo, hi)
	}
	drive := dm.Constraints[len(dm.Constraints)-1]
	d := drive.d
	var out []Constraint
	d.change(func(sk *sketch.Sketch) {
		scale := 1.0
		if sk.Constraint(drive.H).Type != core.ConstraintAxisAngle {
			scale = sk.Units.MmPer()
		}
		for _, b := range []struct {
			l core.Limit
			v float64
		}{{core.LimitMin, lo}, {core.LimitMax, hi}} {
			h, err := sk.AddBoundOf(drive.H, b.l, dm.at+b.v*scale)
			if err != nil {
				fail("%v", err)
			}
			out = append(out, Constraint{d, h})
		}
	})
	return out
}

// String is the mate's parameter and its constraints, for a script's
// print.
func (dm DrivenMate) String() string {
	return fmt.Sprintf("driven by %s, %d constraints", dm.Param, len(dm.Constraints))
}
