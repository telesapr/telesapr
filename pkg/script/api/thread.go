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
	// Designation is the thread, as "M8", "M8x1 LH", "1/4-20 UNC" or
	// "Tr20x4"; "" takes the standard one nearest the circle.
	Designation string
	// Internal makes a nut's thread: the circle is bored out and the
	// ridge put inside. Otherwise the part is turned down and the
	// ridge put around it.
	Internal bool
	// Length is how far the thread reaches from the circle's plane, in
	// mm; 0 is one and a half diameters, rounded up.
	Length float64
	// Through carries the turning or boring past Length, for a part
	// that ends there; otherwise it stops at a shoulder.
	Through bool
	// Reverse runs the thread against the circle's normal.
	Reverse bool
	// Clearance moves the thread away from its mate, in mm.
	Clearance float64
	// LengthExpr is an expression of the parameters for the length, in
	// the document's unit, as "len" or "2*d"; when it is not "", it
	// drives the length in place of Length.
	LengthExpr string
	// Crest is the shape of the ridge's top: "standard" (or ""), the
	// flat crest the standards give; "truncated", cut lower by a
	// quarter of the ridge's height; or "rounded", an arc tangent to
	// both flanks, which prints better.
	Crest string
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
	crest, err := thread.ParseCrest(o.Crest)
	if err != nil {
		fail("%v", err)
	}
	opts.Crest = crest
	opts.Internal, opts.Through, opts.Reverse = o.Internal, o.Through, o.Reverse
	opts.Length, opts.Clearance = o.Length, o.Clearance
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
	return ThreadOptions{
		Designation: o.Spec.String(), Internal: o.Internal, Length: o.Length, Through: o.Through,
		Reverse: o.Reverse, Clearance: o.Clearance, LengthExpr: o.LengthExpr, Crest: o.Crest.String(),
	}
}

// ThreadGroup is the view of a screw thread, by its helix group, the
// last of the four that Features.Thread makes: what it was made with,
// to read and to change as one.
type ThreadGroup struct{ g Group }

// AsThread is the view of a thread's helix group.
func (g Group) AsThread() ThreadGroup {
	g.d.read(func(sk *sketch.Sketch) {
		if group := g.get(sk); !group.Thread.IsThread() {
			fail("AsThread is for a screw thread's helix group, and %s is a %s", groupDescription(group), kindName(group.Type))
		}
	})
	return ThreadGroup{g}
}

// IsThread reports whether the group is a screw thread's helix group,
// which AsThread edits.
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

// Groups are the thread's four groups: the sketch and the lathe that
// turn the part down or bore it, the sketch of the ridge, and the
// helix that sweeps it.
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

// Edit makes the thread again with the options o, in one step, as the
// thread's screen in the window does: its four groups are drawn again,
// keeping their handles. A Designation of "" keeps the thread's, and a
// Length of 0 (with no LengthExpr) its length. Options gives what o
// starts from:
//
//	o := t.Options()
//	o.Designation, o.Crest = "M8x1", "rounded"
//	t.Edit(o)
func (v ThreadGroup) Edit(o ThreadOptions) {
	g := v.g
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
		if _, err := sk.EditThread(group.H, opts); err != nil {
			fail("%v", err)
		}
	})
}

// ThreadNearest is the designation of the standard thread whose major
// diameter is nearest d mm, as the screw thread screen first offers.
func ThreadNearest(d float64) string { return thread.Nearest(d).String() }
