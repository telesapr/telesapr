package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/thread"
)

// A thread is drawn from its circle the same way whatever it is to
// fit, so a nut and its screw interleave only where the nut happens to
// sit a whole number of pitches and a half from where the screw's ridge
// starts. Two ways put it right. A thread's phase moves its ridges along
// it by a part of the lead (threadDrawer.shift). A screw mate of two
// threaded circles moves the part it places along the axis, by less
// than half a pitch, to where the ridges lie between each other
// (ThreadFitMove); turning it by the mate then keeps them there. The
// mate reads the threads off the models it links: a link group carries
// its model's threads (LinkThreads).

// LinkedThread is a screw thread of a model as a file linking it sees
// it: the circle it is on and the point that holds its first ridge (none
// for a cosmetic thread), by the model's handles, and its options.
type LinkedThread struct {
	Circle, Mark core.HEntity
	ThreadOptions
}

// LinkThreads are the model's screw threads, for a file that links it:
// those whose groups are still a thread's.
func (sk *Sketch) LinkThreads() []LinkedThread {
	var out []LinkedThread
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if !g.Thread.IsThread() {
			continue
		}
		// A cosmetic thread has no ridge to fit, but gives its lead.
		if g.Thread.Cosmetic {
			out = append(out, LinkedThread{Circle: g.Thread.Circle, ThreadOptions: g.Thread.ThreadOptions})
			continue
		}
		tg, err := sk.threadGroupsOf(g.H)
		circle := sk.Entity(g.Thread.Circle)
		if err != nil || circle == nil {
			continue
		}
		if mark := sk.ridgeMark(tg.profile, circle.Point[0]); mark != nil {
			out = append(out, LinkedThread{Circle: g.Thread.Circle, Mark: mark.PtB, ThreadOptions: g.Thread.ThreadOptions})
		}
	}
	return out
}

// threadAt is where a thread's ridges lie: its circle's centre, the
// unit vector it runs along from there, a unit radius that its first
// ridge's sketch plane points along, and how far along the middle of
// that ridge is.
type threadAt struct {
	origin, along, radial geom.Vector
	middle                float64
}

// markBelowCentre is how far along the thread the point that holds a
// ridge in its sketch lies before the ridge's middle at its pitch
// radius: a flat crest's near end, or a rounded crest's centre, on it,
// and a buttress ridge's crest, and its rounded crest's centre, lie off
// that middle.
func (l threadLayout) markBelowCentre() float64 {
	p := l.profile
	switch {
	case p.CrestRoundRadius > 0 && p.Symmetric():
		return 0
	case p.CrestRoundRadius > 0:
		centre, _, _ := p.CrestArc()
		return p.MiddleShift(p.PitchRadius) - centre[1]
	}
	return p.CrestWidth/2 + p.MiddleShift(p.PitchRadius)
}

// ridgeMiddle is how far along the thread the middle of the first
// ridge is when no phase moves it.
func (l threadLayout) ridgeMiddle() float64 { return l.ridgeBottom + l.baseW/2 }

// phaseRange brings a move of the ridges along a thread of pitch p
// into the room a phased thread leaves them: from a quarter of a pitch
// towards the circle to three quarters away from it.
func phaseRange(move, p float64) float64 {
	return move - p*math.Floor((move+p/4)/p)
}

// ridgeMark is the dimension that holds the first ridge of the ridge
// sketch profile from the circle's centre (threadDrawer.placeRidge).
func (sk *Sketch) ridgeMark(profile core.HGroup, center core.HEntity) *core.Constraint {
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Group == profile && c.Type == core.ConstraintProjPtDistance && c.PtA == center {
			return c
		}
	}
	return nil
}

// threadCandidate is a thread of the sketch, made in it or linked:
// its circle and the point that holds its first ridge, as entities of
// the sketch, its options, and the group it came in by.
type threadCandidate struct {
	circle, mark core.HEntity
	o            ThreadOptions
	group        core.HGroup
}

