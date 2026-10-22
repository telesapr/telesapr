package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/thread"
)

// A screw thread is drawn with the groups a user would make by hand, so
// the file it saves is an ordinary one. Its helix group keeps what made
// it (Group.Thread), so that the four can be edited as one thread
// (thread_edit.go).

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
	// LengthExpr is the expression Length comes from, in the document's
	// unit, or "" for a plain number. The dimensions of the thread's
	// length then carry it, so that a parameter drives the thread.
	LengthExpr string
	// Reverse runs the thread against the circle's normal.
	Reverse bool
	// Through carries the turning or the boring past Length, for a
	// part that ends there; otherwise it stops at a shoulder.
	Through bool
	// Clearance moves the thread away from its mate, in millimeters.
	Clearance float64
	// Crest is the shape of the ridge's top: the standard flat one, cut
	// lower, or rounded, which prints better.
	Crest thread.Crest
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
	if o.Crest < thread.CrestStandard || o.Crest > thread.CrestRounded {
		return threadLayout{}, fmt.Errorf("unknown thread crest %v", o.Crest)
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
	// The crest is shaped once the ridge's foot is placed, which it
	// leaves where it was.
	l.profile = p.WithCrest(o.Crest)

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
	circle := sk.Entity(he)
	if circle == nil || !circle.IsCircle() {
		return errors.New("thread must be made on a circle or an arc")
	}
	if o.LengthExpr != "" {
		if _, _, err := sk.parseNamed(o.LengthExpr); err != nil {
			return fmt.Errorf("thread length: %w", err)
		}
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
	d := sk.newThreadDrawer(he, o, nil)

	addSketch := func(what string, angle float64) core.HGroup {
		g := Group{Type: GroupDrawingWorkplane, Subtype: GroupWorkplaneByPointOrtho}
		d.placeSketch(&g, what, angle)
		gg := sk.addGroupAfterActive(g)
		gg.ActiveWorkplane = gg.H.Entity(0)
		return gg.H
	}
	addSolid := func(g Group, what string, how CombineAs) *Group {
		g.OpA = sk.ActiveGroup()
		g.Predef.Origin = d.center
		g.Predef.EntityB = d.circle.Normal
		g.MeshCombine = how
		d.nameSolid(&g, what)
		return sk.addGroupAfterActive(g)
	}

	var tg threadGroups
	tg.blank = addSketch("blank", 0)
	d.drawBlank()
	tg.lathe = addSolid(Group{Type: GroupLathe}, d.latheName(), CombineDifference).H
	tg.profile = addSketch("profile", d.start)
	d.drawProfile()

	helix := Group{Type: GroupHelix, Subtype: GroupOneSided}
	hg := sk.nextGroupHandle()
	sk.Params = append(sk.Params, core.Param{H: hg.Param(3)}, core.Param{H: hg.Param(7)})
	d.setHelix(&helix, hg)
	hel := addSolid(helix, "thread", CombineUnion)
	if hel.H != hg {
		panic("sketch: thread helix group got another handle")
	}
	tg.helix = hel.H
	d.dimensionHelix(hel)

	sk.GenerateAllTyped(GenerateAll)
	sound := sk.solidIsSound(tg.lathe) && sk.solidIsSound(tg.helix)
	return ThreadResult{Group: tg.helix, Sound: sound}, nil
}

// solidIsSound tells whether the group's Boolean succeeded and the
// mesh of its solid is watertight and doesn't cross itself.
func (sk *Sketch) solidIsSound(hg core.HGroup) bool {
	g := sk.Group(hg)
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

// addGroupAfterActive adds g after the active group, as a feature of the Features menu does:
// in the colour of the solid so far, the later groups moved on. It
// makes g active and generates its entities, without solving.
func (sk *Sketch) addGroupAfterActive(g Group) *Group {
	g.Visible = true
	g.Scale = 1
	g.Color = core.NewRGBA(100, 100, 100, 255)
	active := sk.ActiveGroup()
	if g.IsMeshGroup() {
		if running := sk.RunningMeshGroupFor(active); running != nil {
			g.Color = running.Color
		}
	}
	afterActive := false
	for _, hg := range sk.GroupOrder {
		gi := sk.Group(hg)
		if afterActive {
			gi.Order++
		}
		if gi.H == active {
			g.Order = gi.Order + 1
			afterActive = true
		}
	}
	hg := sk.AddGroup(g)
	sk.Group(hg).Clean = false
	sk.Active = hg
	sk.GenerateAllTyped(GenerateRegen)
	return sk.Group(hg)
}
