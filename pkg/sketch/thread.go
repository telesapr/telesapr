package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/thread"
)

// This file is not a port: C++ SolveSpace has no screw threads. It
// draws one with the groups a user would make by hand, so the file it
// saves is an ordinary one that C++ opens and regenerates.

// ThreadOptions is a screw thread to make on a circle.
type ThreadOptions struct {
	Spec thread.Spec
	// Internal makes the thread of a nut: a hole is bored and the
	// ridge put inside it. Otherwise the thread is a screw's: the part
	// is turned down and the ridge put around it.
	Internal bool
	// Length is how far the thread reaches from the circle's plane,
	// in millimeters.
	Length float64
	// Reverse runs the thread against the circle's normal.
	Reverse bool
	// Through carries the turning or the boring past Length, for a
	// part that ends there; otherwise it stops at a shoulder.
	Through bool
	// Clearance moves the thread away from its mate, in millimeters.
	Clearance float64
}

// threadLayout is where AddThread draws, in millimeters: radii from
// the axis, and heights along it from the circle's plane.
type threadLayout struct {
	profile thread.Profile
	// baseR is the radius of the ridge's foot, which lies past the root
	// so that the ridge and the part overlap.
	baseR, baseW float64
	// blankIn and blankOut are the radii that the turning or the boring
	// takes away between; blankBottom and blankTop the heights.
	blankIn, blankOut       float64
	blankBottom, blankTop   float64
	ridgeBottom, ridgeSweep float64
}

// layout places the thread on a circle of the given radius.
func (o ThreadOptions) layout(circleR float64) (threadLayout, error) {
	s := o.Spec
	if err := s.Validate(); err != nil {
		return threadLayout{}, err
	}
	if !(o.Length > 0) || math.IsInf(o.Length, 0) {
		return threadLayout{}, errors.New("thread length must be positive")
	}
	if math.IsNaN(o.Clearance) || math.IsInf(o.Clearance, 0) {
		return threadLayout{}, errors.New("thread clearance must be a number")
	}
	l := threadLayout{profile: s.Profile(o.Internal, o.Clearance)}
	p := l.profile

	// The ridge goes into the part by a tenth of the pitch, or by less
	// where that would bring its turns too close.
	slope := 2 * math.Tan(p.FlankAngle/2*math.Pi/180)
	into := math.Min(0.1*s.Pitch, (s.Pitch-p.RootWidth)/(2*slope))
	margin := s.Pitch
	if o.Internal {
		l.baseR = p.RootRadius + into
		l.blankIn, l.blankOut = 0, p.RootRadius
	} else {
		l.baseR = p.RootRadius - into
		l.blankIn, l.blankOut = p.RootRadius, math.Max(circleR, p.CrestRadius)+margin
	}
	l.baseW = p.WidthAt(l.baseR)
	if !(p.CrestRadius > 0 && p.RootRadius > 0 && l.baseR > 0) || !(into > 0) {
		return threadLayout{}, errors.New("thread clearance is too large for its size")
	}

	l.blankBottom, l.blankTop = -margin, o.Length
	if o.Through {
		l.blankTop += margin
	}
	// The ridge stops half a pitch short of both ends, which leaves it
	// blunt ends and the mating thread a lead-in.
	l.ridgeBottom = s.Pitch / 2
	profiles := float64(s.NumStarts()-1)*s.Pitch + l.baseW
	l.ridgeSweep = o.Length - 2*l.ridgeBottom - profiles
	if !(l.ridgeSweep > 0) {
		return threadLayout{}, fmt.Errorf("thread length must be more than %.4g mm for this pitch",
			2*l.ridgeBottom+profiles)
	}
	return l, nil
}

// CheckThread reports why AddThread would refuse to make the thread,
// without changing anything.
func (sk *Sketch) CheckThread(he core.HEntity, o ThreadOptions) error {
	circle := sk.GetEntity(he)
	if circle == nil || !circle.IsCircle() {
		return errors.New("thread must be made on a circle or an arc")
	}
	_, err := o.layout(circle.CircleGetRadiusNum(sk))
	return err
}

// ThreadResult is what AddThread made.
type ThreadResult struct {
	// Group is the helix group, the last of the thread's four, which is
	// left active.
	Group core.HGroup
	// Sound tells whether the solid came out whole: both Booleans
	// succeeded and the mesh is watertight.
	Sound bool
}

