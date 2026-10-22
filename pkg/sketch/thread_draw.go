package sketch

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// threadGroups are the four groups of a thread, in their order.
type threadGroups struct {
	blank, lathe, profile, helix core.HGroup
}

// threadDrawer draws a thread's sketches and dimensions into the
// active group. Making a thread draws them into new groups; editing one
// draws them again into its groups, taking the requests a sketch
// already has before adding any, so that what the later groups refer
// to keeps its handle where the thread keeps its shape.
type threadDrawer struct {
	sk     *Sketch
	o      ThreadOptions
	l      threadLayout
	circle *core.Entity
	center core.HEntity
	origin geom.Vector
	q      geom.Quaternion
	// along is 1 or -1, along the circle's normal or against it; hand
	// is 1 for a right-hand thread.
	along, hand float64
	turns       float64
	// u and v are the active sketch's directions: a radius, and the
	// axis upwards.
	u, v geom.Vector
	// start is the angle of the ridge sketch's plane.
	start float64
	// reuse are requests of the groups being drawn again, by group,
	// in the order they were drawn.
	reuse map[core.HGroup][]core.HRequest
	// first is the first ridge's request whose point the others and
	// the helix's length are measured from.
	first     core.HRequest
	firstMark int
}

// newThreadDrawer lays out the thread o on the circle he, which
// CheckThread has passed.
func (sk *Sketch) newThreadDrawer(he core.HEntity, o ThreadOptions, reuse map[core.HGroup][]core.HRequest) *threadDrawer {
	circle := sk.MustEntity(he)
	l, err := o.layout(circle.CircleGetRadiusNum(sk))
	if err != nil {
		panic("sketch: thread laid out after its check failed: " + err.Error())
	}
	d := &threadDrawer{sk: sk, o: o, l: l, circle: circle, center: circle.Point[0], reuse: reuse,
		along: 1, hand: 1}
	d.origin = sk.MustEntity(d.center).PointGetNum(sk)
	d.q = sk.MustEntity(circle.Normal).NormalGetNum(sk)
	if o.Reverse {
		d.along = -1
	}
	if o.Spec.LeftHand {
		d.hand = -1
	}
	d.turns = l.ridgeSweep / o.Spec.Lead()
	d.v = d.q.RotationN().ScaledBy(d.along)

	// A revolved surface is made in quarters, from the plane of its
	// sketch on, and a ridge has a flat end in that plane and another
	// where its sweep stops. Those ends would meet the turned surface
	// along the joins of its quarters, where a Boolean is least sure of
	// itself: so the ridges' plane is turned to put both ends as far
	// from the joins as can be, which is an eighth of a turn or more.
	quarter := math.Pi / 2
	past := math.Mod(d.turns*2*math.Pi, quarter)
	start := (quarter - past) / 2
	if past > quarter/2 {
		start = (2*quarter - past) / 2
	}
	d.start = start * d.along * d.hand
	return d
}

// name is the name of the thread's group that does what.
func (d *threadDrawer) name(what string) string { return d.o.Spec.String() + "-" + what }

func (d *threadDrawer) latheName() string {
	if d.o.Internal {
		return "bore"
	}
	return "core"
}

// plane is the sketch plane at the given angle about the circle's
// normal: a radius to the right, the axis upwards.
func (d *threadDrawer) plane(angle float64) geom.Quaternion {
	return geom.QuaternionFromBasis(d.q.RotationU().RotatedAroundAxis(d.q.RotationN(), angle), d.v)
}

// placeSketch puts a sketch group of the thread in its plane, and
// makes that plane the one drawn in.
func (d *threadDrawer) placeSketch(g *Group, what string, angle float64) {
	g.Name = d.name(what)
	g.Predef.Q = d.plane(angle)
	g.Predef.Origin = d.center
	d.u = g.Predef.Q.RotationU()
}

// nameSolid names a lathe or a helix group of the thread.
func (d *threadDrawer) nameSolid(g *Group, what string) { g.Name = d.name(what) }

// at is the point at radius r and height z in the active sketch.
func (d *threadDrawer) at(r, z float64) geom.Vector {
	return d.origin.Plus(d.u.ScaledBy(r)).Plus(d.v.ScaledBy(z))
}

