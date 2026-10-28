package sketch

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"

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
	// Clearance moves the thread away from its mate, in millimeters,
	// when it has no Class.
	Clearance float64
	// Class is the thread's tolerance class, as "6g" or "2B", which
	// sets its clearance (thread.ClassClearance); "" for Clearance as
	// given.
	Class string
	// Crest is the shape of the ridge's top: the standard flat one, cut
	// lower, or rounded, which prints better.
	Crest thread.Crest
	// RunOut is how far, in millimeters along the axis, the thread
	// fades out before its shoulder, as a die's thread does: over it a
	// cone cuts the ridge down from its crest towards its root. 0 is
	// none; a thread that is Through has no shoulder, and none.
	RunOut float64
	// LevelStarts starts every start of a thread of several at the
	// circle's plane, each its own ridge and helix turned by a part of
	// a turn from the last; otherwise one helix sweeps all the ridges,
	// stacked a pitch apart, which costs a pitch of length per start.
	LevelStarts bool
	// Chamfer cuts the thread's free end at 45° to the axis, from a
	// little inside the ridges' feet out past the crest: a screw's end
	// chamfered as ISO 4753 has it, a nut's mouth countersunk, so that
	// the thread starts clean rather than at a ridge's blunt end. The
	// free end is the circle's; a Through thread has a free end at its
	// length too, which is cut the same.
	Chamfer bool
	// Phase turns the thread about its axis, in degrees: its ridges
	// move away from the circle's plane by that part of the lead, and
	// are a pitch shorter, to leave them the room. A screw mate of two
	// threads phases the part it places instead (thread_fit.go).
	Phase float64
	// FullLength, for a thread made on a round face (AddThreadOnFace),
	// gives it the face's length, through when the face reaches the end
	// of the part: Length and Through are measured again from the face
	// when the thread is made and each time it is edited, and Length
	// follows the face at each regeneration (thread_follow.go).
	FullLength bool
	// LoadFlankBack turns a buttress thread's ridge about: its load
	// flank, the one nearly square to the axis, faces back towards
	// where the thread starts on a screw, and away from it in a nut,
	// rather than the other way. A screw and a nut mate when their load
	// flanks face each other, which depends on which way each thread
	// runs. Other threads' ridges are symmetric, and it leaves them as
	// they are.
	LoadFlankBack bool
	// Cosmetic records the thread on the part without modelling its
	// turns (thread_cosmetic_build.go): a screw is left at, or turned
	// down to, its major diameter and a nut bored to its minor one, and
	// drawings show the thread by the drafting convention. Crest and
	// LevelStarts shape only a modelled thread.
	Cosmetic bool
}

// FitClearance is the clearance the thread is made with: its class's,
// or Clearance when it has none.
//
// A cosmetic thread's class is a record, which its part doesn't
// follow: the part is left at the thread's nominal size, as a drawing
// calls for it, and the class is the machinist's to keep. Without a
// class, a clearance typed moves it as it would a modelled thread.
func (o ThreadOptions) FitClearance() (float64, error) {
	if o.Class == "" {
		return o.Clearance, nil
	}
	c, err := thread.ClassClearance(o.Spec, o.Internal, o.Class)
	if o.Cosmetic {
		return 0, err
	}
	return c, err
}

// phased tells whether the thread's ridges are moved along it from
// where a thread starts them; a cosmetic thread has none to move, and
// keeps its phase only for when it is modelled.
func (o ThreadOptions) phased() bool { return o.Phase != 0 && !o.Cosmetic }

// levelStarts tells whether the thread is made of a ridge and a helix
// per start.
func (o ThreadOptions) levelStarts() bool {
	return o.LevelStarts && !o.Cosmetic && o.Spec.NumStarts() > 1
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
	// runOut is whether the turning or the boring tapers, from the
	// root at runFrom to runTo at the thread's end.
	runOut         bool
	runFrom, runTo float64
	// ridgeTop is how far short of the thread's length the ridges end.
	ridgeTop float64
	// chamfer is whether the free ends are cut, by a cone through
	// chamferR at the end's plane, at 45° to the axis, that the cut
	// takes from chamferPast beyond the end out to chamferOut;
	// farChamfer whether the end at the thread's length is cut too.
	// turnR is where the turning or the boring meets the end's plane at
	// a chamfered end: its own cone, parallel to the chamfer's and a
	// little short of it.
	chamfer, farChamfer  bool
	chamferR, chamferOut float64
	chamferPast, turnR   float64
}

