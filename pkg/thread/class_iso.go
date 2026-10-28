package thread

import "math"

// The pitch diameter tolerances and fundamental deviations of ISO metric
// (ISO 965-1) and ISO metric trapezoidal (ISO 2903) threads, in µm.
//
// Both standards compute them from formulae — es = -(15 + 11P) for g,
// -(30 + 11P) for f, -(50 + 11P) for e and EI = +(15 + 11P) for G;
// Td2(6) = 90 P^0.4 d^0.1 with d the geometric mean of the limits of
// the diameter range the size falls in — and round the result to the
// nearest number of the R40 series, but "in order to reproduce a smooth
// progression, these rules of rounding off have not always been used"
// (ISO 965-1:1998 clause 13): M10 6H's TD2 is 180 µm where the formula
// rounds to 170. So the tables are what is looked up, and the formulae
// only give what they lack: a pitch a diameter range doesn't list, a
// grade it leaves blank, a size outside its ranges.

// isoRanges are the limits of ISO 965-1's ranges of basic major
// diameter, in mm: a size is over one limit and up to and including the
// next. ISO 2903's tables use the same ranges.
var isoRanges = []float64{0.99, 1.4, 2.8, 5.6, 11.2, 22.4, 45, 90, 180, 355}

// isoRangeOf is the lower limit of the range a diameter d falls in and
// the geometric mean of the range's limits, which the formulae take as
// the diameter; d for both when d is outside the ranges.
func isoRangeOf(d float64) (over, mean float64) {
	for i := 1; i < len(isoRanges); i++ {
		if d > isoRanges[i-1] && d <= isoRanges[i] {
			return isoRanges[i-1], math.Sqrt(isoRanges[i-1] * isoRanges[i])
		}
	}
	return d, d
}

// metricTolerance is a row of ISO 965-1:1998's Table 6 (Td2, an
// external thread's pitch diameter tolerance, grades 3 to 9) and Table 5
// (TD2, an internal thread's, grades 4 to 8), which have the same rows:
// the lower limit of the diameter range and the pitch, in mm, and the
// tolerances, 0 where the table has none.
type metricTolerance struct {
	over, pitch float64
	ext         [7]int
	in          [5]int
}

