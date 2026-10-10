package api

import (
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
}

// ThreadNearest is the designation of the standard thread whose major
// diameter is nearest d mm, as the screw thread screen first offers.
func ThreadNearest(d float64) string { return thread.Nearest(d).String() }
