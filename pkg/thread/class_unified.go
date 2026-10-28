package thread

import "math"

// The pitch diameter tolerances and allowances of ASME B1.1's unified
// classes, in inches.
//
// Class 2A's tolerance is 0.0015 D^(1/3) + 0.0015 LE^(1/2) + 0.015
// P^(2/3), with D the basic major diameter and LE the length of
// engagement; the other classes' are 1.5 (1A), 0.75 (3A), 1.95 (1B),
// 1.3 (2B) and 0.975 (3B) times it, and 1A and 2A have an allowance of
// 0.3 times it; each is rounded to 0.0001 inch. The tables take LE as D
// for the UNC and UNF series ("The pitch diameter tolerances ... for all
// classes of the UNC and UNF series are based on a length of engagement
// equal to the basic major (nominal) diameter", ASME B1.1-2003 5.8) and
// as 9 pitches for the others: 1/2-28 UNEF 2A's tolerance is 0.0037 inch
// (SAAMI's suppressor thread drawings, pitch diameter 0.4757 to 0.4720),
// what 9P gives, where D would give 0.0039.
//
// Against the UNC 2A table of steelmasters.co.nz ("External Thread
// Dimensions for Unified UNC Screw Threads", #1 to 4 inch) this gives
// every tolerance and allowance but two: 1 1/8-7's tolerance, 0.0073
// for the table's 0.0072 (the formula's 0.0072502 is all but a tie),
// and 5/8-11's allowance, 0.0017 for 0.0016 (0.3 times 0.0055010).
// Of 1/4-20's six classes (machiningdoctor.com) it gives all but 2B's
// tolerance, 0.0048 for 0.0049 (1.3 times 0.0037306 is 0.0048498).

// unifiedScales scale class 2A's pitch diameter tolerance to each
// unified class's.
var unifiedScales = map[string]float64{"1A": 1.5, "2A": 1, "3A": 0.75, "1B": 1.95, "2B": 1.3, "3B": 0.975}

// uncLarge are the UNC sizes over 2 inches, which the series of sizes
// to pick from (unified) stops short of, and their threads per inch.
var uncLarge = []struct{ d, tpi float64 }{
	{2.25, 4.5}, {2.5, 4}, {2.75, 4}, {3, 4}, {3.25, 4}, {3.5, 4}, {3.75, 4}, {4, 4},
}

// unifiedTolerance is the pitch diameter tolerance and the allowance,
// in inches, of a unified class of a thread of diameter d and pitch p,
// in inches.
func unifiedTolerance(d, p float64, class string) (tolerance, allowance float64) {
	le := 9 * p
	if coarseOrFine(d, 1/p) {
		le = d
	}
	td2 := 0.0015*math.Cbrt(d) + 0.0015*math.Sqrt(le) + 0.015*math.Pow(p, 2.0/3)
	if class == "1A" || class == "2A" {
		allowance = roundTenThousandth(0.3 * td2)
	}
	return roundTenThousandth(unifiedScales[class] * td2), allowance
}

// coarseOrFine reports whether a diameter d, in inches, at tpi threads
// per inch is a UNC or UNF thread.
func coarseOrFine(d, tpi float64) bool {
	for _, u := range unified {
		if same(u.d, d) && (same(u.unc, tpi) || same(u.unf, tpi)) {
			return true
		}
	}
	for _, u := range uncLarge {
		if same(u.d, d) && same(u.tpi, tpi) {
			return true
		}
	}
	return false
}

// roundTenThousandth rounds an inch measure to 0.0001 inch, as the
// tables print it.
func roundTenThousandth(x float64) float64 {
	return math.Round(x*1e4) / 1e4
}

// unifiedClearance is ClassClearance for ASME B1.1's classes; the class
// says the side.
func unifiedClearance(s Spec, class string) float64 {
	tolerance, allowance := unifiedTolerance(s.Diameter/mmPerInch, s.Pitch/mmPerInch, class)
	return (allowance + tolerance/2) / 2 * mmPerInch
}
