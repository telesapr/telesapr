package thread

import "math"

// The pitch diameter tolerances and allowances of buttress threads:
// ASME B1.9's inch classes, in inches, and DIN 513-3's metric ones, in
// µm.
//
// B1.9's class 2 tolerance is 0.002 D^(1/3) + 0.00278 Le^(1/2) +
// 0.00854 p^(1/2), with D the major diameter, Le the length of
// engagement and p the pitch; its tables take Le as 10 pitches, which
// makes it a diameter increment of 0.002 D^(1/3), D the middle of a
// range of diameters, and a pitch increment of 0.0173 p^(1/2), each
// rounded to 0.00001 inch and their sum to 0.0001, halves to even. Class 3's is two
// thirds of class 2's, and a nut's the same as a screw's of its class.
// A screw's pitch diameter has an allowance of class 3's tolerance in
// either class; a nut's none. That is what NBS Handbook H28 (1957),
// Part III, section XIV, gives for the 7°/45° form (its tables
// XIV.2, XIV.3 and XIV.5, which B1.9 took up), and Machinery's
// Handbook's worked example of a 2-4 BUTT class 2 thread: an allowance
// of 0.0074 and a tolerance of 0.0112. B1.9 drops H28's class 1.
//
// DIN 513-3:2020 takes the pitch diameter tolerances of ISO 2903's
// trapezoidal threads, grades 7 to 9 (its Tables 8 and 9), with
// fundamental deviations of positions c and e of its own (Table 2), and
// a nut's in position H. DIN 513-3 gives the nut's pitch diameter a
// fundamental deviation EI of 3.1758 a, 2/(tan 3° + tan 30°) times the
// axial play a of DIN 513-1, which is not to hand: H is taken as basic,
// as ISO 2903's trapezoidal H is. A thread of several starts takes the
// tolerances of its pitch times the factor of DIN 513-3 clause 12.

// buttressInchRanges are the limits of H28's ranges of major diameter,
// in inches: a size is over one limit and up to and including the next,
// the first including its lower limit.
var buttressInchRanges = []float64{0.5, 11.0 / 16, 1, 1.5, 2.5, 4, 6, 10, 16, 24}

// buttressInchTolerance is the pitch diameter tolerance and the
// allowance, in inches, of class 2 or 3 of an ASME B1.9 thread of
// diameter d and pitch p, in inches.
func buttressInchTolerance(d, p float64, class int) (tolerance, allowance float64) {
	mid := d
	for i := 1; i < len(buttressInchRanges); i++ {
		if d >= buttressInchRanges[i-1]-1e-9 && d <= buttressInchRanges[i]+1e-9 {
			mid = (buttressInchRanges[i-1] + buttressInchRanges[i]) / 2
			break
		}
	}
	// Halves to even: H28's pitch increment of 16 threads per inch,
	// 0.004325, is 0.00432, and ½ to 11/16 inch at 20 threads per inch,
	// 0.00168 + 0.00387, is 0.0056.
	round := func(x, scale float64) float64 { return math.RoundToEven(math.Round(x*scale*1e6)/1e6) / scale }
	td2 := round(0.002*math.Cbrt(mid), 1e5) + round(0.0173*math.Sqrt(p), 1e5)
	allowance = round(2*td2/3, 1e4)
	if class == 3 {
		return allowance, allowance
	}
	return round(td2, 1e4), allowance
}

// buttressInchClearance is ClassClearance for ASME B1.9's classes; the
// class says the side.
func buttressInchClearance(s Spec, class string) float64 {
	tolerance, allowance := buttressInchTolerance(s.Diameter/mmPerInch, s.Pitch/mmPerInch, int(class[0]-'0'))
	if class[1] == 'B' {
		allowance = 0
	}
	return (allowance + tolerance/2) / 2 * mmPerInch
}

// buttressTolerance is a row of DIN 513-3:2020's Tables 9 (Td2, a
// screw's pitch diameter tolerance) and 8 (TD2, a nut's), which have
// the same rows: the lower limit of the diameter range and the pitch,
// in mm, and the tolerances of grades 7, 8 and 9, in µm.
type buttressTolerance struct {
	over, pitch float64
	ext, in     [3]int
}