// metricTolerances are ISO 965-1:1998's Tables 5 and 6, as BS ISO
// 965-1:1998 prints them.
var metricTolerances = []metricTolerance{
	{0.99, 0.2, [7]int{24, 30, 38, 48, 0, 0, 0}, [5]int{40, 0, 0, 0, 0}},
	{0.99, 0.25, [7]int{26, 34, 42, 53, 0, 0, 0}, [5]int{45, 56, 0, 0, 0}},
	{0.99, 0.3, [7]int{28, 36, 45, 56, 0, 0, 0}, [5]int{48, 60, 75, 0, 0}},
	{1.4, 0.2, [7]int{25, 32, 40, 50, 0, 0, 0}, [5]int{42, 0, 0, 0, 0}},
	{1.4, 0.25, [7]int{28, 36, 45, 56, 0, 0, 0}, [5]int{48, 60, 0, 0, 0}},
	{1.4, 0.35, [7]int{32, 40, 50, 63, 80, 0, 0}, [5]int{53, 67, 85, 0, 0}},
	{1.4, 0.4, [7]int{34, 42, 53, 67, 85, 0, 0}, [5]int{56, 71, 90, 0, 0}},
	{1.4, 0.45, [7]int{36, 45, 56, 71, 90, 0, 0}, [5]int{60, 75, 95, 0, 0}},
	{2.8, 0.35, [7]int{34, 42, 53, 67, 85, 0, 0}, [5]int{56, 71, 90, 0, 0}},
	{2.8, 0.5, [7]int{38, 48, 60, 75, 95, 0, 0}, [5]int{63, 80, 100, 125, 0}},
	{2.8, 0.6, [7]int{42, 53, 67, 85, 106, 0, 0}, [5]int{71, 90, 112, 140, 0}},
	{2.8, 0.7, [7]int{45, 56, 71, 90, 112, 0, 0}, [5]int{75, 95, 118, 150, 0}},
	{2.8, 0.75, [7]int{45, 56, 71, 90, 112, 0, 0}, [5]int{75, 95, 118, 150, 0}},
	{2.8, 0.8, [7]int{48, 60, 75, 95, 118, 150, 190}, [5]int{80, 100, 125, 160, 200}},
	{5.6, 0.75, [7]int{50, 63, 80, 100, 125, 0, 0}, [5]int{85, 106, 132, 170, 0}},
	{5.6, 1, [7]int{56, 71, 90, 112, 140, 180, 224}, [5]int{95, 118, 150, 190, 236}},
	{5.6, 1.25, [7]int{60, 75, 95, 118, 150, 190, 236}, [5]int{100, 125, 160, 200, 250}},
	{5.6, 1.5, [7]int{67, 85, 106, 132, 170, 212, 265}, [5]int{112, 140, 180, 224, 280}},
	{11.2, 1, [7]int{60, 75, 95, 118, 150, 190, 236}, [5]int{100, 125, 160, 200, 250}},
	{11.2, 1.25, [7]int{67, 85, 106, 132, 170, 212, 265}, [5]int{112, 140, 180, 224, 280}},
	{11.2, 1.5, [7]int{71, 90, 112, 140, 180, 224, 280}, [5]int{118, 150, 190, 236, 300}},
	{11.2, 1.75, [7]int{75, 95, 118, 150, 190, 236, 300}, [5]int{125, 160, 200, 250, 315}},
	{11.2, 2, [7]int{80, 100, 125, 160, 200, 250, 315}, [5]int{132, 170, 212, 265, 335}},
	{11.2, 2.5, [7]int{85, 106, 132, 170, 212, 265, 335}, [5]int{140, 180, 224, 280, 355}},
	{22.4, 1, [7]int{63, 80, 100, 125, 160, 200, 250}, [5]int{106, 132, 170, 212, 0}},
	{22.4, 1.5, [7]int{75, 95, 118, 150, 190, 236, 300}, [5]int{125, 160, 200, 250, 315}},
	{22.4, 2, [7]int{85, 106, 132, 170, 212, 265, 335}, [5]int{140, 180, 224, 280, 355}},
	{22.4, 3, [7]int{100, 125, 160, 200, 250, 315, 400}, [5]int{170, 212, 265, 335, 425}},
	{22.4, 3.5, [7]int{106, 132, 170, 212, 265, 335, 425}, [5]int{180, 224, 280, 355, 450}},
	{22.4, 4, [7]int{112, 140, 180, 224, 280, 355, 450}, [5]int{190, 236, 300, 375, 475}},
	{22.4, 4.5, [7]int{118, 150, 190, 236, 300, 375, 475}, [5]int{200, 250, 315, 400, 500}},
	{45, 1.5, [7]int{80, 100, 125, 160, 200, 250, 315}, [5]int{132, 170, 212, 265, 335}},
	{45, 2, [7]int{90, 112, 140, 180, 224, 280, 355}, [5]int{150, 190, 236, 300, 375}},
	{45, 3, [7]int{106, 132, 170, 212, 265, 335, 425}, [5]int{180, 224, 280, 355, 450}},
	{45, 4, [7]int{118, 150, 190, 236, 300, 375, 475}, [5]int{200, 250, 315, 400, 500}},
	{45, 5, [7]int{125, 160, 200, 250, 315, 400, 500}, [5]int{212, 265, 335, 425, 530}},
	{45, 5.5, [7]int{132, 170, 212, 265, 335, 425, 530}, [5]int{224, 280, 355, 450, 560}},
	{45, 6, [7]int{140, 180, 224, 280, 355, 450, 560}, [5]int{236, 300, 375, 475, 600}},
	{90, 2, [7]int{95, 118, 150, 190, 236, 300, 375}, [5]int{160, 200, 250, 315, 400}},
	{90, 3, [7]int{112, 140, 180, 224, 280, 355, 450}, [5]int{190, 236, 300, 375, 475}},
	{90, 4, [7]int{125, 160, 200, 250, 315, 400, 500}, [5]int{212, 265, 335, 425, 530}},
	{90, 6, [7]int{150, 190, 236, 300, 375, 475, 600}, [5]int{250, 315, 400, 500, 630}},
	{90, 8, [7]int{170, 212, 265, 335, 425, 530, 670}, [5]int{280, 355, 450, 560, 710}},
	{180, 3, [7]int{125, 160, 200, 250, 315, 400, 500}, [5]int{212, 265, 335, 425, 530}},
	{180, 4, [7]int{140, 180, 224, 280, 355, 450, 560}, [5]int{236, 300, 375, 475, 600}},
	{180, 6, [7]int{160, 200, 250, 315, 400, 500, 630}, [5]int{265, 335, 425, 530, 670}},
	{180, 8, [7]int{180, 224, 280, 355, 450, 560, 710}, [5]int{300, 375, 475, 600, 750}},
}