// layout places the thread on a circle of the given radius.
func (o ThreadOptions) layout(circleR float64) (threadLayout, error) { return o.layoutTry(circleR, 0) }

// layoutTry is layout with a chamfer made the try'th way of
// chamferTries.
func (o ThreadOptions) layoutTry(circleR float64, try int) (threadLayout, error) {
	s := o.Spec
	if err := s.Validate(); err != nil {
		return threadLayout{}, err
	}
	if !(o.Length > 0) || math.IsInf(o.Length, 0) {
		return threadLayout{}, errors.New("thread length must be positive")
	}
	clearance, err := o.FitClearance()
	if err != nil {
		return threadLayout{}, err
	}
	if math.IsNaN(clearance) || math.IsInf(clearance, 0) {
		return threadLayout{}, errors.New("thread clearance must be a number")
	}
	if math.IsNaN(o.Phase) || math.IsInf(o.Phase, 0) {
		return threadLayout{}, errors.New("thread phase must be a number")
	}
	if o.Crest < thread.CrestStandard || o.Crest > thread.CrestRounded {
		return threadLayout{}, fmt.Errorf("unknown thread crest %v", o.Crest)
	}
	if o.Cosmetic {
		// Its crest is the standard's, where the part's surface is.
		o.Crest = thread.CrestStandard
	}
	if o.Crest == thread.CrestRounded && s.Buttress() {
		return threadLayout{}, ErrButtressRounded
	}
	l := threadLayout{profile: s.Profile(o.Internal, clearance)}
	if o.LoadFlankBack {
		l.profile = l.profile.Turned()
	}
	p := l.profile

	// The ridge goes into the part by a tenth of the pitch, or by less
	// where that would bring its turns too close.
	into := math.Min(0.1*s.Pitch, (s.Pitch-p.RootWidth)/(2*p.Slope()))
	margin := s.Pitch
	if o.Internal {
		l.baseR = p.RootRadius + into
		l.blankIn, l.blankOut = 0, p.RootRadius
		// A hole drawn at about the thread's root, as at its nominal
		// size, would leave the boring's cylinder lying on the hole's,
		// where the Boolean fails: the boring then reaches a little past
		// the hole, still short of the ridges' foot.
		if near := into / 4; math.Abs(circleR-p.RootRadius) < near {
			l.blankOut = circleR + near
		}
	} else {
		l.baseR = p.RootRadius - into
		l.blankIn, l.blankOut = p.RootRadius, math.Max(circleR, p.CrestRadius)+margin
		// So too a shaft already turned to about the root.
		if near := into / 4; math.Abs(circleR-p.RootRadius) < near {
			l.blankIn = circleR - near
		}
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
	// blunt ends and the mating thread a lead-in; a chamfered end has
	// the ridge start past it, for the chamfer to cut.
	l.ridgeBottom, l.ridgeTop = s.Pitch/2, s.Pitch/2
	if o.Chamfer {
		l.layChamfer(o, chamferTries[try])
	}
	profiles := float64(s.NumStarts()-1)*s.Pitch + l.baseW
	if o.levelStarts() {
		profiles = l.baseW
	}
	// A phased thread's ridges are a pitch shorter, which leaves them
	// room to move by up to a quarter of a pitch down and three
	// quarters up (phaseRange).
	if o.phased() {
		profiles += s.Pitch
	}
	l.ridgeSweep = o.Length - (l.ridgeBottom + l.ridgeTop) - profiles
	if !(l.ridgeSweep > 0) {
		return threadLayout{}, fmt.Errorf("thread length must be more than %.4g mm for this pitch",
			l.ridgeBottom+l.ridgeTop+profiles)
	}
	if err := l.layRunOut(o); err != nil {
		return threadLayout{}, err
	}
	if err := l.checkChamfer(o); err != nil {
		return threadLayout{}, err
	}
	return l, nil
}

// chamferInto is how far, as a part of the pitch, a chamfer's cone
// reaches inside the ridges' feet at the end's plane: so far that the
// ridges' ends lie in it whole without starting far past the end, and
// that the turning's cone (chamferTries) keeps clear of it.
const chamferInto = 0.4

// chamferTry is one way of making a chamfered end, which changes only
// what the chamfer takes away: turnInto is how far, as a part of the
// pitch, the turning's or the boring's cone reaches inside the ridges'
// feet at the end's plane, and lower how much further past the end
// the ridges start.
type chamferTry struct{ turnInto, lower float64 }

// chamferTries are the ways a chamfered end is made, the first that
// comes out sound kept: where the ridges' ends and the cones lie as
// the Boolean takes them least well depends on the thread, and the
// Boolean that fails at one of these, at every plane for the ridges,
// may not at another (found over the thread cases: 32 of 33 at the
// first, all at the first two).
var chamferTries = [...]chamferTry{{0.2, 0}, {0.2, 0.4}, {0.1, 0}, {0.05, 0}, {0.15, 0.2}, {0.25, 0.25}, {0.1, 0.4}}

// layChamfer places the chamfers, as the try'th of chamferTries makes
// them: a cone at 45° from chamferInto of a pitch inside the ridges'
// feet at the end's plane, out past the crest. The ridges start (and,
// at a chamfered far end, stop) past the end, so far that the cone
// takes their blunt ends whole and the thread comes out of the cone
// as a die cuts it.
//
// So the ridges cross the end's plane. Had the part a flat end there,
// the ridges' feet, which go into it, would cut a sliver of it that the
// Boolean loses; so the turning or the boring cuts the end back first,
// by a cone of its own inside the feet, which the ridges then stand
// on, and the chamfer's cone, parallel to it and short of it, takes it
// away with the ridges' ends: no two faces lie on each other.
func (l *threadLayout) layChamfer(o ThreadOptions, try chamferTry) {
	p, pitch := l.profile, o.Spec.Pitch
	depth := math.Abs(p.CrestRadius - p.RootRadius)
	l.chamfer, l.farChamfer = true, o.Through
	// out is the way from the axis to the part: outwards on a screw,
	// inwards in a nut.
	out := 1.0
	if o.Internal {
		out = -1
	}
	l.chamferR = l.baseR - out*chamferInto*pitch
	l.turnR = l.baseR - out*try.turnInto*pitch
	l.chamferOut = p.CrestRadius + out*depth/4
	// A ridge's end lies in the cone where its foot's corner nearest the
	// end does: so far into the cone, and a tenth of a pitch more. Starts
	// stacked a pitch apart have their ends a pitch apart, all in it.
	past := l.baseW + (0.1+try.lower-chamferInto)*pitch
	if !o.levelStarts() {
		past += float64(o.Spec.NumStarts()-1) * pitch
	}
	// A phase moves the ridges by up to a quarter of a pitch towards the
	// circle's plane and three quarters away, and they are a pitch
	// shorter for it (layoutTry): they start and stop so much further
	// past the ends that their ends lie in the cones however they move.
	bottom, top, over := past, past, past
	if o.phased() {
		bottom, top = past+0.75*pitch, past+1.25*pitch
		over = max(bottom+0.25*pitch, top-0.25*pitch)
	}
	l.ridgeBottom = -bottom
	if l.farChamfer {
		l.ridgeTop = -top
	}
	l.chamferPast = over + 0.25*pitch
}

// ErrButtressRounded is returned for a buttress thread with a rounded
// crest: an arc tangent to flanks at different angles has no top where
// the standard crest is, and is not offered.
var ErrButtressRounded = errors.New("a buttress thread's crest can't be rounded")

// ErrChamferTooFine is returned for a chamfered screw so small for its
// pitch that the cones, which run on past the end by a pitch, would
// reach the axis there (a Tr12x3).
var ErrChamferTooFine = errors.New("thread is too fine for a chamfer")

// chamferReach is how far along the axis from its end a chamfer cuts
// the part: to where its cone leaves the crest.
func (l *threadLayout) chamferReach() float64 {
	return math.Abs(l.profile.CrestRadius - l.chamferR)
}

// checkChamfer refuses chamfers that would meet each other, or the
// run-out, or whose turning's cone would reach the axis past the end.
// A rounded crest is chamfered as any other: the cone cuts it, meeting
// its flanks just past their feet at a shallow angle.
func (l *threadLayout) checkChamfer(o ThreadOptions) error {
	if !l.chamfer {
		return nil
	}
	reach := l.chamferReach()
	// The cones run on past the end to the blank's bottom: the
	// chamfer's, which is the cosmetic thread's turning's, and the
	// modelled thread's turning's, which is outside it.
	if !o.Internal && l.chamferR+l.blankBottom <= 0 {
		return ErrChamferTooFine
	}
	free := o.Length
	if l.runOut {
		free = l.runFrom
	}
	ends := 1.0
	if l.farChamfer {
		ends = 2
	}
	if ends*reach >= free {
		return fmt.Errorf("thread length must be more than %.4g mm for its chamfers", ends*reach)
	}
	return nil
}

// layRunOut places the run-out: the turned or bored surface leaves the
// root RunOut short of the thread's end and tapers to the crest there,
// so the ridge sinks into it, as a die's thread does at a shoulder.
func (l *threadLayout) layRunOut(o ThreadOptions) error {
	if math.IsNaN(o.RunOut) || math.IsInf(o.RunOut, 0) || o.RunOut < 0 {
		return errors.New("thread run-out must be a length of 0 or more")
	}
	if o.RunOut == 0 {
		return nil
	}
	if o.Through {
		return errors.New("a through thread has no shoulder to run out at")
	}
	// The run-out keeps the ridges' end off its cone's circle, which
	// moving them would undo.
	if o.phased() {
		return errors.New("a thread with a run-out can't be phased")
	}
	if o.RunOut >= o.Length {
		return fmt.Errorf("thread run-out must be less than its length, %.4g mm", o.Length)
	}
	l.runOut = true
	l.runFrom = o.Length - o.RunOut
	// A screw's cone stops a tenth of the thread's depth short of the
	// crest, which a rod as thick as the thread has as its radius: the
	// cone would end on the rod's side, touching it along a circle.
	p := l.profile
	l.runTo = p.CrestRadius
	if !o.Internal {
		l.runTo -= 0.1 * (p.CrestRadius - p.RootRadius)
	}
	l.ridgeSweep += l.clearOfRunOut(o)
	if !(l.ridgeSweep > 0) {
		return fmt.Errorf("thread run-out of %.4g mm leaves no thread before it", o.RunOut)
	}
	return nil
}

// runOutClear is how far, as a part of the pitch, a flank of a ridge's
// end is kept from the circle where the root turns into the run-out's
// cone.
const runOutClear = 0.1

// clearOfRunOut is how far to move the ridges' end along the axis so
// that no flank of it meets the root's circle at runFrom. There the
// circle would run through the corner of the ridge's flat end and its
// flank, within the Boolean's tolerance of both, and the Boolean
// dropped the cone's quarter it lies in (M10 with a run-out of 2 mm, a
// flank 0.04 mm above the circle); turning the ridges' plane does not
// help, since the end turns with it. The least move that clears every
// flank of every start's end, up or down by at most twice runOutClear
// of a pitch, is taken: the ridge still stops short of the shoulder,
// and only the end of its last turn moves.
func (l *threadLayout) clearOfRunOut(o ThreadOptions) float64 {
	s := o.Spec
	m := runOutClear * s.Pitch
	end := l.ridgeBottom + l.baseW/2 + l.ridgeSweep
	var gaps []float64
	for i := range s.NumStarts() {
		z := end
		if !o.levelStarts() {
			z += float64(i) * s.Pitch
		}
		lo, hi := l.flanksAt(z, l.profile.RootRadius)
		gaps = append(gaps, lo-l.runFrom, hi-l.runFrom)
	}
	clears := func(shift float64) bool {
		return !slices.ContainsFunc(gaps, func(g float64) bool { return math.Abs(g+shift) < m })
	}
	moves := []float64{0}
	for _, g := range gaps {
		moves = append(moves, -m-g, m-g)
	}
	slices.SortStableFunc(moves, func(a, b float64) int { return cmp.Compare(math.Abs(a), math.Abs(b)) })
	for _, shift := range moves {
		if math.Abs(shift) < 2*m && clears(shift) {
			return shift
		}
	}
	return 0
}

// flanksAt is where along the thread the flanks of a ridge whose foot's
// middle is at z cross the radius r: a buttress ridge's middle lies
// further from its crest's the further r is from the crest.
func (l threadLayout) flanksAt(z, r float64) (lo, hi float64) {
	p := l.profile
	mid, half := z-p.MiddleShift(l.baseR)+p.MiddleShift(r), p.WidthAt(r)/2
	return mid - half, mid + half
}

// CheckThread reports why AddThread would refuse to make the thread,
// without changing anything: among others, a thread longer than the
// solid so far, which it is made on.
func (sk *Sketch) CheckThread(he core.HEntity, o ThreadOptions) error {
	return sk.checkThread(he, o, sk.solidAt(sk.ActiveGroup()))
}

// CheckThreadEdit reports why EditThread would refuse to make the
// thread whose helix group is hg again with o, without changing
// anything.
func (sk *Sketch) CheckThreadEdit(hg core.HGroup, o ThreadOptions) error {
	tg, err := sk.threadGroupsOf(hg)
	if err != nil {
		return err
	}
	// The blank sketch passes on the solid from before the thread.
	return sk.checkThread(sk.Group(hg).Thread.Circle, o, sk.solidAt(tg.blank))
}

// checkThread is CheckThread with the thread made on part's solid so
// far.
func (sk *Sketch) checkThread(he core.HEntity, o ThreadOptions, part *Group) error {
	circle := sk.Entity(he)
	if circle == nil || !circle.IsCircle() {
		return errors.New("thread must be made on a circle or an arc")
	}
	if o.LengthExpr != "" {
		if _, _, err := sk.parseNamed(o.LengthExpr); err != nil {
			return fmt.Errorf("thread length: %w", err)
		}
	}
	l, err := o.layout(circle.CircleGetRadiusNum(sk))
	if err != nil {
		return err
	}
	return sk.checkThreadPart(he, o, l, part)
}

// ThreadResult is what AddThread made.
type ThreadResult struct {
	// Group is the group that keeps what made the thread: the helix of
	// its first start, or a cosmetic thread's sketch. The thread's last
	// group is left active.
	Group core.HGroup
	// Sound tells whether the solid came out whole: both Booleans
	// succeeded and the mesh is watertight.
	Sound bool
}

// AddThread makes a screw thread on the circle or arc he, along its
// normal, after the active group: a sketch and a lathe group that turn
// the part down to the root of the thread (or bore it out, for an
// internal thread), then a sketch of the ridge and a helix group that
// sweeps it; with level starts, a sketch and a helix for each further
// start. A cosmetic thread is a sketch, and a lathe after it only where
// the part must be turned or bored (addCosmeticThread). It generates the
// sketch, and says whether the solid came out sound.
func (sk *Sketch) AddThread(he core.HEntity, o ThreadOptions) (ThreadResult, error) {
	if err := sk.CheckThread(he, o); err != nil {
		return ThreadResult{}, err
	}
	if o.Cosmetic {
		return sk.addCosmeticThread(he, o), nil
	}
	d := sk.newThreadDrawer(he, o, 0)

	var tg threadGroups
	tg.blank = d.addSketch("blank", 0)
	d.drawBlank()
	tg.lathe = d.addSolid(Group{Type: GroupLathe}, d.latheName(), CombineDifference).H
	tg.profile = d.addSketch("profile", d.start)
	d.drawProfile()
	tg.helix = d.addHelix("thread", true).H
	d.addStarts(&tg)
	d.addChamfer(&tg)
	d.keepLengthDims(tg.helix)

	sk.GenerateAllTyped(GenerateAll)
	res := ThreadResult{Group: tg.helix, Sound: sk.threadIsSound(tg)}
	if !res.Sound && d.retries() {
		return sk.redrawThread(tg.helix, o, 1)
	}
	return res, nil
}

// threadIsSound tells whether every solid of the thread came out
// sound.
func (sk *Sketch) threadIsSound(tg threadGroups) bool {
	for _, h := range tg.solids() {
		if !sk.solidIsSound(h) {
			return false
		}
	}
	return true
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
