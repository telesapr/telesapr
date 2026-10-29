package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// ThreadOptions are what Features > Screw Thread asks for. The zero
// value is what the screen starts with: the standard thread nearest
// the circle's diameter, external, one and a half diameters long,
// ending at a shoulder.
type ThreadOptions struct {
	// Designation is the thread, as "M8", "M8x1 LH", "1/4-20 UNC",
	// "Tr20x4", "1/4 BSW", "G1/2", "1/2-10 ACME", or the buttress
	// threads "S20x4" (DIN 513) and "1-12 BUTT" (ASME B1.9); "" takes the
	// standard one nearest the circle. ThreadSizes and ThreadPitches
	// list the standard ones.
	Designation string
	// LeftHand makes the thread left-hand, as -LH at the end of the
	// designation does; either is enough. Options gives the
	// designation without the -LH and says the hand here.
	LeftHand bool
	// Internal makes a nut's thread: the circle is bored out and the
	// ridge put inside. Otherwise the part is turned down and the
	// ridge put around it.
	Internal bool
	// Length is how far the thread reaches from the circle's plane, in
	// mm; 0 is one and a half diameters, rounded up. It is no more
	// than the part reaches along it.
	Length float64
	// Through carries the turning or boring past Length, for a part
	// that ends there; otherwise it stops at a shoulder.
	Through bool
	// Reverse runs the thread against the circle's normal: into the
	// part from a circle at its top, which points out of it.
	Reverse bool
	// Clearance moves the thread away from its mate, in mm, when it
	// has no Class. Options gives the clearance the class sets.
	Clearance float64
	// Class is the tolerance class, which sets the clearance to the
	// middle of its zone: "6g", "6h", "6f", "6e", "4h" or "8g" for an
	// ISO metric screw, "6H", "4H", "5H", "7H" or "6G" for its nut;
	// "1A", "2A", "3A" and "1B", "2B", "3B" for unified threads; "7e",
	// "8e" and "7H", "8H" for trapezoidal ones. "" is Clearance as
	// given. ThreadClasses lists a thread's. A Cosmetic thread keeps
	// its class as a record, its part left at the nominal size.
	Class string
	// LengthExpr is an expression of the parameters for the length, in
	// the document's unit, as "len" or "2*d"; when it is not "", it
	// drives the length in place of Length.
	LengthExpr string
	// Crest is the shape of the ridge's top: "standard" (or ""), the
	// flat crest the standards give; "truncated", cut lower by a
	// quarter of the ridge's height; or "rounded", an arc tangent to
	// both flanks, which prints better (on a buttress thread its top
	// lies off the crest's middle, towards the steeper flank).
	Crest string
	// RunOut is how far, in mm, the thread fades out before its
	// shoulder: the turned or bored surface tapers from the root to
	// about the crest, as a die leaves a thread. 0 is none; a Through
	// thread has none.
	RunOut float64
	// LevelStarts begins every start of a thread of several at the
	// circle's plane, each a ridge sketch and a helix of its own;
	// otherwise one helix sweeps the ridges stacked a pitch apart.
	LevelStarts bool
	// LoadFlankBack turns a buttress thread's ridge about: its load
	// flank, nearly square to the axis, faces back, towards where the
	// thread starts, on a screw, and away from it in a nut, rather than
	// the other way. A screw and a nut mate when their load flanks face
	// each other, which depends on which way each thread runs:
	// Assembly.Screw refuses two that face the same way. Other threads'
	// ridges are symmetric and stay as they are. A designation that says
	// which way the load goes sets it: "PUSH-BUTT" leaves the load flank
	// ahead, "PULL-BUTT" turns it back.
	LoadFlankBack bool
	// RoundRoot rounds the roots of an ASME B1.9 buttress thread (BUTT)
	// as the standard does, by an arc of 0.07141 pitches tangent to both
	// flanks; otherwise they are flat. The arcs are small: a chord
	// tolerance coarser than a fifth of their radius is refused, saying
	// the tolerance to set (Edit.SetChordTolerance). Other threads'
	// roots stay flat.
	RoundRoot bool
	// Phase turns the thread about its axis, in degrees: its ridges
	// move along it, away from the circle, by that part of the lead,
	// and are a pitch shorter, for the room; with a run-out, their end
	// is kept off the circle where the root turns into the run-out's
	// cone where the phase has moved it. A Cosmetic thread, with no
	// ridges, keeps it for when it is modelled. Assembly.Screw fits a screw and a nut in an
	// assembly without it.
	Phase float64
	// FullLength, for a thread made on a face (ThreadOnFace,
	// ThreadOnEdge), runs it the face's whole length, through where the
	// face reaches the part's end, in place of Length; the face is
	// measured again at every regeneration, and the thread's length
	// follows it. A face come to reach the part's end or to stop short
	// of it, or too short for the thread, is reported on the thread's
	// group (Group.Problem) and taken up when the thread is edited.
	FullLength bool
	// Cosmetic records the thread on the part without modelling its
	// turns, as drawings show a thread anyway: a screw's rod is left at
	// its major diameter (turned down to it when thicker) and a nut's
	// part bored to its minor diameter, unless a hole that wide is there
	// already. Its one group, a sketch, keeps it (two where the part is
	// cut, the sketch and a lathe); drawings show it by the convention.
	// Crest and LevelStarts shape only a modelled thread.
	Cosmetic bool
	// Chamfer cuts the thread's free end, the circle's, by a cone at
	// 45° to the axis from a little inside the minor diameter out past
	// the crest: a screw's end chamfered, a nut's mouth countersunk, so
	// that the thread starts clean. A Through thread is chamfered at
	// both ends. A Cosmetic thread's part is cut so, and a "rounded"
	// Crest as any other.
	Chamfer bool
}