// metricDeviation is a row of ISO 965-1:1998's Table 1: a pitch, in mm,
// and the size of the fundamental deviations of positions g (and G), f
// and e, 0 where the table has none (e below 0.5 mm, f below 0.35).
type metricDeviation struct {
	pitch   float64
	g, f, e int
}

// metricDeviations are ISO 965-1:1998's Table 1.
var metricDeviations = []metricDeviation{
	{0.2, 17, 0, 0}, {0.25, 18, 0, 0}, {0.3, 18, 0, 0}, {0.35, 19, 34, 0},
	{0.4, 19, 34, 0}, {0.45, 20, 35, 0}, {0.5, 20, 36, 50}, {0.6, 21, 36, 53},
	{0.7, 22, 38, 56}, {0.75, 22, 38, 56}, {0.8, 24, 38, 60}, {1, 26, 40, 60},
	{1.25, 28, 42, 63}, {1.5, 32, 45, 67}, {1.75, 34, 48, 71}, {2, 38, 52, 71},
	{2.5, 42, 58, 80}, {3, 48, 63, 85}, {3.5, 53, 70, 90}, {4, 60, 75, 95},
	{4.5, 63, 80, 100}, {5, 71, 85, 106}, {5.5, 75, 90, 112}, {6, 80, 95, 118},
	{8, 100, 118, 140},
}

// trapezoidalTolerance is a row of ISO 2903's pitch diameter
// tolerances: the lower limit of the diameter range and the pitch, in
// mm, and Td2 of grades 7 and 8 and TD2 of grades 7 and 8.
type trapezoidalTolerance struct {
	over, pitch float64
	ext, in     [2]int
}