// AddThread makes a screw thread on the circle or arc he, along its
// normal, after the active group: a sketch and a lathe group that turn
// the part down to the root of the thread (or bore it out, for an
// internal thread), then a sketch of the ridge and a helix group that
// sweeps it. It generates the sketch, and says whether the solid came
// out sound.
func (sk *Sketch) AddThread(he core.HEntity, o ThreadOptions) (ThreadResult, error) {
	if err := sk.CheckThread(he, o); err != nil {
		return ThreadResult{}, err
	}
	circle := sk.GetEntity(he)
	center := circle.Point[0]
	radius := circle.CircleGetRadiusNum(sk)
	l, _ := o.layout(radius)
	p := l.profile

	origin := sk.MustGetEntity(center).PointGetNum(sk)
	q := sk.MustGetEntity(circle.Normal).NormalGetNum(sk)
	along := 1.0
	if o.Reverse {
		along = -1
	}
	hand := 1.0
	if o.Spec.LeftHand {
		hand = -1
	}
	turns := l.ridgeSweep / o.Spec.Lead()

	// The sketches lie in planes through the axis: a radius to the
	// right, the axis upwards.
	u, v := q.RotationU(), q.RotationN().ScaledBy(along)
	at := func(r, z float64) geom.Vector {
		return origin.Plus(u.ScaledBy(r)).Plus(v.ScaledBy(z))
	}
	name := o.Spec.String()

	// plane is the sketch plane at the given angle about the circle's
	// normal.
	plane := func(angle float64) geom.Quaternion {
		return geom.QuaternionFromBasis(q.RotationU().RotatedAroundAxis(q.RotationN(), angle), v)
	}
	addSketch := func(what string, angle float64) core.HGroup {
		g := Group{Type: GroupDrawingWorkplane, Subtype: GroupWorkplaneByPointOrtho, Name: name + "-" + what}
		g.Predef.Q = plane(angle)
		g.Predef.Origin = center
		u = g.Predef.Q.RotationU()
		gg := sk.addGroupAfterActive(g)
		gg.ActiveWorkplane = gg.H.Entity(0)
		return gg.H
	}
	addSolid := func(g Group, what string, how CombineAs) *Group {
		g.Name = name + "-" + what
		g.OpA = sk.ActiveGroup()
		g.Predef.Origin = center
		g.Predef.EntityB = circle.Normal
		g.MeshCombine = how
		return sk.addGroupAfterActive(g)
	}
	// line draws a line segment of the active sketch.
	line := func(r0, z0, r1, z1 float64) core.HRequest {
		hr := sk.AddRequestOfType(RequestLineSegment, false)
		sk.MustGetEntity(hr.Entity(1)).PointForceTo(sk, at(r0, z0))
		sk.MustGetEntity(hr.Entity(2)).PointForceTo(sk, at(r1, z1))
		return hr
	}
	closeLoop := func(lines []core.HRequest) {
		for i := range lines {
			sk.ConstrainCoincident(lines[i].Entity(2), lines[(i+1)%len(lines)].Entity(1))
		}
	}
	constrain := func(typ core.ConstraintType, ptA, ptB, entityA, entityB core.HEntity) {
		sk.Constrain(typ, ptA, ptB, entityA, entityB, false, false)
	}
	// dimension adds a constraint with a value, which the drawing already
	// has: the sign and the supplement are whatever make it so.
	dimension := func(typ core.ConstraintType, val float64, ptA, ptB, entityA, entityB core.HEntity) {
		c := core.Constraint{}
		c.Group = sk.ActiveGroup()
		c.Workplane = sk.ActiveWorkplane()
		c.Type = typ
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
		sk.AddConstraintRemember(&c, false)
	}
	none := core.NoEntity

	// What the turning or the boring takes away: a rectangle, revolved.
	addSketch("blank", 0)
	blank := []core.HRequest{
		line(l.blankIn, l.blankBottom, l.blankIn, l.blankTop),
		line(l.blankIn, l.blankTop, l.blankOut, l.blankTop),
		line(l.blankOut, l.blankTop, l.blankOut, l.blankBottom),
		line(l.blankOut, l.blankBottom, l.blankIn, l.blankBottom),
	}
	closeLoop(blank)
	constrain(core.ConstraintVertical, none, none, blank[0].Entity(0), none)
	constrain(core.ConstraintVertical, none, none, blank[2].Entity(0), none)
	constrain(core.ConstraintHorizontal, none, none, blank[1].Entity(0), none)
	constrain(core.ConstraintHorizontal, none, none, blank[3].Entity(0), none)
	if l.blankIn == 0 {
		constrain(core.ConstraintPtOnLine, center, none, blank[0].Entity(0), none)
	} else {
		dimension(core.ConstraintPtLineDistance, l.blankIn, center, none, blank[0].Entity(0), none)
	}
	dimension(core.ConstraintPtLineDistance, l.blankOut, center, none, blank[2].Entity(0), none)
	dimension(core.ConstraintPtLineDistance, l.blankTop, center, none, blank[1].Entity(0), none)
	dimension(core.ConstraintPtLineDistance, -l.blankBottom, center, none, blank[3].Entity(0), none)

	what := "core"
	if o.Internal {
		what = "bore"
	}
	lathe := addSolid(Group{Type: GroupLathe}, what, CombineDifference).H

	// The ridges, one for each start, a pitch apart. A revolved surface
	// is made in quarters, from the plane of its sketch on, and a ridge
	// has a flat end in that plane and another where its sweep stops.
	// Those ends would meet the turned surface along the joins of its
	// quarters, where a Boolean is least sure of itself: so the ridges'
	// plane is turned to put both ends as far from the joins as can be,
	// which is an eighth of a turn or more.
	quarter := math.Pi / 2
	past := math.Mod(turns*2*math.Pi, quarter)
	start := (quarter - past) / 2
	if past > quarter/2 {
		start = (2*quarter - past) / 2
	}
	start *= along * hand
	addSketch("profile", start)
	var first core.HRequest
	for i := range o.Spec.NumStarts() {
		z := l.ridgeBottom + float64(i)*o.Spec.Pitch + l.baseW/2
		base, crest := l.baseW/2, p.CrestWidth/2
		ridge := []core.HRequest{
			line(l.baseR, z-base, l.baseR, z+base),
			line(l.baseR, z+base, p.CrestRadius, z+crest),
			line(p.CrestRadius, z+crest, p.CrestRadius, z-crest),
			line(p.CrestRadius, z-crest, l.baseR, z-base),
		}
		closeLoop(ridge)
		constrain(core.ConstraintVertical, none, none, ridge[0].Entity(0), none)
		constrain(core.ConstraintVertical, none, none, ridge[2].Entity(0), none)
		dimension(core.ConstraintPtLineDistance, l.baseR, center, none, ridge[0].Entity(0), none)
		dimension(core.ConstraintPtLineDistance, p.CrestRadius, center, none, ridge[2].Entity(0), none)
		dimension(core.ConstraintPtPtDistance, p.CrestWidth, ridge[2].Entity(1), ridge[2].Entity(2), none, none)
		dimension(core.ConstraintAngle, p.FlankAngle, none, none, ridge[1].Entity(0), ridge[3].Entity(0))
		constrain(core.ConstraintEqualLengthLines, none, none, ridge[1].Entity(0), ridge[3].Entity(0))
		// Along the axis: the first ridge from the circle's plane, the
		// others from the first.
		if i == 0 {
			first = ridge[2]
			dimension(core.ConstraintProjPtDistance, z-crest, center, first.Entity(2), circle.Normal, none)
		} else {
			dimension(core.ConstraintProjPtDistance, float64(i)*o.Spec.Pitch,
				first.Entity(2), ridge[2].Entity(2), circle.Normal, none)
		}
	}

	// The helix turns by four times its angle param while it moves by
	// twice its distance param; a right-hand thread turns the way it
	// moves, about the axis.
	helix := Group{Type: GroupHelix, Subtype: GroupOneSided, ValA: 2, ValB: hand * o.Spec.Lead()}
	hg := sk.nextGroupHandle()
	sk.Params = append(sk.Params,
		core.Param{H: hg.Param(3), Val: along * hand * turns * 2 * math.Pi / 4},
		core.Param{H: hg.Param(7), Val: along * l.ridgeSweep / 2})
	hel := addSolid(helix, "thread", CombineUnion)
	if hel.H != hg {
		panic("sketch: thread helix group got another handle")
	}
	// Without its length the helix could be dragged longer.
	dimension(core.ConstraintProjPtDistance, l.ridgeSweep,
		hel.RemapEntity(first.Entity(2), RemapLatheStart), hel.RemapEntity(first.Entity(2), RemapLatheEnd),
		circle.Normal, none)

	sk.GenerateAllTyped(GenerateAll)
	sound := sk.solidIsSound(lathe) && sk.solidIsSound(hel.H)
	return ThreadResult{Group: hel.H, Sound: sound}, nil
}