// threadCandidates are the sketch's threads, its own and those of the
// models it links whose circle and ridge came in with them.
func (sk *Sketch) threadCandidates() []threadCandidate {
	var out []threadCandidate
	for i := range sk.Groups {
		g := &sk.Groups[i]
		if g.Type == GroupLinked {
			for _, lt := range g.LinkThreads {
				circle, ok1 := g.importedEntity(lt.Circle)
				mark, ok2 := g.importedEntity(lt.Mark)
				if ok1 && (ok2 || lt.Cosmetic) {
					out = append(out, threadCandidate{circle, mark, lt.ThreadOptions, g.H})
				}
			}
			continue
		}
		if !g.Thread.IsThread() {
			continue
		}
		if g.Thread.Cosmetic {
			out = append(out, threadCandidate{g.Thread.Circle, core.NoEntity, g.Thread.ThreadOptions, core.HGroup{}})
			continue
		}
		tg, err := sk.threadGroupsOf(g.H)
		circle := sk.Entity(g.Thread.Circle)
		if err != nil || circle == nil {
			continue
		}
		if mark := sk.ridgeMark(tg.profile, circle.Point[0]); mark != nil {
			out = append(out, threadCandidate{g.Thread.Circle, mark.PtB, g.Thread.ThreadOptions, core.HGroup{}})
		}
	}
	return out
}

// importedEntity is the entity a link group made of its model's
// entity in, if it made one.
func (g *Group) importedEntity(in core.HEntity) (core.HEntity, bool) {
	id, ok := g.Remap[core.EntityKey{Input: in, CopyNumber: 0}]
	if !ok {
		return core.NoEntity, false
	}
	return g.H.Entity(int(id.V)), true
}

// threadAtNow is where the thread c lies as the sketch is now
// generated, read off the point that holds its first ridge.
func (sk *Sketch) threadAtNow(c threadCandidate) (threadAt, error) {
	circle, mark := sk.Entity(c.circle), sk.Entity(c.mark)
	if circle == nil || mark == nil || !circle.IsCircle() {
		return threadAt{}, errors.New("the thread's circle or ridge is gone")
	}
	l, err := c.o.layout(circle.CircleGetRadiusNum(sk))
	if err != nil {
		return threadAt{}, err
	}
	at := threadAt{origin: sk.MustEntity(circle.Point[0]).PointGetNum(sk), along: sk.threadAlong(circle, c.o)}
	rel := mark.PointGetNum(sk).Minus(at.origin)
	along := rel.Dot(at.along)
	at.radial = rel.Minus(at.along.ScaledBy(along))
	if at.radial.Magnitude() < core.LengthEps {
		return threadAt{}, errors.New("the thread's ridge is on its axis")
	}
	at.radial = at.radial.WithMagnitude(1)
	at.middle = along + l.markBelowCentre()
	return at, nil
}

// threadAlong is the unit vector a thread on circle runs along.
func (sk *Sketch) threadAlong(circle *core.Entity, o ThreadOptions) geom.Vector {
	n := sk.MustEntity(circle.Normal).NormalGetNum(sk).RotationN()
	if o.Reverse {
		n = n.ScaledBy(-1)
	}
	return n
}

// fitTolerance is how far off a thread's axis, in millimetres, a
// circle may lie and still be on it.
const fitTolerance = 1e-4

// ThreadOnCircle finds the screw thread the circle or arc he carries:
// the one made on it, or else one on its axis of the same part (the
// same link, or the model itself), as a nut's thread made on one rim
// of its hole, or on the hole's face, is found from either rim. It
// reports false for none.
func (sk *Sketch) ThreadOnCircle(he core.HEntity) (ThreadOptions, bool) {
	c, ok := sk.threadOnCircle(he)
	return c.o, ok
}

// threadOnCircle is ThreadOnCircle with where the thread came from.
func (sk *Sketch) threadOnCircle(he core.HEntity) (threadCandidate, bool) {
	e := sk.Entity(he)
	if e == nil || !e.IsCircle() {
		return threadCandidate{}, false
	}
	candidates := sk.threadCandidates()
	for _, c := range candidates {
		if c.circle == he {
			return c, true
		}
	}
	// A circle of a link finds that link's threads, any other this
	// model's own.
	var part core.HGroup
	if g := sk.Group(e.Group); g != nil && g.Type == GroupLinked {
		part = g.H
	}
	center := sk.MustEntity(e.Point[0]).PointGetNum(sk)
	n := sk.MustEntity(e.Normal).NormalGetNum(sk).RotationN()
	for _, c := range candidates {
		tc := sk.Entity(c.circle)
		if tc == nil || c.group != part {
			continue
		}
		o := sk.MustEntity(tc.Point[0]).PointGetNum(sk)
		axis := sk.MustEntity(tc.Normal).NormalGetNum(sk).RotationN()
		rel := center.Minus(o)
		if axis.Cross(n).Magnitude() < 1e-9 && rel.Minus(axis.ScaledBy(rel.Dot(axis))).Magnitude() < fitTolerance {
			return c, true
		}
	}
	return threadCandidate{}, false
}