// trapezoidalTolerances are ISO 2903:1993's, for every diameter and
// pitch of ISO 2902, as the calculator of obliczenia.icad.pl
// (sprawdzianyGw/trapezowe, "acc. to ISO 2903:1993") gives them for
// classes 7e, 8e, 7H and 8H. Where a row is also in ISO 965-1's tables
// the two agree (Tr 20x2 7e 200, 7H 265; Tr 10x1.5 7e 170).
var trapezoidalTolerances = []trapezoidalTolerance{
	{5.6, 1.5, [2]int{170, 212}, [2]int{224, 280}}, {5.6, 2, [2]int{190, 236}, [2]int{250, 315}},
	{5.6, 3, [2]int{212, 265}, [2]int{280, 355}}, {11.2, 2, [2]int{200, 250}, [2]int{265, 335}},
	{11.2, 3, [2]int{224, 280}, [2]int{300, 375}}, {11.2, 4, [2]int{265, 335}, [2]int{355, 450}},
	{11.2, 5, [2]int{280, 355}, [2]int{375, 475}}, {11.2, 8, [2]int{355, 450}, [2]int{475, 600}},
	{22.4, 3, [2]int{250, 315}, [2]int{335, 425}}, {22.4, 5, [2]int{300, 375}, [2]int{400, 500}},
	{22.4, 6, [2]int{335, 425}, [2]int{450, 560}}, {22.4, 7, [2]int{355, 450}, [2]int{475, 600}},
	{22.4, 8, [2]int{375, 475}, [2]int{500, 630}}, {22.4, 10, [2]int{400, 500}, [2]int{530, 670}},
	{22.4, 12, [2]int{425, 530}, [2]int{560, 710}}, {45, 3, [2]int{265, 335}, [2]int{355, 450}},
	{45, 4, [2]int{300, 375}, [2]int{400, 500}}, {45, 8, [2]int{400, 500}, [2]int{530, 670}},
	{45, 9, [2]int{425, 530}, [2]int{560, 710}}, {45, 10, [2]int{425, 530}, [2]int{560, 710}},
	{45, 12, [2]int{475, 600}, [2]int{630, 800}}, {45, 14, [2]int{500, 630}, [2]int{670, 850}},
	{45, 16, [2]int{530, 670}, [2]int{710, 900}}, {45, 18, [2]int{560, 710}, [2]int{750, 950}},
	{90, 4, [2]int{315, 400}, [2]int{425, 530}}, {90, 6, [2]int{375, 475}, [2]int{500, 630}},
	{90, 8, [2]int{425, 530}, [2]int{560, 710}}, {90, 12, [2]int{500, 630}, [2]int{670, 850}},
	{90, 14, [2]int{530, 670}, [2]int{710, 900}}, {90, 16, [2]int{560, 710}, [2]int{750, 950}},
	{90, 18, [2]int{600, 750}, [2]int{800, 1000}}, {90, 20, [2]int{600, 750}, [2]int{800, 1000}},
	{90, 22, [2]int{630, 800}, [2]int{850, 1060}}, {90, 24, [2]int{670, 850}, [2]int{900, 1120}},
	{90, 28, [2]int{710, 900}, [2]int{950, 1180}}, {180, 8, [2]int{450, 560}, [2]int{600, 750}},
	{180, 12, [2]int{530, 670}, [2]int{710, 900}}, {180, 18, [2]int{630, 800}, [2]int{850, 1060}},
	{180, 20, [2]int{670, 850}, [2]int{900, 1120}}, {180, 22, [2]int{670, 850}, [2]int{900, 1120}},
	{180, 24, [2]int{710, 900}, [2]int{950, 1180}}, {180, 32, [2]int{800, 1000}, [2]int{1060, 1320}},
	{180, 36, [2]int{850, 1060}, [2]int{1120, 1400}}, {180, 40, [2]int{850, 1060}, [2]int{1120, 1400}},
	{180, 44, [2]int{900, 1120}, [2]int{1250, 1500}},
}

// trapezoidalDeviation is a pitch, in mm, and the size of ISO 2903's
// fundamental deviation e there: ISO 965-1's -(50 + 11P) up to 6 mm, and
// less than it beyond.
type trapezoidalDeviation struct {
	pitch float64
	e     int
}

// trapezoidalDeviations are from the same calculator as
// trapezoidalTolerances.
var trapezoidalDeviations = []trapezoidalDeviation{
	{1.5, 67}, {2, 71}, {3, 85}, {4, 95}, {5, 106}, {6, 118},
	{7, 125}, {8, 132}, {9, 140}, {10, 150}, {12, 160}, {14, 180},
	{16, 190}, {18, 200}, {20, 212}, {22, 224}, {24, 236}, {28, 250},
	{32, 265}, {36, 280}, {40, 300}, {44, 315},
}