// solidIsSound tells whether the group's Boolean succeeded and the
// mesh of its solid is watertight and doesn't cross itself.
func (sk *Sketch) solidIsSound(hg core.HGroup) bool {
	g := sk.GetGroup(hg)
	if g.BooleanFailed {
		return false
	}
	g.GenerateDisplayItems(sk)
	if g.DisplayMesh.IsEmpty() {
		return false
	}
	var naked srf.SEdgeList
	var inters, leaks bool
	srf.SKdNodeFrom(&g.DisplayMesh).MakeCertainEdgesInto(&naked, srf.EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
	return !inters && !leaks && len(naked.Edges) == 0
}

// nextGroupHandle is the handle AddGroup will give the next group.
func (sk *Sketch) nextGroupHandle() core.HGroup {
	var h uint32
	for i := range sk.Groups {
		h = max(h, sk.Groups[i].H.V)
	}
	return core.HGroup{V: h + 1}
}

// addGroupAfterActive adds g after the active group, as New Group does
// (C++ Group::MenuGroup): in the colour of the solid so far, the later
// groups moved on. It makes g active and generates its entities,
// without solving.
func (sk *Sketch) addGroupAfterActive(g Group) *Group {
	g.Visible = true
	g.Scale = 1
	g.Color = core.NewRgbaColor(100, 100, 100, 255)
	active := sk.ActiveGroup()
	if g.IsMeshGroup() {
		if running := sk.GetRunningMeshGroupFor(active); running != nil {
			g.Color = running.Color
		}
	}
	afterActive := false
	for _, hg := range sk.GroupOrder {
		gi := sk.GetGroup(hg)
		if afterActive {
			gi.Order++
		}
		if gi.H == active {
			g.Order = gi.Order + 1
			afterActive = true
		}
	}
	hg := sk.AddGroup(g)
	sk.GetGroup(hg).Clean = false
	sk.Active = hg
	sk.GenerateAllTyped(GenerateRegen)
	return sk.GetGroup(hg)
}
