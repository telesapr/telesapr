package sketch

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/thread"
)

// threadGroups are the groups of a thread, in their order: four, then
// a ridge's sketch and helix for each level start past the first, then
// with a chamfer its sketch and its lathe. A cosmetic thread has only
// its sketch, blank, and the lathe that turns or bores the part when it
// must; its other handles are zero.
type threadGroups struct {
	blank, lathe, profile, helix core.HGroup
	// starts are the sketch and the helix of each start past the first.
	starts [][2]core.HGroup
	// chamfer is the sketch and the lathe that cut the chamfers, none
	// for a thread without.
	chamfer [2]core.HGroup
}

// chamfered tells whether the thread has a chamfer's groups.
func (tg threadGroups) chamfered() bool { return tg.chamfer[0] != (core.HGroup{}) }

// all are the thread's groups in their order.
func (tg threadGroups) all() []core.HGroup {
	var out []core.HGroup
	for _, h := range []core.HGroup{tg.blank, tg.lathe, tg.profile, tg.helix} {
		if h != (core.HGroup{}) {
			out = append(out, h)
		}
	}
	for _, st := range tg.starts {
		out = append(out, st[0], st[1])
	}
	if tg.chamfered() {
		out = append(out, tg.chamfer[0], tg.chamfer[1])
	}
	return out
}

// extra are the groups after the first helix.
func (tg threadGroups) extra() []core.HGroup {
	var out []core.HGroup
	for _, st := range tg.starts {
		out = append(out, st[0], st[1])
	}
	return out
}

// solids are the thread's groups that make solids.
func (tg threadGroups) solids() []core.HGroup {
	var out []core.HGroup
	for _, h := range []core.HGroup{tg.lathe, tg.helix} {
		if h != (core.HGroup{}) {
			out = append(out, h)
		}
	}
	for _, st := range tg.starts {
		out = append(out, st[1])
	}
	if tg.chamfered() {
		out = append(out, tg.chamfer[1])
	}
	return out
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
	// shift moves the ridges along the thread, by its phase.
	shift float64
	// reuse are requests of the groups being drawn again, by group,
	// in the order they were drawn.
	reuse map[core.HGroup][]core.HRequest
	// first is, by ridge sketch, the point of its first ridge that the
	// others and its helix's length are measured from.
	first map[core.HGroup]core.HEntity
	// lengthDims are the dimensions drawn that carry the thread's
	// length, kept on a full-length thread's record (keepLengthDims).
	lengthDims []core.HConstraint
}

// newThreadDrawer lays out the thread o on the circle he, which
// CheckThread has passed.
//
// try picks among the planes for the ridges that keep their edges off
// the joins, the best first (startTries), and then, for a chamfered
// thread, among the ways of making the chamfer (chamferTries): all the
// planes for each.
func (sk *Sketch) newThreadDrawer(he core.HEntity, o ThreadOptions, try int) *threadDrawer {
	circle := sk.MustEntity(he)
	l, err := o.layoutTry(circle.CircleGetRadiusNum(sk), try/startTries)
	if err != nil {
		panic("sketch: thread laid out after its check failed: " + err.Error())
	}
	// A class's clearance is kept as the thread's own too, which a
	// reader of the file from before classes makes it with; a cosmetic
	// thread's class sets none (FitClearance).
	if !o.Cosmetic {
		o.Clearance, _ = o.FitClearance()
	}
	d := &threadDrawer{sk: sk, o: o, l: l, circle: circle, center: circle.Point[0],
		along: 1, hand: 1, first: map[core.HGroup]core.HEntity{}}
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
	if d.retries() {
		start = startsClearOfJoins(d.crossings())[try%startTries]
	}
	d.start = start * d.along * d.hand
	d.shift = o.phaseShift()
	return d
}

// retries tells whether another plane for the ridges may make a
// solid that failed: only one with level starts, a run-out or a
// chamfer picks among several.
func (d *threadDrawer) retries() bool { return d.o.levelStarts() || d.l.runOut || d.l.chamfer }

// tries is how many ways of drawing the thread there are to try.
func (d *threadDrawer) tries() int {
	if d.l.chamfer {
		return startTries * len(chamferTries)
	}
	return startTries
}

