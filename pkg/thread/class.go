package thread

import (
	"fmt"
	"slices"
	"strings"
)

// A tolerance class says how far a thread's pitch diameter lies from
// the basic one: a fundamental deviation (the class's letter, the
// allowance of the inch classes, or 0 for the unified classes 3A and
// every nut but ISO 965-1's G), and a tolerance (its grade, or the inch
// class's number). A thread is modelled at one size, so a class is
// turned into the clearance of Spec.Profile: half the deviation plus
// half the tolerance on the diameter, the middle of the class's zone,
// taken as a radius. A 6g screw then sits a little inside its basic
// profile and a 6H nut a little outside it, and the two fit as their
// parts do on average.
//
// The deviations and tolerances are the standards' tables: ISO 965-1's
// for metric threads and ISO 2903's for trapezoidal ones, with the
// formulae the tables come from for what they lack (class_iso.go);
// ASME B1.1's formulae, with the length of engagement and rounding its
// tables use, for unified threads (class_unified.go); ASME B1.5's for
// ACME threads (class_acme.go); and ASME B1.9's and DIN 513-3's for
// the inch and metric buttress threads (class_buttress.go).

// metricClasses are the ISO 965-1 classes offered, external then
// internal: the usual medium and fine ones; and the other forms'.
// ACME's general purpose classes are named alike on either side.
var (
	metricExternal      = []string{"4h", "6e", "6f", "6g", "6h", "8g"}
	metricInternal      = []string{"4H", "5H", "6G", "6H", "7H"}
	unifiedExternal     = []string{"1A", "2A", "3A"}
	unifiedInternal     = []string{"1B", "2B", "3B"}
	trapezoidalExternal = []string{"7e", "8e"}
	trapezoidalInternal = []string{"7H", "8H"}
	acmeClasses         = []string{"2G", "3G", "4G", "5G"}
	buttressInchExt     = []string{"2A", "3A"}
	buttressInchInt     = []string{"2B", "3B"}
	buttressExternal    = []string{"7e", "8e", "8c", "9c"}
	buttressInternal    = []string{"7H", "8H", "9H"}
)

// Classes are the tolerance classes offered for a thread of the form
// of s, of a screw or of a nut; none for Whitworth threads, whose fit
// is a clearance typed in.
func Classes(s Spec, internal bool) []string {
	var ext, in []string
	switch s.Form {
	case FormMetric:
		ext, in = metricExternal, metricInternal
	case FormUnified:
		ext, in = unifiedExternal, unifiedInternal
	case FormTrapezoidal:
		ext, in = trapezoidalExternal, trapezoidalInternal
	case FormACME:
		ext, in = acmeClasses, acmeClasses
	case FormButtressInch:
		ext, in = buttressInchExt, buttressInchInt
	case FormButtress:
		ext, in = buttressExternal, buttressInternal
	default:
		return nil
	}
	if internal {
		return slices.Clone(in)
	}
	return slices.Clone(ext)
}

// DefaultClass is the class a thread of the form of s has when none
// is said: 6g and 6H, 2A and 2B (unified and ASME B1.9 buttress), 7e
// and 7H (trapezoidal and DIN 513 buttress), 2G (ACME); "" for a form
// with none.
func DefaultClass(s Spec, internal bool) string {
	switch s.Form {
	case FormMetric:
		return pick(internal, "6H", "6g")
	case FormUnified, FormButtressInch:
		return pick(internal, "2B", "2A")
	case FormTrapezoidal, FormButtress:
		return pick(internal, "7H", "7e")
	case FormACME:
		return "2G"
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
	switch s.Form {
	case FormUnified:
		return unifiedClearance(s, class), nil
	case FormACME:
		return acmeClearance(s, internal, class), nil
	case FormButtressInch:
		return buttressInchClearance(s, class), nil
	case FormButtress:
		return buttressClearance(s, internal, class), nil
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
	case FormButtress, FormButtressInch:
		return "buttress"
	}
	return "metric"
}

// isoClearance is ClassClearance for ISO 965-1 and ISO 2903 classes:
// a grade digit and a position letter.
func isoClearance(s Spec, internal bool, class string) float64 {
	grade, position := int(class[0]-'0'), class[1]
	td2 := isoTolerance(s.Form, s.Diameter, s.Pitch, grade, internal)
	return (isoDeviation(s.Form, s.Pitch, position) + td2/2) / 2 / 1000
}
