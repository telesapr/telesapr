package thread

import (
	"fmt"
	"math"
	"slices"
	"strings"
)

// A tolerance class says how far a thread's pitch diameter lies from
// the basic one: a fundamental deviation (the class's letter, or 0 for
// the unified classes 3A and every B), and a tolerance (its grade).
// A thread is modelled at one size, so a class is turned into the
// clearance of Spec.Profile: half the deviation plus half the
// tolerance on the diameter, the middle of the class's zone, taken as
// a radius. A 6g screw then sits a little inside its basic profile and
// a 6H nut a little outside it, and the two fit as their parts do on
// average.
//
// The numbers come from the standards' formulas rather than their
// tables, which were not to hand: for ISO metric (ISO 965-1) and
// trapezoidal (ISO 2903) threads the deviations es = -(15 + 11P) µm
// for g, -(30 + 11P) for f, -(50 + 11P) for e and +(15 + 11P) for G,
// and the pitch diameter tolerance Td2 = 90 P^0.4 d^0.1 µm for grade
// 6, scaled by grade, 1.32 times that for a nut; for unified threads
// (ASME B1.1) Td2 = 0.0015 d^(1/3) + 0.0015 (9P)^(1/2) + 0.015 P^(2/3)
// inch for class 2A, 1.5 and 0.75 times that for 1A and 3A, 1.95, 1.3
// and 0.975 times it for 1B, 2B and 3B, and an allowance of 0.3 times
// it for 1A and 2A. The standards take d as the geometric mean of the
// diameter range a size falls in; here it is the size itself. For M10
// the formulas give 6g's es and Td2 as 32 and 133 µm, and 6H's TD2 as
// 175 µm, where ISO 965-2's table has 32, 132 and 180; for 1/4-20 UNC
// 2A, Td2 as 0.0040 inch where ASME B1.1's has 0.0037.

// classGrades scales the grade 6 tolerance to the other grades.
var classGrades = map[byte]float64{3: 0.5, 4: 0.63, 5: 0.8, 6: 1, 7: 1.25, 8: 1.6, 9: 2}

// metricClasses are the ISO 965-1 classes offered, external then
// internal: the usual medium and fine ones.
var (
	metricExternal      = []string{"4h", "6e", "6f", "6g", "6h", "8g"}
	metricInternal      = []string{"4H", "5H", "6G", "6H", "7H"}
	unifiedExternal     = []string{"1A", "2A", "3A"}
	unifiedInternal     = []string{"1B", "2B", "3B"}
	trapezoidalExternal = []string{"7e", "8e"}
	trapezoidalInternal = []string{"7H", "8H"}
)

// Classes are the tolerance classes offered for a thread of the form
// of s, of a screw or of a nut; none for Whitworth and ACME threads,
// whose fit is a clearance typed in.
func Classes(s Spec, internal bool) []string {
	var ext, in []string
	switch s.Form {
	case FormMetric:
		ext, in = metricExternal, metricInternal
	case FormUnified:
		ext, in = unifiedExternal, unifiedInternal
	case FormTrapezoidal:
		ext, in = trapezoidalExternal, trapezoidalInternal
	default:
		return nil
	}
	if internal {
		return slices.Clone(in)
	}
	return slices.Clone(ext)
}

// DefaultClass is the class a thread of the form of s has when none
// is said: 6g and 6H, 2A and 2B, 7e and 7H; "" for a form with none.
func DefaultClass(s Spec, internal bool) string {
	switch s.Form {
	case FormMetric:
		return pick(internal, "6H", "6g")
	case FormUnified:
		return pick(internal, "2B", "2A")
	case FormTrapezoidal:
		return pick(internal, "7H", "7e")
	}
	return ""
}

func pick(internal bool, in, ext string) string {
	if internal {
		return in
	}
	return ext
}

// ClassClearance is the clearance, in millimetres, that puts a thread
// of s in the middle of the tolerance class: what Spec.Profile takes.
// It refuses a class the form doesn't have, or one of the other side
// (a nut's 6H on a screw).
func ClassClearance(s Spec, internal bool, class string) (float64, error) {
	if !slices.Contains(Classes(s, internal), class) {
		return 0, classError(s, internal, class)
	}
	if s.Form == FormUnified {
		return unifiedClearance(s, class), nil
	}
	return isoClearance(s, internal, class), nil
}

// classError says why class is not one of the thread's.
func classError(s Spec, internal bool, class string) error {
	offered := Classes(s, internal)
	if len(offered) == 0 {
		return fmt.Errorf("a %s thread has no tolerance classes; give its clearance", formName(s.Form))
	}
	if slices.Contains(Classes(s, !internal), class) {
		return fmt.Errorf("%s is a class of %s thread; this one takes %s", class,
			pick(!internal, "an internal", "an external"), strings.Join(offered, ", "))
	}
	return fmt.Errorf("unknown tolerance class %q; this thread takes %s", class, strings.Join(offered, ", "))
}

// formName is how an error names a form.
func formName(f Form) string {
	switch f {
	case FormWhitworth:
		return "Whitworth"
	case FormACME:
		return "ACME"
	case FormUnified:
		return "unified"
	case FormTrapezoidal:
		return "trapezoidal"
	}
	return "metric"
}

// isoClearance is ClassClearance for ISO 965-1 and ISO 2903 classes:
// a grade digit and a position letter.
func isoClearance(s Spec, internal bool, class string) float64 {
	grade, position := class[0]-'0', class[1]
	p := s.Pitch
	td2 := 90 * math.Pow(p, 0.4) * math.Pow(s.Diameter, 0.1) * classGrades[grade]
	if internal {
		td2 *= 1.32
	}
	var deviation float64
	switch position {
	case 'e':
		deviation = 50 + 11*p
	case 'f':
		deviation = 30 + 11*p
	case 'g', 'G':
		deviation = 15 + 11*p
	}
	return (deviation + td2/2) / 2 / 1000
}

// unifiedScales scale class 2A's pitch diameter tolerance to each
// unified class's.
var unifiedScales = map[string]float64{"1A": 1.5, "2A": 1, "3A": 0.75, "1B": 1.95, "2B": 1.3, "3B": 0.975}

// unifiedClearance is ClassClearance for ASME B1.1's classes, whose
// formulas are in inches; the class says the side.
func unifiedClearance(s Spec, class string) float64 {
	d, p := s.Diameter/mmPerInch, s.Pitch/mmPerInch
	td2 := 0.0015*math.Cbrt(d) + 0.0015*math.Sqrt(9*p) + 0.015*math.Pow(p, 2.0/3)
	allowance := 0.0
	if class == "1A" || class == "2A" {
		allowance = 0.3 * td2
	}
	return (allowance + unifiedScales[class]*td2/2) / 2 * mmPerInch
}
