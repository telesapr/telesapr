// Package thread has the screw thread standards: the sizes and pitches
// of the ISO metric, unified inch and ISO metric trapezoidal series,
// their designations, and the profile of a thread in a plane through
// its axis. It is not a port: C++ SolveSpace has no threads.
package thread

import (
	"errors"
	"math"
)

// Form is the shape of a thread's profile.
type Form int

const (
	// FormMetric is the ISO 68-1 basic profile, with flanks at 60
	// degrees, of the M threads.
	FormMetric Form = iota
	// FormUnified is the same profile for the inch series (UNC, UNF),
	// which differ in how they are designated.
	FormUnified
	// FormTrapezoidal is the ISO 2904 profile, with flanks at 30
	// degrees, of the Tr threads.
	FormTrapezoidal
)

// Spec is a thread: what its designation says.
type Spec struct {
	Form Form
	// Diameter is the nominal diameter, the major diameter of the
	// external thread, in millimeters.
	Diameter float64
	// Pitch is the distance between neighbouring ridges, in millimeters.
	Pitch float64
	// Starts is the number of ridges; 0 stands for 1.
	Starts   int
	LeftHand bool
}

// NumStarts is the number of ridges.
func (s Spec) NumStarts() int {
	return max(s.Starts, 1)
}

// Lead is how far the thread advances in a turn.
func (s Spec) Lead() float64 {
	return s.Pitch * float64(s.NumStarts())
}

// fundamental is the height H of the sharp triangle that the 60 degree
// profile truncates.
func (s Spec) fundamental() float64 {
	return math.Sqrt(3) / 2 * s.Pitch
}

// crestClearance is ISO 2904's a_c: the gap between the crest of a
// trapezoidal thread and the root of its mate.
func (s Spec) crestClearance() float64 {
	switch {
	case s.Pitch <= 1.5:
		return 0.15
	case s.Pitch <= 5:
		return 0.25
	case s.Pitch <= 12:
		return 0.5
	}
	return 1
}

// PitchDiameter is the diameter where the ridge and the groove are
// equally wide (d2, D2).
func (s Spec) PitchDiameter() float64 {
	if s.Form == FormTrapezoidal {
		return s.Diameter - 0.5*s.Pitch
	}
	return s.Diameter - 2*3.0/8*s.fundamental()
}

// MinorDiameter is the minor diameter of the internal thread (D1): of
// the hole to make before tapping it. For the 60 degree profile it is
// also the basic minor diameter of the external thread (d1).
func (s Spec) MinorDiameter() float64 {
	if s.Form == FormTrapezoidal {
		return s.Diameter - s.Pitch
	}
	return s.Diameter - 2*5.0/8*s.fundamental()
}

// ExternalMinorDiameter is the diameter at the root of the external
// thread: d1 for the 60 degree profile, d3 for the trapezoidal one.
func (s Spec) ExternalMinorDiameter() float64 {
	if s.Form == FormTrapezoidal {
		return s.Diameter - s.Pitch - 2*s.crestClearance()
	}
	return s.MinorDiameter()
}

// InternalMajorDiameter is the diameter at the root of the internal
// thread: D for the 60 degree profile, D4 for the trapezoidal one.
func (s Spec) InternalMajorDiameter() float64 {
	if s.Form == FormTrapezoidal {
		return s.Diameter + 2*s.crestClearance()
	}
	return s.Diameter
}

// FlankAngle is the angle between the two flanks of a ridge, in
// degrees.
func (s Spec) FlankAngle() float64 {
	if s.Form == FormTrapezoidal {
		return 30
	}
	return 60
}

// Validate reports what makes the thread impossible.
func (s Spec) Validate() error {
	switch {
	case !(s.Diameter > 0) || math.IsInf(s.Diameter, 0):
		return errors.New("thread diameter must be positive")
	case !(s.Pitch > 0) || math.IsInf(s.Pitch, 0):
		return errors.New("thread pitch must be positive")
	case s.Starts < 0:
		return errors.New("thread must have at least one start")
	case s.ExternalMinorDiameter() <= 0:
		return errors.New("thread pitch is too coarse for its diameter")
	}
	return nil
}

// Profile is one ridge of a thread, cut by a plane through the axis: a
// trapezoid, symmetric about a line perpendicular to the axis, whose
// parallel sides lie along the axis at the crest and at the root.
type Profile struct {
	// CrestRadius and RootRadius are the distances from the axis of the
	// ridge's top and of its foot. The crest is the nearer of the two
	// for an internal thread.
	CrestRadius, RootRadius float64
	// CrestWidth and RootWidth are the lengths along the axis of the
	// ridge at those radii.
	CrestWidth, RootWidth float64
	// FlankAngle is the angle between the flanks, in degrees.
	FlankAngle float64
}

// Profile returns the ridge of the external or the internal thread.
// The clearance moves the whole profile away from the mating thread,
// inwards for an external thread and outwards for an internal one; with
// none, the 60 degree profiles of a screw and its nut touch all over.
func (s Spec) Profile(internal bool, clearance float64) Profile {
	pitchR := s.PitchDiameter() / 2
	p := Profile{FlankAngle: s.FlankAngle()}
	if internal {
		p.CrestRadius = s.MinorDiameter() / 2
		p.RootRadius = s.InternalMajorDiameter() / 2
	} else {
		p.CrestRadius = s.Diameter / 2
		p.RootRadius = s.ExternalMinorDiameter() / 2
	}
	// The ridge is half a pitch wide at the pitch diameter.
	slope := 2 * math.Tan(p.FlankAngle/2*math.Pi/180)
	p.CrestWidth = s.Pitch/2 - slope*math.Abs(p.CrestRadius-pitchR)
	p.RootWidth = s.Pitch/2 + slope*math.Abs(p.RootRadius-pitchR)
	if internal {
		p.CrestRadius += clearance
		p.RootRadius += clearance
	} else {
		p.CrestRadius -= clearance
		p.RootRadius -= clearance
	}
	return p
}

// Height is the distance between the crest and the root.
func (p Profile) Height() float64 {
	return math.Abs(p.CrestRadius - p.RootRadius)
}

// WidthAt is the length along the axis of the ridge at radius r, which
// may lie beyond the root, where the flanks are carried on.
func (p Profile) WidthAt(r float64) float64 {
	slope := 2 * math.Tan(p.FlankAngle/2*math.Pi/180)
	if p.RootRadius < p.CrestRadius {
		return p.CrestWidth + slope*(p.CrestRadius-r)
	}
	return p.CrestWidth + slope*(r-p.CrestRadius)
}