// sketchOptions are the options o gives a thread on a circle of the
// given diameter: the nearest standard thread for no designation, and
// one and a half diameters long for no length.
func (o ThreadOptions) sketchOptions(sk *sketch.Sketch, diameter float64) sketch.ThreadOptions {
	var opts sketch.ThreadOptions
	if o.Designation == "" {
		opts.Spec = thread.Nearest(diameter)
	} else {
		spec, err := thread.Parse(o.Designation)
		if err != nil {
			fail("%v", err)
		}
		opts.Spec = spec
	}
	opts.Spec.LeftHand = opts.Spec.LeftHand || o.LeftHand
	crest, err := thread.ParseCrest(o.Crest)
	if err != nil {
		fail("%v", err)
	}
	opts.Crest = crest
	opts.Internal, opts.Through, opts.Reverse = o.Internal, o.Through, o.Reverse
	opts.Length, opts.Clearance = o.Length, o.Clearance
	opts.RunOut, opts.LevelStarts, opts.Cosmetic = o.RunOut, o.LevelStarts, o.Cosmetic
	opts.FullLength, opts.Chamfer = o.FullLength, o.Chamfer
	opts.Class, opts.Phase, opts.LoadFlankBack, opts.RoundRoot = o.Class, o.Phase, o.LoadFlankBack, o.RoundRoot
	if o.LengthExpr != "" {
		v, expr, err := sk.ThreadLengthText(o.LengthExpr)
		if err != nil {
			fail("thread length: %v", err)
		}
		opts.Length, opts.LengthExpr = v, expr
	}
	if opts.Length == 0 {
		opts.Length = math.Ceil(1.5 * opts.Spec.Diameter)
	}
	return opts
}

// threadOptionsOf is what a script reads back of a thread's options.
func threadOptionsOf(o sketch.ThreadOptions) ThreadOptions {
	right := o.Spec
	right.LeftHand = false
	clearance, err := o.FitClearance()
	if err != nil {
		clearance = o.Clearance
	}
	return ThreadOptions{
		Designation: right.String(), LeftHand: o.Spec.LeftHand, Internal: o.Internal, Length: o.Length, Through: o.Through,
		Reverse: o.Reverse, Clearance: clearance, Class: o.Class, LengthExpr: o.LengthExpr, Crest: o.Crest.String(),
		RunOut: o.RunOut, LevelStarts: o.LevelStarts, FullLength: o.FullLength, Cosmetic: o.Cosmetic, Phase: o.Phase,
		Chamfer: o.Chamfer, LoadFlankBack: o.LoadFlankBack, RoundRoot: o.RoundRoot,
	}
}

// ThreadGroup is the view of a screw thread, by the group that keeps
// it: the helix group, the fourth of the groups that Features.Thread
// makes, or a cosmetic thread's sketch. It is what the thread was made
// with, to read and to change as one.
type ThreadGroup struct{ g Group }

// AsThread is the view of the group that keeps a thread: its helix
// group, or a cosmetic thread's sketch.
func (g Group) AsThread() ThreadGroup {
	g.d.read(func(sk *sketch.Sketch) {
		if group := g.get(sk); !group.Thread.IsThread() {
			fail("AsThread is for the group that keeps a screw thread, and %s is a %s that keeps none",
				groupDescription(group), kindName(group.Type))
		}
	})
	return ThreadGroup{g}
}

// IsThread reports whether the group keeps a screw thread (a helix
// group, or a cosmetic thread's sketch), which AsThread edits.
func (g Group) IsThread() (is bool) {
	g.d.read(func(sk *sketch.Sketch) { is = g.get(sk).Thread.IsThread() })
	return is
}

// Options are what the thread is made with now: its designation, its
// length in mm (as its expression gives it, when it has one), and the
// rest as Features.Thread takes them.
func (v ThreadGroup) Options() (o ThreadOptions) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		opts, err := sk.ThreadOf(g.get(sk).H)
		if err != nil {
			fail("%v", err)
		}
		o = threadOptionsOf(opts)
	})
	return o
}