// request adds a request of the type to the active group, or takes
// the first of that type the group had.
func (d *threadDrawer) request(typ RequestType) core.HRequest {
	sk := d.sk
	hg := sk.ActiveGroup()
	old := d.reuse[hg]
	for i, hr := range old {
		if r := sk.Request(hr); r != nil && r.Type == typ {
			d.reuse[hg] = slices.Delete(old, i, i+1)
			return hr
		}
	}
	return sk.AddRequestOfType(typ, false)
}

// line draws a line segment of the active sketch.
func (d *threadDrawer) line(r0, z0, r1, z1 float64) core.HRequest {
	sk := d.sk
	hr := d.request(RequestLineSegment)
	sk.MustEntity(hr.Entity(1)).PointForceTo(sk, d.at(r0, z0))
	sk.MustEntity(hr.Entity(2)).PointForceTo(sk, d.at(r1, z1))
	return hr
}

// arc draws an arc of the active sketch about (rc, zc), anticlockwise
// in the sketch's plane from (r0, z0) to (r1, z1).
func (d *threadDrawer) arc(rc, zc, r0, z0, r1, z1 float64) core.HRequest {
	sk := d.sk
	hr := d.request(RequestArcOfCircle)
	sk.MustEntity(hr.Entity(1)).PointForceTo(sk, d.at(rc, zc))
	sk.MustEntity(hr.Entity(2)).PointForceTo(sk, d.at(r0, z0))
	sk.MustEntity(hr.Entity(3)).PointForceTo(sk, d.at(r1, z1))
	return hr
}

func (d *threadDrawer) coincident(a, b core.HEntity) { d.sk.ConstrainCoincident(a, b) }

func (d *threadDrawer) closeLoop(lines []core.HRequest) {
	for i := range lines {
		d.coincident(lines[i].Entity(2), lines[(i+1)%len(lines)].Entity(1))
	}
}

func (d *threadDrawer) constrain(typ core.ConstraintType, ptA, ptB, entityA, entityB core.HEntity, other bool) {
	d.sk.Constrain(typ, ptA, ptB, entityA, entityB, other, false)
}

// dimension adds a constraint with a value, which the drawing already
// has: the sign and the supplement are whatever make it so.
func (d *threadDrawer) dimension(typ core.ConstraintType, val float64, ptA, ptB, entityA, entityB core.HEntity) *core.Constraint {
	sk := d.sk
	c := core.Constraint{
		Group:     sk.ActiveGroup(),
		Workplane: sk.ActiveWorkplane(),
		Type:      typ}
	c.PtA, c.PtB = ptA, ptB
	c.EntityA, c.EntityB = entityA, entityB
	c.ModifyToSatisfy(sk)
	if typ == core.ConstraintAngle && math.Abs(c.ValA-val) > 1 {
		c.Other = true
		c.ModifyToSatisfy(sk)
	}
	if math.Abs(math.Abs(c.ValA)-val) > 1e-6 {
		panic(fmt.Sprintf("sketch: thread dimension is %v, drawn as %v", val, c.ValA))
	}
	c.ValA = math.Copysign(val, c.ValA)
	return sk.Constraint(sk.AddConstraintRemember(&c, false))
}

// lengthDimension adds a dimension that is the thread's length plus
// plus millimetres, which carries the length's expression when it has
// one.
func (d *threadDrawer) lengthDimension(typ core.ConstraintType, plus float64, ptA, ptB, entityA, entityB core.HEntity) {
	c := d.dimension(typ, d.o.Length+plus, ptA, ptB, entityA, entityB)
	if d.o.LengthExpr == "" {
		return
	}
	expr := d.o.LengthExpr
	if k := plus / d.sk.Units.MmPer(); k != 0 {
		sign := " + "
		if k < 0 {
			sign = " - "
		}
		expr = "(" + expr + ")" + sign + strconv.FormatFloat(math.Abs(k), 'g', -1, 64)
	}
	if c.ValA < 0 {
		expr = "-(" + expr + ")"
	}
	c.Expr = expr
}