// externalGrades and internalGrades scale the grade 6 Td2 of the
// formula to a grade's Td2 and TD2 (ISO 965-1 clauses 13.4.1, 13.4.2).
var (
	externalGrades = map[int]float64{3: 0.5, 4: 0.63, 5: 0.8, 6: 1, 7: 1.25, 8: 1.6, 9: 2}
	internalGrades = map[int]float64{4: 0.85, 5: 1.06, 6: 1.32, 7: 1.7, 8: 2.12}
)

// isoTolerance is the pitch diameter tolerance, µm, of the given grade
// of a thread of form f (metric or trapezoidal), diameter d and pitch
// p, external or internal: the table's, or the formula's rounded to R40.
func isoTolerance(f Form, d, p float64, grade int, internal bool) float64 {
	over, mean := isoRangeOf(d)
	if t := isoTableTolerance(f, over, p, grade, internal); t > 0 {
		return float64(t)
	}
	scale := externalGrades[grade]
	if internal {
		scale = internalGrades[grade]
	}
	return roundR40(scale * 90 * math.Pow(p, 0.4) * math.Pow(mean, 0.1))
}

// isoTableTolerance is the tolerance of the tables' row for the range
// over and the pitch p, 0 when they have none.
func isoTableTolerance(f Form, over, p float64, grade int, internal bool) int {
	if f == FormTrapezoidal {
		for _, r := range trapezoidalTolerances {
			if r.over == over && same(r.pitch, p) && grade >= 7 && grade <= 8 {
				if internal {
					return r.in[grade-7]
				}
				return r.ext[grade-7]
			}
		}
		return 0
	}
	for _, r := range metricTolerances {
		if r.over == over && same(r.pitch, p) {
			switch {
			case internal && grade >= 4 && grade <= 8:
				return r.in[grade-4]
			case !internal && grade >= 3 && grade <= 9:
				return r.ext[grade-3]
			}
		}
	}
	return 0
}

// isoDeviation is the size of the fundamental deviation, µm, of the
// position (e, f, g or G; 0 for h and H) of a thread of form f and
// pitch p: the table's or, for a pitch it lacks, the formula's rounded
// to R40. A trapezoidal pitch ISO 2903 doesn't list takes the deviation
// of the nearest one it does, the formula running away from its table
// above 6 mm.
func isoDeviation(f Form, p float64, position byte) float64 {
	if f == FormTrapezoidal && position == 'e' {
		best := trapezoidalDeviations[0]
		for _, r := range trapezoidalDeviations {
			if math.Abs(math.Log(r.pitch/p)) < math.Abs(math.Log(best.pitch/p)) {
				best = r
			}
		}
		return float64(best.e)
	}
	base := 0.0
	switch position {
	case 'e':
		base = 50
	case 'f':
		base = 30
	case 'g', 'G':
		base = 15
	default:
		return 0
	}
	for _, r := range metricDeviations {
		if !same(r.pitch, p) {
			continue
		}
		v := r.g
		switch position {
		case 'e':
			v = r.e
		case 'f':
			v = r.f
		}
		if v > 0 {
			return float64(v)
		}
	}
	return roundR40(base + 11*p)
}

// r40 are the numbers of the R40 series of preferred numbers in a
// decade (ISO 3).
var r40 = []float64{
	100, 106, 112, 118, 125, 132, 140, 150, 160, 170, 180, 190, 200, 212, 224, 236, 250, 265, 280, 300,
	315, 335, 355, 375, 400, 425, 450, 475, 500, 530, 560, 600, 630, 670, 710, 750, 800, 850, 900, 950, 1000,
}

// roundR40 rounds x, a positive number of µm, to the nearest number of
// the R40 series and then to a whole number, as ISO 965-1 clause 13
// says the tables' values were.
func roundR40(x float64) float64 {
	scale := math.Pow(10, math.Floor(math.Log10(x))-2)
	best := r40[0] * scale
	for _, r := range r40 {
		if math.Abs(r*scale-x) < math.Abs(best-x) {
			best = r * scale
		}
	}
	return math.Round(best)
}