// crossings are the angles, from the ridge sketch's plane and the way
// the thread turns, where a ridge's edge would cross a join of the
// turned surface's quarters if that plane lay on one: each start's ends
// and, with a run-out, where each flank crosses the circle at which the
// root turns into the run-out's cone.
func (d *threadDrawer) crossings() []float64 {
	o, l := d.o, d.l
	sweep := d.turns * 2 * math.Pi
	n := o.Spec.NumStarts()
	var out []float64
	for i := range n {
		// The start's turn about the axis and its ridge's middle along
		// it: level starts are turned, stacked ones raised a pitch.
		turn, z := 0.0, l.ridgeMiddle()+o.phaseShift()
		if o.levelStarts() {
			turn = float64(i) * 2 * math.Pi / float64(n)
		} else {
			z += float64(i) * o.Spec.Pitch
		}
		if i == 0 || o.levelStarts() {
			out = append(out, turn, turn+sweep)
		}
		if !l.runOut {
			continue
		}
		lo, hi := l.flanksAt(z, l.profile.RootRadius)
		for _, flank := range []float64{lo, hi} {
			if at := (l.runFrom - flank) / o.Spec.Lead() * 2 * math.Pi; at > 0 && at < sweep {
				out = append(out, turn+at)
			}
		}
	}
	return out
}

// startTries is how many planes for the ridges a thread with level
// starts or a run-out tries: where the ridges' edges cross the
// turned surface's cone, a Boolean that fails at one plane may not at
// another.
const startTries = 4

