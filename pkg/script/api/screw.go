package api

import (
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// ScrewMate is what Screw made: the parameters that drive it and its
// constraints.
type ScrewMate struct {
	// Turn names the parameter that turns the part, in degrees from
	// where it was; it starts at 0.
	Turn string
	// Lead names the parameter that is how far a whole turn moves the
	// part along the axis, in the document's unit, negative for a
	// left-hand screw.
	Lead        string
	Constraints []Constraint
}

// Screw puts two circles or arcs on one axis, as Concentric does, and
// couples the second part's turn about it to its travel along it, as a
// nut's on a screw: the part turns by the parameter Turn names and moves
// along the first circle's normal by Turn/360 times the parameter Lead
// names, from where it is, so that a motion study of Turn screws it in.
// The turn is held by the half-plane through the axis that a point of
// the second part lies in (an axis-angle constraint), which counts whole
// turns and either way; the travel by a distance along the axis. lead is
// in mm, as the parameter's first value. The parameters are new, named
// "turn" and "lead", or with a number after for a second screw. The
// mate is made in 3d, and the second part needs a point off the axis.
//
// When the circles carry screw threads, a screw's and a nut's of one
// size (made on them, or on their axis in the same part, as on the
// other rim of a nut's hole), the part is first moved along the axis,
// by less than half a pitch, to where its thread's ridges lie between
// the other's; a lead of 0 is the threads' own, negative for left-hand
// ones. With the threads' lead the two then fit at every turn.
func (m AssemblyMenu) Screw(a, b Entity, lead float64) (mate ScrewMate) {
	d := m.d
	if math.IsNaN(lead) || math.IsInf(lead, 0) {
		fail("a screw's lead is a distance that is not zero, not %g", lead)
	}
	if lead == 0 {
		d.read(func(sk *sketch.Sketch) {
			o, ok := sk.ThreadOnCircle(a.H)
			if !ok {
				o, ok = sk.ThreadOnCircle(b.H)
			}
			if !ok {
				fail("a screw's lead is a distance that is not zero, unless a thread on its circles gives it")
			}
			lead = o.Spec.Lead()
			if o.Spec.LeftHand {
				lead = -lead
			}
		})
	}
	d.read(func(sk *sketch.Sketch) {
		if !core.IsFreeIn3D(sk.ActiveWorkplane()) {
			fail("a screw mate is made in 3d; leave the workplane (Sketch.Anywhere3D)")
		}
		for _, e := range []Entity{a, b} {
			if t := e.get(sk).Type; t != core.EntityCircle && t != core.EntityArcOfCircle {
				fail("a screw mate is of two circles or arcs, and %s isn't one", e.describe(sk))
			}
		}
		mate.Turn, mate.Lead = freeName(sk, "turn"), freeName(sk, "lead")
	})
	mate.Constraints = m.Concentric(a, b)

	// Where the part is now, after Concentric moved it: the point of
	// it farthest from the axis, its angle about it and how far along.
	ca, cb, axis := a.Center(), b.Center(), a.Normal()
	var p Entity
	var angle, along, mmPer float64
	d.Edit.Regen()
	d.read(func(sk *sketch.Sketch) {
		n := axis.get(sk)
		nn := n.NormalN(sk)
		o := ca.get(sk).PointGetNum(sk)
		far := 0.0
		for i := range sk.Entities {
			e := &sk.Entities[i]
			if e.Group != b.get(sk).Group || !e.IsPoint() {
				continue
			}
			off := e.PointGetNum(sk).Minus(o)
			if r := off.Minus(nn.ScaledBy(off.Dot(nn))).Magnitude(); r > far+core.LengthEps {
				far, p = r, Entity{d, e.H}
			}
		}
		if far < core.LengthEps {
			fail("%s has no point off the axis to count its turn by", b.describe(sk))
		}
		off := p.get(sk).PointGetNum(sk).Minus(o)
		angle = math.Atan2(off.Dot(n.NormalV(sk)), off.Dot(n.NormalU(sk))) * 180 / math.Pi
		along = cb.get(sk).PointGetNum(sk).Minus(o).Dot(nn)
		mmPer = sk.Units.MmPer()
		move, err := sk.ThreadFitMove(a.H, b.H)
		switch {
		case errors.Is(err, sketch.ErrNoThreads):
		case err != nil:
			fail("%v", err)
		default:
			along += move
		}
	})
	if err := d.Params.Set(mate.Turn, "0"); err != nil {
		fail("%v", err)
	}
	if err := d.Params.Set(mate.Lead, strconv.FormatFloat(lead/mmPer, 'g', -1, 64)); err != nil {
		fail("%v", err)
	}
	turn := d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintAxisAngle
		c.Workplane = core.NoEntity // free in 3d
		c.PtA, c.EntityA, c.PtB = ca.H, axis.H, p.H
		c.ValA = angle
	})
	travel := d.constrain(func(sk *sketch.Sketch, c *core.Constraint) {
		c.Type = core.ConstraintProjPtDistance
		c.Workplane = core.NoEntity
		c.PtA, c.PtB, c.EntityA = ca.H, cb.H, axis.H
		c.ValA = along
	})
	// Signed as written: SetExpr would take a magnitude and keep the
	// side the distance is on, which turning the other way changes.
	d.change(func(sk *sketch.Sketch) {
		sk.Constraint(turn.H).Expr = plus(mate.Turn, angle)
		sk.Constraint(travel.H).Expr = plus(fmt.Sprintf("%s*%s/360", mate.Turn, mate.Lead), along/mmPer)
		sk.EvaluateParameters()
		for _, ee := range sk.ExprErrors {
			if ee.Constraint == turn.H || ee.Constraint == travel.H {
				fail("%v", ee)
			}
		}
	})
	mate.Constraints = append(mate.Constraints, turn, travel)
	return mate
}

// freeName is base, or base with the first number from 2 after it that
// names no parameter or dimension.
func freeName(sk *sketch.Sketch, base string) string {
	name := base
	for i := 2; sk.Parameter(name) != nil || sk.NamedConstraint(name) != nil; i++ {
		name = base + strconv.Itoa(i)
	}
	return name
}

// plus is the expression expr plus v, with v written exactly.
func plus(expr string, v float64) string {
	switch {
	case v == 0:
		return expr
	case v < 0:
		return expr + " - " + strconv.FormatFloat(-v, 'g', -1, 64)
	}
	return expr + " + " + strconv.FormatFloat(v, 'g', -1, 64)
}
