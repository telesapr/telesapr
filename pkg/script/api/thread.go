package api

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/sketch"
	"github.com/spacesolve/spacesolve/pkg/thread"
)

// ThreadOptions are what New Group > Screw Thread asks for. The zero
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
}

// ThreadNearest is the designation of the standard thread whose major
// diameter is nearest d mm, as the screw thread screen first offers.
func ThreadNearest(d float64) string { return thread.Nearest(d).String() }

// Thread makes a screw thread on a circle or an arc, about its axis
// and from its plane, as New Group > Screw Thread does: four groups,
// a sketch and a lathe that turn the part down (or bore it), and a
// ridge sketch and a helix that sweeps it. It returns the helix group,
// which becomes active; its Check says whether the solid came out
// sound at the chord tolerance of the moment.
func (d *Doc) Thread(circle Entity, o ThreadOptions) (g Group) {
	d.change(func(sk *sketch.Sketch) {
		if d.h.Sketch().RegenerationPending() {
			d.h.Regenerate()
			sk = d.h.Sketch()
		}
		c := circle.get(sk)
		if !c.IsCircle() {
			fail("a thread is made on a circle or an arc, and %s isn't one", circle.describe(sk))
		}
		var opts sketch.ThreadOptions
		diameter := 2 * c.CircleGetRadiusNum(sk)
		if o.Designation == "" {
			opts.Spec = thread.Nearest(diameter)
		} else {
			spec, err := thread.Parse(o.Designation)
			if err != nil {
				fail("%v", err)
			}
			opts.Spec = spec
		}
		opts.Internal, opts.Through, opts.Reverse = o.Internal, o.Through, o.Reverse
		opts.Length, opts.Clearance = o.Length, o.Clearance
		if opts.Length == 0 {
			opts.Length = math.Ceil(1.5 * opts.Spec.Diameter)
		}
		res, err := sk.AddThread(circle.H, opts)
		if err != nil {
			fail("%v", err)
		}
		d.h.Activate(res.Group)
		g = Group{d, res.Group}
	})
	return g
}