// startsClearOfJoins are startTries angles in the first quarter turn
// that, added to each of the angles, keep the nearest of them far from
// a multiple of a quarter turn: the farthest first, then the farthest
// of those an eighth of a quarter from the ones before.
func startsClearOfJoins(angles []float64) []float64 {
	const steps = 900
	quarter := math.Pi / 2
	margin := make([]float64, steps)
	for k := range steps {
		s := quarter * float64(k) / steps
		margin[k] = quarter
		for _, a := range angles {
			margin[k] = min(margin[k], math.Abs(math.Remainder(s+a, quarter)))
		}
	}
	var picked []float64
	for range startTries {
		best, bestAt := -1.0, 0.0
		for k := range steps {
			s := quarter * float64(k) / steps
			if slices.ContainsFunc(picked, func(p float64) bool {
				return math.Abs(math.Remainder(s-p, quarter)) < quarter/8
			}) {
				continue
			}
			if margin[k] > best {
				best, bestAt = margin[k], s
			}
		}
		picked = append(picked, bestAt)
	}
	return picked
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
	// A height past the circle's plane, as a chamfered thread's first
	// ridge has, is a distance the other way.
	val = math.Abs(val)
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
	if d.o.FullLength {
		d.lengthDims = append(d.lengthDims, c.H)
	}
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
// boring takes away: a rectangle, revolved; with a run-out, its side at
// the root tapers to the crest at the thread's end.
func (d *threadDrawer) drawBlank() {
	if d.l.chamfer {
		d.drawChamferedBlank()
		return
	}
	if d.l.runOut {
		d.drawTaperedBlank()
		return
	}
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

// drawTaperedBlank is drawBlank with a run-out: five sides, the root's
// side ending at the run-out's start and a cone from there to about the
// crest at the thread's end, where the part's shoulder is.
func (d *threadDrawer) drawTaperedBlank() {
	l, none := d.l, core.NoEntity
	var blank []core.HRequest
	// rootSide is the side at the root, and top the side along the
	// thread's end, whose lengths hold the cone.
	var rootSide, top, inSide, outSide core.HRequest
	if !d.o.Internal {
		blank = []core.HRequest{
			d.line(l.blankIn, l.blankBottom, l.blankIn, l.runFrom),
			d.line(l.blankIn, l.runFrom, l.runTo, l.blankTop),
			d.line(l.runTo, l.blankTop, l.blankOut, l.blankTop),
			d.line(l.blankOut, l.blankTop, l.blankOut, l.blankBottom),
			d.line(l.blankOut, l.blankBottom, l.blankIn, l.blankBottom),
		}
		rootSide, top, inSide, outSide = blank[0], blank[2], blank[0], blank[3]
		d.dimension(core.ConstraintPtPtDistance, l.blankOut-l.runTo, top.Entity(1), top.Entity(2), none, none)
	} else {
		blank = []core.HRequest{
			d.line(l.blankIn, l.blankBottom, l.blankIn, l.blankTop),
			d.line(l.blankIn, l.blankTop, l.runTo, l.blankTop),
			d.line(l.runTo, l.blankTop, l.blankOut, l.runFrom),
			d.line(l.blankOut, l.runFrom, l.blankOut, l.blankBottom),
			d.line(l.blankOut, l.blankBottom, l.blankIn, l.blankBottom),
		}
		rootSide, top, inSide, outSide = blank[3], blank[1], blank[0], blank[3]
		d.dimension(core.ConstraintPtPtDistance, l.runTo-l.blankIn, top.Entity(1), top.Entity(2), none, none)
	}
	d.closeLoop(blank)
	d.constrain(core.ConstraintVertical, none, none, inSide.Entity(0), none, false)
	d.constrain(core.ConstraintVertical, none, none, outSide.Entity(0), none, false)
	d.constrain(core.ConstraintHorizontal, none, none, top.Entity(0), none, false)
	d.constrain(core.ConstraintHorizontal, none, none, blank[4].Entity(0), none, false)
	if l.blankIn == 0 {
		d.constrain(core.ConstraintPtOnLine, d.center, none, inSide.Entity(0), none, false)
	} else {
		d.dimension(core.ConstraintPtLineDistance, l.blankIn, d.center, none, inSide.Entity(0), none)
	}
	d.dimension(core.ConstraintPtLineDistance, l.blankOut, d.center, none, outSide.Entity(0), none)
	d.lengthDimension(core.ConstraintPtLineDistance, l.blankTop-d.o.Length, d.center, none, top.Entity(0), none)
	d.dimension(core.ConstraintPtLineDistance, -l.blankBottom, d.center, none, blank[4].Entity(0), none)
	d.lengthDimension(core.ConstraintPtPtDistance, l.runFrom-l.blankBottom-d.o.Length,
		rootSide.Entity(1), rootSide.Entity(2), none, none)
}

// drawProfile draws, in the active sketch, the ridges, one for each
// start, a pitch apart; with level starts, each sketch has one.
func (d *threadDrawer) drawProfile() {
	n := d.o.Spec.NumStarts()
	if d.o.levelStarts() {
		n = 1
	}
	for i := range n {
		z := d.l.ridgeMiddle() + float64(i)*d.o.Spec.Pitch + d.shift
		switch {
		case d.l.profile.CrestRoundRadius > 0:
			d.drawRoundRidge(i, z)
		case d.l.profile.RootRoundRadius > 0:
			d.drawRoundRootRidge(i, z)
		default:
			d.drawFlatRidge(i, z)
		}
	}
}

// drawFlatRidge draws the ridge i, its foot centred at height z, with a
// flat crest: a trapezoid, whose crest lies off its foot's middle on a
// buttress thread.
func (d *threadDrawer) drawFlatRidge(i int, z float64) {
	p, l, none := d.l.profile, d.l, core.NoEntity
	base, crest := l.baseW/2, p.CrestWidth/2
	zc := z - l.footShift
	ridge := []core.HRequest{
		d.line(l.baseR, z-base, l.baseR, z+base),
		d.line(l.baseR, z+base, p.CrestRadius, zc+crest),
		d.line(p.CrestRadius, zc+crest, p.CrestRadius, zc-crest),
		d.line(p.CrestRadius, zc-crest, l.baseR, z-base),
	}
	d.closeLoop(ridge)
	d.constrain(core.ConstraintVertical, none, none, ridge[0].Entity(0), none, false)
	d.constrain(core.ConstraintVertical, none, none, ridge[2].Entity(0), none, false)
	d.dimension(core.ConstraintPtLineDistance, l.baseR, d.center, none, ridge[0].Entity(0), none)
	d.dimension(core.ConstraintPtLineDistance, p.CrestRadius, d.center, none, ridge[2].Entity(0), none)
	d.dimension(core.ConstraintPtPtDistance, p.CrestWidth, ridge[2].Entity(1), ridge[2].Entity(2), none, none)
	d.dimension(core.ConstraintAngle, p.FlankAngle, none, none, ridge[1].Entity(0), ridge[3].Entity(0))
	if p.Symmetric() {
		d.constrain(core.ConstraintEqualLengthLines, none, none, ridge[1].Entity(0), ridge[3].Entity(0), false)
	} else {
		// The upper flank's angle to the foot, which runs along the axis,
		// leans the ridge.
		d.dimension(core.ConstraintAngle, 90+p.UpperFlank, none, none, ridge[0].Entity(0), ridge[1].Entity(0))
	}
	d.placeRidge(i, ridge[2], 2, zc-crest)
}

// drawRoundRootRidge draws the ridge i, its foot centred at height z,
// with a flat crest and a rounded root (thread.Profile.RootArcs): each
// flank runs into an arc tangent to it, the arc into its tangent, which
// crosses the root into the part, and a line square to the axis from
// the tangent's end to the foot closes the ridge there, inside the part.
func (d *threadDrawer) drawRoundRootRidge(i int, z float64) {
	p, l, none := d.l.profile, d.l, core.NoEntity
	crest := p.CrestWidth / 2
	zc := z - l.footShift
	lower, upper := p.RootArcs()
	at := func(pt [2]float64) (float64, float64) { return pt[0], zc + pt[1] }
	_, lowZ := at(lower.End)
	_, upZ := at(upper.End)
	b := d.line(l.baseR, lowZ, l.baseR, upZ)
	top := d.line(p.CrestRadius, zc+crest, p.CrestRadius, zc-crest)
	// side draws the root on one side and its flank, from the foot to
	// the crest's end at crestZ: the line from the foot, the tangent,
	// the arc and the flank, as requests in that order.
	side := func(a thread.RootArc, footZ, crestZ float64) [4]core.HRequest {
		er, ez := at(a.End)
		ar, az := at(a.Turn)
		tr, tz := at(a.Tangent)
		cr, cz := at(a.Centre)
		var arc core.HRequest
		// Anticlockwise, a screw's upper arc runs from its turn to the
		// flank, its lower one from the flank; a nut's the other way.
		if (crestZ > zc) == (p.RootRadius < p.CrestRadius) {
			arc = d.arc(cr, cz, ar, az, tr, tz)
		} else {
			arc = d.arc(cr, cz, tr, tz, ar, az)
		}
		return [4]core.HRequest{d.line(l.baseR, footZ, er, ez), d.line(er, ez, ar, az), arc,
			d.line(tr, tz, p.CrestRadius, crestZ)}
	}
	ups := side(upper, upZ, zc+crest)
	downs := side(lower, lowZ, zc-crest)
	up, down := ups[3], downs[3]

	// The arc's ends: at its turn, and at the flank.
	ends := func(arc core.HRequest, upper bool) (turn, flank core.HEntity, flankAtEnd bool) {
		fromTurn := upper == (p.RootRadius < p.CrestRadius)
		if fromTurn {
			return arc.Entity(2), arc.Entity(3), true
		}
		return arc.Entity(3), arc.Entity(2), false
	}
	d.coincident(top.Entity(2), down.Entity(2))
	d.coincident(up.Entity(2), top.Entity(1))
	for _, s := range []struct {
		reqs   [4]core.HRequest
		corner core.HEntity
		upper  bool
		arc    thread.RootArc
	}{{ups, b.Entity(2), true, upper}, {downs, b.Entity(1), false, lower}} {
		foot, tangent, arc, flank := s.reqs[0], s.reqs[1], s.reqs[2], s.reqs[3]
		turn, onFlank, flankAtEnd := ends(arc, s.upper)
		d.coincident(s.corner, foot.Entity(1))
		d.coincident(foot.Entity(2), tangent.Entity(1))
		d.coincident(tangent.Entity(2), turn)
		d.coincident(onFlank, flank.Entity(1))
		d.constrain(core.ConstraintHorizontal, none, none, foot.Entity(0), none, false)
		d.dimension(core.ConstraintPtLineDistance, math.Abs(s.arc.End[0]-l.baseR), foot.Entity(2), none, b.Entity(0), none)
		slope := math.Atan2(math.Abs(s.arc.Turn[0]-s.arc.End[0]), math.Abs(s.arc.Turn[1]-s.arc.End[1])) * 180 / math.Pi
		d.dimension(core.ConstraintAngle, slope, none, none, b.Entity(0), tangent.Entity(0))
		d.constrain(core.ConstraintArcLineTangent, none, none, arc.Entity(0), tangent.Entity(0), !flankAtEnd)
		d.constrain(core.ConstraintArcLineTangent, none, none, arc.Entity(0), flank.Entity(0), flankAtEnd)
		d.dimension(core.ConstraintDiameter, 2*p.RootRoundRadius, none, none, arc.Entity(0), none)
		d.dimension(core.ConstraintPtLineDistance, math.Abs(s.arc.Centre[0]-l.baseR), arc.Entity(1), none, b.Entity(0), none)
	}

	d.constrain(core.ConstraintVertical, none, none, b.Entity(0), none, false)
	d.dimension(core.ConstraintPtLineDistance, l.baseR, d.center, none, b.Entity(0), none)
	d.constrain(core.ConstraintVertical, none, none, top.Entity(0), none, false)
	d.dimension(core.ConstraintPtLineDistance, p.CrestRadius, d.center, none, top.Entity(0), none)
	d.dimension(core.ConstraintPtPtDistance, p.CrestWidth, top.Entity(1), top.Entity(2), none, none)
	d.dimension(core.ConstraintAngle, p.FlankAngle, none, none, up.Entity(0), down.Entity(0))
	if p.Symmetric() {
		d.constrain(core.ConstraintEqualLengthLines, none, none, up.Entity(0), down.Entity(0), false)
	} else {
		d.dimension(core.ConstraintAngle, 90+p.UpperFlank, none, none, b.Entity(0), up.Entity(0))
	}
	d.placeRidge(i, top, 2, zc-crest)
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
		d.first[d.sk.ActiveGroup()] = hr.Entity(mark)
		d.dimension(core.ConstraintProjPtDistance, z, d.center, hr.Entity(mark), d.circle.Normal, none)
		return
	}
	d.dimension(core.ConstraintProjPtDistance, float64(i)*d.o.Spec.Pitch,
		d.first[d.sk.ActiveGroup()], hr.Entity(mark), d.circle.Normal, none)
}

// setHelix gives the thread's helix group hg its pitch and its params
// their values. The helix turns by four times its angle param while it
// moves by twice its distance param; a right-hand thread turns the way
// it moves, about the axis. The first start's helix keeps what made
// the thread.
func (d *threadDrawer) setHelix(g *Group, hg core.HGroup, main bool) {
	g.ValA, g.ValB, g.ExprB = 2, d.hand*d.o.Spec.Lead(), ""
	d.sk.MustParam(hg.Param(3)).Val = d.along * d.hand * d.turns * 2 * math.Pi / 4
	d.sk.MustParam(hg.Param(7)).Val = d.along * d.l.ridgeSweep / 2
	g.Thread = GroupThread{}
	if main {
		g.Thread = GroupThread{Circle: d.circle.H, ThreadOptions: d.o}
	}
}

// keepLengthDims records on the group that keeps the thread, hg, which
// dimensions carry its length, for a full-length thread to follow its
// face (thread_follow.go); it is called once every dimension is drawn.
func (d *threadDrawer) keepLengthDims(hg core.HGroup) {
	d.sk.Group(hg).Thread.LengthDims = slices.Clone(d.lengthDims)
}

// dimensionHelix holds the helix's length, without which it could be
// dragged longer. The helix group must be active, its entities made.
func (d *threadDrawer) dimensionHelix(hel *Group) {
	mark := d.first[hel.OpA]
	d.lengthDimension(core.ConstraintProjPtDistance, d.l.ridgeSweep-d.o.Length,
		hel.RemapEntity(mark, RemapLatheStart), hel.RemapEntity(mark, RemapLatheEnd),
		d.circle.Normal, core.NoEntity)
}

// startAngle is the angle of the ridge sketch of start i, with level
// starts: each a part of a turn on from the last.
func (d *threadDrawer) startAngle(i int) float64 {
	return d.start + float64(i)*2*math.Pi/float64(d.o.Spec.NumStarts())*d.along*d.hand
}

// startName is the name of what of start i: the first start's names
// are plain.
func startName(what string, i int) string {
	if i == 0 {
		return what
	}
	return what + "-" + strconv.Itoa(i+1)
}

// addSketch adds a sketch of the thread after the active group, in
// its plane at angle, and makes it active.
func (d *threadDrawer) addSketch(what string, angle float64) core.HGroup {
	sk := d.sk
	g := Group{Type: GroupDrawingWorkplane, Subtype: GroupWorkplaneByPointOrtho}
	d.placeSketch(&g, what, angle)
	gg := sk.addGroupAfterActive(g)
	gg.ActiveWorkplane = gg.H.Entity(0)
	return gg.H
}

// addSolid adds a solid group of the thread, of the active sketch,
// about the circle's axis.
func (d *threadDrawer) addSolid(g Group, what string, how CombineAs) *Group {
	sk := d.sk
	g.OpA = sk.ActiveGroup()
	g.Predef.Origin = d.center
	g.Predef.EntityB = d.circle.Normal
	g.MeshCombine = how
	d.nameSolid(&g, what)
	return sk.addGroupAfterActive(g)
}

// addHelix adds the helix group that sweeps the active sketch's ridge,
// and holds its length.
func (d *threadDrawer) addHelix(what string, main bool) *Group {
	sk := d.sk
	helix := Group{Type: GroupHelix, Subtype: GroupOneSided}
	hg := sk.nextGroupHandle()
	sk.Params = append(sk.Params, core.Param{H: hg.Param(3)}, core.Param{H: hg.Param(7)})
	d.setHelix(&helix, hg, main)
	hel := d.addSolid(helix, what, CombineUnion)
	if hel.H != hg {
		panic("sketch: thread helix group got another handle")
	}
	d.dimensionHelix(hel)
	return hel
}

// addStarts adds, after the active group, a ridge's sketch and helix
// for each level start past the first.
func (d *threadDrawer) addStarts(tg *threadGroups) {
	tg.starts = nil
	if !d.o.levelStarts() {
		return
	}
	for i := 1; i < d.o.Spec.NumStarts(); i++ {
		p := d.addSketch(startName("profile", i), d.startAngle(i))
		d.drawProfile()
		tg.starts = append(tg.starts, [2]core.HGroup{p, d.addHelix(startName("thread", i), false).H})
	}
}

// addChamfer adds, after the active group, the sketch of the thread's
// chamfers and the lathe that cuts them, when it has them.
func (d *threadDrawer) addChamfer(tg *threadGroups) {
	tg.chamfer = [2]core.HGroup{}
	if !d.l.chamfer {
		return
	}
	sketch := d.addSketch("end", 0)
	d.drawChamfer()
	tg.chamfer = [2]core.HGroup{sketch, d.addSolid(Group{Type: GroupLathe}, "chamfer", CombineDifference).H}
}

// drawChamfer draws, in the active sketch, what the chamfers take
// away, revolved: a triangle at each chamfered end, its long side on
// the cone.
func (d *threadDrawer) drawChamfer() {
	d.drawChamferAt(false)
	if d.l.farChamfer {
		d.drawChamferAt(true)
	}
}

// drawChamferAt draws what the chamfer takes away at the circle's end,
// or at the far end: from the cone's crossing of the end's plane, at
// 45° out past the crest, and back to where the cone runs on past the
// end, beyond every ridge's end; where the cone would close in on the
// axis there, a side parallel to the axis cuts it off.
func (d *threadDrawer) drawChamferAt(far bool) {
	l := d.l
	out := 1.0
	if d.o.Internal {
		out = -1
	}
	past := l.chamferPast
	// The cone is followed past the end by up to half the chamfer's
	// radius on a screw, so that the side across it stays off the axis.
	back := min(past/2, l.chamferR/2)
	rIn, rOut := l.chamferR-out*back, l.chamferOut
	reach := out * (rOut - l.chamferR)
	z := func(h float64) float64 {
		if far {
			return d.o.Length - h
		}
		return h
	}
	d.drawLathePolygon([]rz{{rIn, z(-back)}, {rOut, z(reach)}, {rOut, z(-past)}, {rIn, z(-past)}})
}

// rz is a point of a thread's sketch: its radius, and its height along
// the axis from the circle's plane.
type rz struct{ r, z float64 }

// drawLathePolygon draws, in the active sketch, the closed polygon of
// pts and holds it: a side parallel to the axis by its radius, a side
// across it by its height (from the far end when it is the blank's top
// or a chamfer's at the far end, so that the length's expression moves
// it), and a slanting side by its ends: at 45° where it meets a side
// across the axis at a corner, else by the height of its end on a side
// parallel to the axis and the length of the side across the axis it
// ends on.
func (d *threadDrawer) drawLathePolygon(pts []rz) {
	none := core.NoEntity
	const eps = 1e-9
	n := len(pts)
	sides := make([]core.HRequest, n)
	for i, p := range pts {
		q := pts[(i+1)%n]
		sides[i] = d.line(p.r, p.z, q.r, q.z)
	}
	d.closeLoop(sides)
	vertical := func(i int) bool { return math.Abs(pts[i].r-pts[(i+1)%n].r) < eps }
	across := func(i int) bool { return math.Abs(pts[i].z-pts[(i+1)%n].z) < eps }
	fromLength := func(z float64) bool { return z > d.o.Length/2 }
	height := func(typ core.ConstraintType, z float64, ptA, ptB, entity core.HEntity) {
		if fromLength(z) {
			d.lengthDimension(typ, z-d.o.Length, ptA, ptB, entity, none)
			return
		}
		d.dimension(typ, z, ptA, ptB, entity, none)
	}
	for i, side := range sides {
		switch {
		case vertical(i):
			d.constrain(core.ConstraintVertical, none, none, side.Entity(0), none, false)
			if pts[i].r == 0 {
				d.constrain(core.ConstraintPtOnLine, d.center, none, side.Entity(0), none, false)
			} else {
				d.dimension(core.ConstraintPtLineDistance, pts[i].r, d.center, none, side.Entity(0), none)
			}
		case across(i):
			d.constrain(core.ConstraintHorizontal, none, none, side.Entity(0), none, false)
			height(core.ConstraintPtLineDistance, pts[i].z, d.center, none, side.Entity(0))
		default:
			// A slanting side: each end is held on the side it meets.
			ends := [2]struct {
				other, vertex int
				at            core.HEntity
			}{{(i + n - 1) % n, i, side.Entity(1)}, {(i + 1) % n, (i + 1) % n, side.Entity(2)}}
			for _, end := range ends {
				switch {
				case vertical(end.other):
					height(core.ConstraintProjPtDistance, pts[end.vertex].z, d.center, end.at, d.circle.Normal)
				case across(end.other):
					// The side across ends at a corner held by the
					// sides on either side of it.
					o := sides[end.other]
					d.dimension(core.ConstraintPtPtDistance,
						math.Abs(pts[end.other].r-pts[(end.other+1)%n].r), o.Entity(1), o.Entity(2), none, none)
				}
			}
		}
	}
}

// drawChamferedBlank is drawBlank at a chamfered end: the side at the
// root turns, short of the end, into a cone at 45° that cuts the end
// back inside the ridges' feet (layChamfer), at both ends with a
// chamfered far end, and into the run-out's cone with a run-out; a
// cosmetic thread's side, at the crest, into the chamfer's cone.
func (d *threadDrawer) drawChamferedBlank() {
	l, o := d.l, d.o
	// A modelled thread's turning is at its root, its cone the
	// turning's own, short of the chamfer's; a cosmetic thread's, at
	// its crest, is the chamfer's.
	root, cone := l.profile.RootRadius, l.turnR
	if o.Cosmetic {
		root, cone = l.profile.CrestRadius, l.chamferR
	}
	out := 1.0
	if o.Internal {
		out = -1
	}
	zb, zt := l.blankBottom, l.blankTop
	zc := out * (root - cone)
	// The side at the root, from the blank's bottom to its top.
	side := []rz{{cone + out*zb, zb}, {root, zc}}
	switch {
	case l.runOut:
		side = append(side, rz{root, l.runFrom}, rz{l.runTo, zt})
	case l.farChamfer:
		side = append(side, rz{root, o.Length - zc}, rz{cone + out*(o.Length-zt), zt})
	default:
		side = append(side, rz{root, zt})
	}
	var pts []rz
	if o.Internal {
		pts = []rz{{0, zb}, {0, zt}}
		for _, p := range slices.Backward(side) {
			pts = append(pts, p)
		}
	} else {
		pts = append(slices.Clip(side), rz{l.blankOut, zt}, rz{l.blankOut, zb})
	}
	d.drawLathePolygon(pts)
}
