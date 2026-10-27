package api

import (
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/thread"
)

// ThreadOnFace makes a screw thread on a cylindrical face of the solid
// so far, as Solid.FaceAt gives it, as Features > Screw Thread does
// with a face selected: the face says where the thread is and which
// way it runs, so no circle need be drawn and Reverse guessed. The
// thread starts at the end of the face that is the part's end (a
// shaft's tip, the mouth of a blind hole), or with both ends free at
// its upper end; Reverse starts it at the other end. It runs along the
// face into the part. A shaft's face carries a screw's thread and a
// hole's a nut's; Internal set on a shaft's face is an error. A
// Designation of "" takes the standard thread nearest the shaft, or the
// one a hole is drawn or drilled for (its nominal or tap drill size);
// a Length of 0 one and a half diameters, or the face's length where
// that is less. FullLength runs the thread the face's whole length,
// through where the face reaches the part's far end, and follows the
// face's length when the part changes afterwards.
//
// The thread is drawn on a construction circle in a datum at the face's
// end, which follows the face when the part changes upstream; the
// groups are those Features.Thread makes, after that datum. It returns
// the helix group (AsThread); the thread's last group becomes active.
func (m FeaturesMenu) ThreadOnFace(face SolidFace, o ThreadOptions) Group {
	return m.threadAt(func(sk *sketch.Sketch) (sketch.ThreadPlace, error) {
		return sk.ThreadPlaceOnFace(face.id, o.Reverse)
	}, o)
}

// ThreadOnEdge is ThreadOnFace from the circular edge at one end of a
// cylindrical face, as Solid.EdgeAt gives it: the thread starts at
// that end, so Reverse is not needed.
func (m FeaturesMenu) ThreadOnEdge(edge SolidEdge, o ThreadOptions) Group {
	return m.threadAt(func(sk *sketch.Sketch) (sketch.ThreadPlace, error) {
		return sk.ThreadPlaceAtEdge(edge.name)
	}, o)
}

// threadAt makes the thread o at the place that place finds in the
// solid so far.
func (m FeaturesMenu) threadAt(place func(*sketch.Sketch) (sketch.ThreadPlace, error), o ThreadOptions) (g Group) {
	d := m.d
	d.change(func(sk *sketch.Sketch) {
		if d.h.Sketch().RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		p, err := place(sk)
		if err != nil {
			fail("%v", err)
		}
		if o.Internal && !p.At.Hole {
			fail("the face is a shaft's, which carries an external thread; an internal thread is made on a hole's face")
		}
		res, err := sk.AddThreadOnFace(p, o.placeOptions(sk, p))
		if err != nil {
			fail("%v", err)
		}
		hs, err := sk.ThreadGroups(res.Group)
		if err != nil {
			fail("%v", err)
		}
		d.h.Activate(hs[len(hs)-1])
		g = Group{d, res.Group}
	})
	return g
}

// placeOptions are the options o gives a thread on a face at p: the
// standard thread the face is drawn for when o names none, and for no
// length one and a half diameters or the face's length where that is
// less.
func (o ThreadOptions) placeOptions(sk *sketch.Sketch, p sketch.ThreadPlace) sketch.ThreadOptions {
	if o.Designation == "" {
		if p.At.Hole {
			o.Designation = thread.NearestForHole(p.Diameter()).String()
		} else {
			o.Designation = thread.Nearest(p.Diameter()).String()
		}
	}
	defaultLength := o.Length == 0 && o.LengthExpr == "" && !o.FullLength
	opts := o.sketchOptions(sk, p.Diameter())
	if defaultLength && opts.Length >= p.At.Length {
		opts.FullLength = true
	}
	return opts
}

// ThreadNearestForHole is the designation of the standard thread a hole
// of diameter d mm is likely drawn for, at its nominal or its tap drill
// size, as the screw thread screen offers for a hole's face.
func ThreadNearestForHole(d float64) string { return thread.NearestForHole(d).String() }