// buttressTolerances are DIN 513-3:2020's Tables 8 and 9. Where ISO
// 2903's trapezoidal tables have the row, its grades 7 and 8 are theirs
// (trapezoidalTolerances).
var buttressTolerances = []buttressTolerance{
	{5.6, 2, [3]int{190, 236, 300}, [3]int{250, 315, 400}},
	{5.6, 3, [3]int{212, 265, 335}, [3]int{280, 355, 450}},
	{11.2, 2, [3]int{200, 250, 315}, [3]int{265, 335, 425}},
	{11.2, 3, [3]int{224, 280, 355}, [3]int{300, 375, 475}},
	{11.2, 4, [3]int{265, 335, 400}, [3]int{355, 450, 560}},
	{11.2, 5, [3]int{280, 355, 450}, [3]int{375, 475, 600}},
	{11.2, 8, [3]int{355, 450, 560}, [3]int{475, 600, 750}},
	{22.4, 3, [3]int{250, 315, 400}, [3]int{335, 425, 530}},
	{22.4, 5, [3]int{300, 375, 475}, [3]int{400, 500, 630}},
	{22.4, 6, [3]int{335, 425, 530}, [3]int{450, 560, 710}},
	{22.4, 7, [3]int{355, 450, 560}, [3]int{475, 600, 750}},
	{22.4, 8, [3]int{375, 475, 600}, [3]int{500, 630, 800}},
	{22.4, 10, [3]int{400, 500, 630}, [3]int{530, 670, 850}},
	{22.4, 12, [3]int{425, 530, 710}, [3]int{560, 710, 900}},
	{45, 3, [3]int{265, 335, 425}, [3]int{355, 450, 560}},
	{45, 4, [3]int{300, 375, 475}, [3]int{400, 500, 630}},
	{45, 8, [3]int{400, 500, 630}, [3]int{530, 670, 850}},
	{45, 9, [3]int{425, 530, 670}, [3]int{560, 710, 900}},
	{45, 10, [3]int{425, 530, 670}, [3]int{560, 710, 900}},
	{45, 12, [3]int{475, 600, 750}, [3]int{630, 800, 1000}},
	{45, 14, [3]int{500, 630, 800}, [3]int{670, 850, 1060}},
	{45, 16, [3]int{530, 670, 850}, [3]int{710, 900, 1120}},
	{45, 18, [3]int{560, 710, 900}, [3]int{750, 950, 1180}},
	{90, 4, [3]int{315, 400, 500}, [3]int{425, 530, 670}},
	{90, 6, [3]int{375, 475, 600}, [3]int{500, 630, 800}},
	{90, 8, [3]int{425, 530, 670}, [3]int{560, 710, 900}},
	{90, 12, [3]int{500, 630, 800}, [3]int{670, 850, 1060}},
	{90, 14, [3]int{530, 670, 850}, [3]int{710, 900, 1120}},
	{90, 16, [3]int{560, 710, 900}, [3]int{750, 950, 1180}},
	{90, 18, [3]int{600, 750, 950}, [3]int{800, 1000, 1250}},
	{90, 20, [3]int{600, 750, 950}, [3]int{800, 1000, 1250}},
	{90, 22, [3]int{630, 800, 1000}, [3]int{850, 1060, 1320}},
	{90, 24, [3]int{670, 850, 1060}, [3]int{900, 1120, 1400}},
	{90, 28, [3]int{710, 900, 1120}, [3]int{950, 1180, 1500}},
	{180, 8, [3]int{450, 560, 710}, [3]int{600, 750, 950}},
	{180, 12, [3]int{530, 670, 850}, [3]int{710, 900, 1120}},
	{180, 18, [3]int{630, 800, 1000}, [3]int{850, 1060, 1320}},
	{180, 20, [3]int{670, 850, 1060}, [3]int{900, 1120, 1400}},
	{180, 22, [3]int{670, 850, 1060}, [3]int{900, 1120, 1400}},
	{180, 24, [3]int{710, 900, 1120}, [3]int{950, 1180, 1500}},
	{180, 32, [3]int{800, 1000, 1250}, [3]int{1060, 1320, 1700}},
	{180, 36, [3]int{850, 1060, 1320}, [3]int{1120, 1400, 1800}},
	{180, 40, [3]int{850, 1060, 1320}, [3]int{1120, 1400, 1800}},
	{180, 44, [3]int{900, 1120, 1400}, [3]int{1250, 1500, 1900}},
	{355, 12, [3]int{560, 710, 900}, [3]int{760, 950, 1200}},
	{355, 18, [3]int{670, 850, 1060}, [3]int{900, 1120, 1400}},
	{355, 24, [3]int{710, 900, 1120}, [3]int{950, 1180, 1480}},
	{355, 44, [3]int{950, 1220, 1520}, [3]int{1290, 1610, 2000}},
}