// ErrNoThreads is ThreadFitMove's error when either circle carries no
// screw thread.
var ErrNoThreads = errors.New("the circles carry no screw threads to fit")

// ThreadFitMove is how far, along the normal of the circle a, the part
// of circle b must move for the threads they carry to fit: b's ridges
// half a pitch from a's, a screw's in a nut's grooves. The move is
// the least, no more than half a pitch either way. The two must be on
// one axis, a screw's and a nut's of one size, pitch, starts and hand;
// it returns ErrNoThreads when either carries none. A cosmetic thread
// has no ridges, so a pair with one fits where it is: the move is 0.
func (sk *Sketch) ThreadFitMove(a, b core.HEntity) (float64, error) {
	ta, okA := sk.threadOnCircle(a)
	tb, okB := sk.threadOnCircle(b)
	if !okA || !okB {
		return 0, ErrNoThreads
	}
	if ta.o.Internal == tb.o.Internal {
		return 0, errors.New("a nut's thread fits on a screw's, and these are both " +
			map[bool]string{true: "nuts'", false: "screws'"}[ta.o.Internal])
	}
	if !sameThread(ta.o.Spec, tb.o.Spec) {
		return 0, fmt.Errorf("a %s thread doesn't fit on a %s one", tb.o.Spec, ta.o.Spec)
	}
	// A cosmetic thread has no ridges to put between the other's.
	if ta.o.Cosmetic || tb.o.Cosmetic {
		return 0, nil
	}
	if ta.o.Spec.Buttress() && sk.loadFacing(ta).Dot(sk.loadFacing(tb)) > 0 {
		return 0, ErrLoadFlanksSameWay
	}
	mate, err := sk.threadAtNow(ta)
	if err != nil {
		return 0, err
	}
	own, err := sk.threadAtNow(tb)
	if err != nil {
		return 0, err
	}
	rel := own.origin.Minus(mate.origin)
	if own.along.Cross(mate.along).Magnitude() > 1e-9 ||
		rel.Minus(mate.along.ScaledBy(rel.Dot(mate.along))).Magnitude() > fitTolerance {
		return 0, errors.New("the threads aren't on one axis")
	}
	s := ta.o.Spec
	move := -fitOffset(own, mate, s)
	move -= s.Pitch * math.Round(move/s.Pitch)
	na := sk.MustEntity(sk.MustEntity(a).Normal).NormalGetNum(sk).RotationN()
	return move * mate.along.Dot(na), nil
}

// ErrLoadFlanksSameWay is ThreadFitMove's error for two buttress
// threads whose load flanks face the same way along their axis, which
// no move fits: one of them must be made with its load flank turned
// (ThreadOptions.LoadFlankBack).
var ErrLoadFlanksSameWay = errors.New("the buttress threads' load flanks face the same way; turn one thread's load flank")

// loadFacing is the way along its axis that a buttress thread's load
// flank faces: along the thread on a screw and back in a nut, the
// other way when it is turned.
func (sk *Sketch) loadFacing(c threadCandidate) geom.Vector {
	v := sk.threadAlong(sk.MustEntity(c.circle), c.o)
	if c.o.Internal != c.o.LoadFlankBack {
		v = v.ScaledBy(-1)
	}
	return v
}

// fitOffset is how far along mate's axis the ridges of own lie past
// where they would lie between mate's ridges, of a thread s, give or
// take whole pitches.
func fitOffset(own, mate threadAt, s thread.Spec) float64 {
	// A ridge of the mate's passes through the point at angle phi about
	// its axis, from its own ridge's plane, at along it; a right-hand
	// thread advances its lead in a turn the way the axis points,
	// whichever way that is.
	n, u := mate.along, mate.radial
	c := own.origin.Plus(own.radial).Plus(own.along.ScaledBy(own.middle))
	rel := c.Minus(mate.origin)
	phi := math.Atan2(rel.Dot(n.Cross(u)), rel.Dot(u))
	hand := 1.0
	if s.LeftHand {
		hand = -1
	}
	off := rel.Dot(n) - mate.middle - hand*phi/(2*math.Pi)*s.Lead() - s.Pitch/2
	return off - s.Pitch*math.Round(off/s.Pitch)
}

// sameThread tells whether two threads mate: the same form, size,
// pitch, starts and hand.
func sameThread(a, b thread.Spec) bool {
	return a.Form == b.Form && math.Abs(a.Diameter-b.Diameter) < 1e-9 && math.Abs(a.Pitch-b.Pitch) < 1e-9 &&
		a.NumStarts() == b.NumStarts() && a.LeftHand == b.LeftHand
}