// drawBlank draws, in the active sketch, what the turning or the
// boring takes away: a rectangle, revolved.
func (d *threadDrawer) drawBlank() {
	l, none := d.l, core.NoEntity
	blank := []core.HRequest{
		d.line(l.blankIn, l.blankBottom, l.blankIn, l.blankTop),
		d.line(l.blankIn, l.blankTop, l.blankOut, l.blankTop),
		d.line(l.blankOut, l.blankTop, l.blankOut, l.blankBottom),
		d.line(l.blankOut, l.blankBottom, l.blankIn, l.blankBottom),
	}
	d.closeLoop(blank)
	d.constrain(core.ConstraintVertical, none, none, blank[0].Entity(0), none, false)
	d.constrain(core.ConstraintVertical, none, none, blank[2].Entity(0), none, false)
	d.constrain(core.ConstraintHorizontal, none, none, blank[1].Entity(0), none, false)
	d.constrain(core.ConstraintHorizontal, none, none, blank[3].Entity(0), none, false)
	if l.blankIn == 0 {
		d.constrain(core.ConstraintPtOnLine, d.center, none, blank[0].Entity(0), none, false)
	} else {
		d.dimension(core.ConstraintPtLineDistance, l.blankIn, d.center, none, blank[0].Entity(0), none)
	}
	d.dimension(core.ConstraintPtLineDistance, l.blankOut, d.center, none, blank[2].Entity(0), none)
	d.lengthDimension(core.ConstraintPtLineDistance, l.blankTop-d.o.Length, d.center, none, blank[1].Entity(0), none)
	d.dimension(core.ConstraintPtLineDistance, -l.blankBottom, d.center, none, blank[3].Entity(0), none)
}

// drawProfile draws, in the active sketch, the ridges, one for each
// start, a pitch apart.
func (d *threadDrawer) drawProfile() {
	for i := range d.o.Spec.NumStarts() {
		z := d.l.ridgeBottom + float64(i)*d.o.Spec.Pitch + d.l.baseW/2
		if d.l.profile.CrestRoundRadius > 0 {
			d.drawRoundRidge(i, z)
		} else {
			d.drawFlatRidge(i, z)
		}
	}
}

// drawFlatRidge draws the ridge i, centred at height z, with a flat
// crest: a trapezoid.
func (d *threadDrawer) drawFlatRidge(i int, z float64) {
	p, l, none := d.l.profile, d.l, core.NoEntity
	base, crest := l.baseW/2, p.CrestWidth/2
	ridge := []core.HRequest{
		d.line(l.baseR, z-base, l.baseR, z+base),
		d.line(l.baseR, z+base, p.CrestRadius, z+crest),
		d.line(p.CrestRadius, z+crest, p.CrestRadius, z-crest),
		d.line(p.CrestRadius, z-crest, l.baseR, z-base),
	}
	d.closeLoop(ridge)
	d.constrain(core.ConstraintVertical, none, none, ridge[0].Entity(0), none, false)
	d.constrain(core.ConstraintVertical, none, none, ridge[2].Entity(0), none, false)
	d.dimension(core.ConstraintPtLineDistance, l.baseR, d.center, none, ridge[0].Entity(0), none)
	d.dimension(core.ConstraintPtLineDistance, p.CrestRadius, d.center, none, ridge[2].Entity(0), none)
	d.dimension(core.ConstraintPtPtDistance, p.CrestWidth, ridge[2].Entity(1), ridge[2].Entity(2), none, none)
	d.dimension(core.ConstraintAngle, p.FlankAngle, none, none, ridge[1].Entity(0), ridge[3].Entity(0))
	d.constrain(core.ConstraintEqualLengthLines, none, none, ridge[1].Entity(0), ridge[3].Entity(0), false)
	d.placeRidge(i, ridge[2], 2, z-crest)
}

