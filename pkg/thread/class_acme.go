package thread

import "math"

// The pitch diameter tolerances and allowances of ASME B1.5's general
// purpose ACME classes, in inches.
//
// Class 4G's tolerance is 0.010 p^(1/2) + 0.002 D^(1/2), the pitch
// increment and the diameter increment, with p the pitch and D the
// basic major diameter; 2G's, 3G's and 5G's are 3.0, 1.4 and 0.8 times
// it, a nut's the same as a screw's of its class. A screw's pitch
// diameter has an allowance of 0.008, 0.006, 0.004 and 0 times D^(1/2)
// for 2G, 3G, 4G and 5G; a nut's none. Each is rounded to 0.0001 inch,
// as the tables print them, and holds for lengths of engagement up to
// twice D.
//
// Against Engineers Edge's table of B1.5's external general purpose
// threads this gives every pitch diameter of 1-5 and 3/4-6 ACME in
// 2G to 5G (TestACMETable), and the internal 4G of 3/4-6 that Machining
// Doctor lists. B1.5's appendix B adds to the allowance of threads of
// several starts, which this leaves out: a thread's tolerances are of
// its pitch, as for one start.

// acmeScales scale class 4G's pitch diameter tolerance to each class's,
// and acmeAllowances are each class's allowance on a screw, per
// D^(1/2).
var (
	acmeScales     = map[string]float64{"2G": 3, "3G": 1.4, "4G": 1, "5G": 0.8}
	acmeAllowances = map[string]float64{"2G": 0.008, "3G": 0.006, "4G": 0.004, "5G": 0}
)

// acmeTolerance is the pitch diameter tolerance and the allowance, in
// inches, of an ACME class of a thread of diameter d and pitch p, in
// inches; the allowance is a screw's, which a nut hasn't.
func acmeTolerance(d, p float64, class string) (tolerance, allowance float64) {
	td2 := 0.010*math.Sqrt(p) + 0.002*math.Sqrt(d)
	return roundTenThousandth(acmeScales[class] * td2), roundTenThousandth(acmeAllowances[class] * math.Sqrt(d))
}

// acmeClearance is ClassClearance for ASME B1.5's general purpose
// classes.
func acmeClearance(s Spec, internal bool, class string) float64 {
	tolerance, allowance := acmeTolerance(s.Diameter/mmPerInch, s.Pitch/mmPerInch, class)
	if internal {
		allowance = 0
	}
	return (allowance + tolerance/2) / 2 * mmPerInch
}