// Groups are the thread's groups: the sketch and the lathe that turn
// the part down or bore it, the sketch of the ridge, and the helix that
// sweeps it; then, with level starts, a ridge sketch and a helix for
// each further start.
func (v ThreadGroup) Groups() (groups []Group) {
	g := v.g
	g.d.read(func(sk *sketch.Sketch) {
		hs, err := sk.ThreadGroups(g.get(sk).H)
		if err != nil {
			fail("%v", err)
		}
		for _, h := range hs {
			groups = append(groups, Group{g.d, h})
		}
	})
	return groups
}

// Finished is the thread's last group, which holds the part with the
// whole thread on it: the helix group itself for one helix, and the
// helix of the last start with level starts, whose solid the helix
// group's own is only a part of; a cosmetic thread's lathe, or its
// sketch where the part is not cut.
func (v ThreadGroup) Finished() Group {
	groups := v.Groups()
	return groups[len(groups)-1]
}

// Edit makes the thread again with the options o, in one step, as the
// thread's screen in the window does: its groups are drawn again,
// keeping their handles (those of further level starts only while
// there are as many). A Designation of "" keeps the thread's, hand
// and all, and a Length of 0 (with no LengthExpr) its length. Options gives what o
// starts from:
//
//	o := t.Options()
//	o.Designation, o.Crest = "M8x1", "rounded"
//	t.Edit(o)
//
// Changing Cosmetic keeps the thread's first sketch and lathe, and the
// group that keeps the thread is then another, the view of which Edit
// returns: t = t.Edit(o).
func (v ThreadGroup) Edit(o ThreadOptions) (edited ThreadGroup) {
	g := v.g
	edited = v
	g.d.change(func(sk *sketch.Sketch) {
		group := g.get(sk)
		was, err := sk.ThreadOf(group.H)
		if err != nil {
			fail("%v", err)
		}
		if o.Designation == "" {
			o.Designation = was.Spec.String()
		}
		if o.Length == 0 && o.LengthExpr == "" {
			o.Length, o.LengthExpr = was.Length, was.LengthExpr
		}
		opts := o.sketchOptions(sk, was.Spec.Diameter)
		res, err := sk.EditThread(group.H, opts)
		if err != nil {
			fail("%v", err)
		}
		edited = ThreadGroup{Group{g.d, res.Group}}
	})
	return edited
}

// ThreadNearest is the designation of the standard thread whose major
// diameter is nearest d mm, as the screw thread screen first offers.
func ThreadNearest(d float64) string { return thread.Nearest(d).String() }

// ThreadSeries are the names of the series of standard threads, as
// the screw thread screen's [choose] lists them: "metric", "unified",
// "trapezoidal", "Whitworth", "pipe", "ACME", "buttress" (DIN 513's S
// threads) and "BUTT" (ASME B1.9's buttress inch threads).
func ThreadSeries() []string {
	var names []string
	for _, sr := range thread.AllSeries() {
		names = append(names, sr.String())
	}
	return names
}

// ThreadSizes are the designations of a series' standard sizes, each
// at its usual pitch, smallest first: ThreadSizes("metric") begins
// "M1", "M1.2". The series is named as ThreadSeries gives it, in any
// letter case.
func ThreadSizes(series string) []string {
	sr, err := thread.ParseSeries(series)
	if err != nil {
		fail("%v", err)
	}
	var out []string
	for _, s := range sr.Sizes() {
		out = append(out, s.String())
	}
	return out
}

// ThreadPitches are the designations of the standard pitches of the
// size of a designation, coarsest first, with its starts and hand:
// ThreadPitches("M8") is "M8", "M8x1", "M8x0.75". A size no series
// has has none.
func ThreadPitches(designation string) []string {
	s, err := thread.Parse(designation)
	if err != nil {
		fail("%v", err)
	}
	var out []string
	for _, p := range thread.Pitches(s) {
		out = append(out, p.String())
	}
	return out
}

// ThreadDesignation is a designation as the thread is written, which
// Options gives back: ThreadDesignation("m8x1.25") is "M8", the coarse
// pitch being M8's own. It fails for one that is not a thread.
func ThreadDesignation(text string) string {
	s, err := thread.Parse(text)
	if err != nil {
		fail("%v", err)
	}
	return s.String()
}

// ThreadClasses are the tolerance classes of a thread of the
// designation, a screw's or, internal, a nut's, the default first:
// ThreadClasses("M8", false) begins "6g". Whitworth and ACME threads
// have none, and take a Clearance.
func ThreadClasses(designation string, internal bool) []string {
	s, err := thread.Parse(designation)
	if err != nil {
		fail("%v", err)
	}
	def := thread.DefaultClass(s, internal)
	out := []string{}
	if def != "" {
		out = append(out, def)
	}
	for _, c := range thread.Classes(s, internal) {
		if c != def {
			out = append(out, c)
		}
	}
	return out
}