// buttressRanges are DIN 513-3's ranges of nominal diameter, in mm,
// those of ISO 965-1 and one more, to 640.
var buttressRanges = []float64{5.6, 11.2, 22.4, 45, 90, 180, 355, 640}

// buttressDeviation is a pitch, in mm, and the size of DIN 513-3's
// fundamental deviations c and e there, in µm (Table 2).
type buttressDeviation struct {
	pitch float64
	c, e  int
}

// buttressDeviations are DIN 513-3:2020's Table 2.
var buttressDeviations = []buttressDeviation{
	{2, 150, 71}, {3, 170, 85}, {4, 190, 95}, {5, 212, 106}, {6, 236, 118},
	{7, 250, 125}, {8, 265, 132}, {9, 280, 140}, {10, 300, 150}, {12, 335, 170},
	{14, 355, 180}, {16, 375, 190}, {18, 400, 200}, {20, 425, 212}, {22, 450, 224},
	{24, 475, 236}, {28, 500, 250}, {32, 530, 265}, {36, 560, 280}, {40, 600, 300},
	{44, 630, 315},
}

// The grade factors of DIN 513-3 clauses 11.2 and 11.4, which scale the
// grade 6 Td2 of the formula, and the factors of clause 12 for threads
// of 2, 3, 4 and 5 or more starts.
var (
	buttressExternalGrades = [3]float64{1.25, 1.6, 2}
	buttressInternalGrades = [3]float64{1.7, 2.12, 2.65}
	buttressStartFactors   = [4]float64{1.12, 1.25, 1.4, 1.6}
)

// buttressPitchTolerance is the pitch diameter tolerance, µm, of grade
// 7, 8 or 9 of a DIN 513 thread of diameter d and pitch p, one start:
// the table's, or DIN 513-3's formula, 90 P^0.4 d^0.1 of the range's
// geometric mean times the grade's factor, rounded to R40.
func buttressPitchTolerance(d, p float64, grade int, internal bool) float64 {
	over, mean := d, d
	for i := 1; i < len(buttressRanges); i++ {
		if d > buttressRanges[i-1] && d <= buttressRanges[i] {
			over, mean = buttressRanges[i-1], math.Sqrt(buttressRanges[i-1]*buttressRanges[i])
			break
		}
	}
	for _, r := range buttressTolerances {
		if r.over == over && same(r.pitch, p) {
			if internal {
				return float64(r.in[grade-7])
			}
			return float64(r.ext[grade-7])
		}
	}
	scale := buttressExternalGrades[grade-7]
	if internal {
		scale = buttressInternalGrades[grade-7]
	}
	return roundR40(scale * 90 * math.Pow(p, 0.4) * math.Pow(mean, 0.1))
}

// buttressPitchDeviation is the size of the fundamental deviation, µm,
// of position c or e of a DIN 513 screw of pitch p: Table 2's, or for a
// pitch it lacks the formula of clause 11.6 rounded to R40.
func buttressPitchDeviation(p float64, position byte) float64 {
	for _, r := range buttressDeviations {
		if same(r.pitch, p) {
			if position == 'c' {
				return float64(r.c)
			}
			return float64(r.e)
		}
	}
	switch {
	case position == 'e' && p <= 6:
		return roundR40(50 + 11*p)
	case position == 'e':
		return roundR40(47.48 * math.Sqrt(p))
	case p <= 2:
		return roundR40(125 + 11*p)
	}
	return roundR40(5 + 94.12*math.Sqrt(p))
}

// buttressClearance is ClassClearance for DIN 513-3's classes: a grade
// digit and a position letter, c or e on a screw and H on a nut.
func buttressClearance(s Spec, internal bool, class string) float64 {
	grade, position := int(class[0]-'0'), class[1]
	td2 := buttressPitchTolerance(s.Diameter, s.Pitch, grade, internal)
	if n := s.NumStarts(); n > 1 {
		td2 *= buttressStartFactors[min(n, 5)-2]
	}
	es := 0.0
	if !internal {
		es = buttressPitchDeviation(s.Pitch, position)
	}
	return (es + td2/2) / 2 / 1000
}