// drawRoundRidge draws the ridge i, centred at height z, with a
// rounded crest: the flanks run into an arc tangent to both.
func (d *threadDrawer) drawRoundRidge(i int, z float64) {
	p, l, none := d.l.profile, d.l, core.NoEntity
	base, foot := l.baseW/2, p.CrestWidth/2
	toRoot := math.Copysign(1, p.RootRadius-p.CrestRadius)
	rc := p.CrestRadius + toRoot*p.CrestRoundRadius
	b := d.line(l.baseR, z-base, l.baseR, z+base)
	up := d.line(l.baseR, z+base, p.RoundFootRadius, z+foot)
	down := d.line(p.RoundFootRadius, z-foot, l.baseR, z-base)
	// The arc bulges away from the axis on a screw, towards it in a
	// nut, and runs anticlockwise.
	var crest core.HRequest
	atUp := true // whether the arc ends at the upper flank
	if toRoot < 0 {
		crest = d.arc(rc, z, p.RoundFootRadius, z-foot, p.RoundFootRadius, z+foot)
	} else {
		crest = d.arc(rc, z, p.RoundFootRadius, z+foot, p.RoundFootRadius, z-foot)
		atUp = false
	}
	upEnd, downEnd := crest.Entity(3), crest.Entity(2)
	if !atUp {
		upEnd, downEnd = downEnd, upEnd
	}
	d.coincident(b.Entity(2), up.Entity(1))
	d.coincident(up.Entity(2), upEnd)
	d.coincident(downEnd, down.Entity(1))
	d.coincident(down.Entity(2), b.Entity(1))
	d.constrain(core.ConstraintVertical, none, none, b.Entity(0), none, false)
	d.dimension(core.ConstraintPtLineDistance, l.baseR, d.center, none, b.Entity(0), none)
	d.dimension(core.ConstraintAngle, p.FlankAngle, none, none, up.Entity(0), down.Entity(0))
	d.constrain(core.ConstraintEqualLengthLines, none, none, up.Entity(0), down.Entity(0), false)
	d.constrain(core.ConstraintArcLineTangent, none, none, crest.Entity(0), up.Entity(0), atUp)
	d.constrain(core.ConstraintArcLineTangent, none, none, crest.Entity(0), down.Entity(0), !atUp)
	d.dimension(core.ConstraintDiameter, 2*p.CrestRoundRadius, none, none, crest.Entity(0), none)
	d.dimension(core.ConstraintPtLineDistance, math.Abs(rc-l.baseR), crest.Entity(1), none, b.Entity(0), none)
	d.placeRidge(i, crest, 1, z)
}

// placeRidge holds ridge i along the axis by the point mark of hr, at
// height z: the first ridge from the circle's plane, the others from
// the first.
func (d *threadDrawer) placeRidge(i int, hr core.HRequest, mark int, z float64) {
	none := core.NoEntity
	if i == 0 {
		d.first, d.firstMark = hr, mark
		d.dimension(core.ConstraintProjPtDistance, z, d.center, hr.Entity(mark), d.circle.Normal, none)
		return
	}
	d.dimension(core.ConstraintProjPtDistance, float64(i)*d.o.Spec.Pitch,
		d.first.Entity(d.firstMark), hr.Entity(mark), d.circle.Normal, none)
}

// setHelix gives the thread's helix group hg its pitch and its params
// their values. The helix turns by four times its angle param while it
// moves by twice its distance param; a right-hand thread turns the way
// it moves, about the axis.
func (d *threadDrawer) setHelix(g *Group, hg core.HGroup) {
	g.ValA, g.ValB, g.ExprB = 2, d.hand*d.o.Spec.Lead(), ""
	d.sk.MustParam(hg.Param(3)).Val = d.along * d.hand * d.turns * 2 * math.Pi / 4
	d.sk.MustParam(hg.Param(7)).Val = d.along * d.l.ridgeSweep / 2
	g.Thread = GroupThread{Circle: d.circle.H, ThreadOptions: d.o}
}

// dimensionHelix holds the helix's length, without which it could be
// dragged longer. The helix group must be active, its entities made.
func (d *threadDrawer) dimensionHelix(hel *Group) {
	mark := d.first.Entity(d.firstMark)
	d.lengthDimension(core.ConstraintProjPtDistance, d.l.ridgeSweep-d.o.Length,
		hel.RemapEntity(mark, RemapLatheStart), hel.RemapEntity(mark, RemapLatheEnd),
		d.circle.Normal, core.NoEntity)
}
