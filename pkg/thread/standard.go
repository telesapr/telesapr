package thread

import "math"

// metricCoarse is the coarse pitch of each ISO 261 diameter.
var metricCoarse = []struct{ d, pitch float64 }{
	{1, 0.25}, {1.2, 0.25}, {1.4, 0.3}, {1.6, 0.35}, {1.8, 0.35},
	{2, 0.4}, {2.5, 0.45}, {3, 0.5}, {3.5, 0.6}, {4, 0.7}, {5, 0.8},
	{6, 1}, {7, 1}, {8, 1.25}, {10, 1.5}, {12, 1.75}, {14, 2},
	{16, 2}, {18, 2.5}, {20, 2.5}, {22, 2.5}, {24, 3}, {27, 3},
	{30, 3.5}, {33, 3.5}, {36, 4}, {39, 4}, {42, 4.5}, {45, 4.5},
	{48, 5}, {52, 5}, {56, 5.5}, {60, 5.5}, {64, 6},
}

// metricFine is the ISO 261 fine pitches of each diameter that has
// them, coarsest first: those of ISO 262's selected sizes.
var metricFine = map[float64][]float64{
	1: {0.2}, 1.2: {0.2}, 1.4: {0.2}, 1.6: {0.2}, 1.8: {0.2},
	2: {0.25}, 2.5: {0.35}, 3: {0.35}, 3.5: {0.35}, 4: {0.5}, 5: {0.5},
	6: {0.75}, 7: {0.75}, 8: {1, 0.75}, 10: {1.25, 1, 0.75},
	12: {1.5, 1.25, 1}, 14: {1.5, 1.25, 1}, 16: {1.5, 1},
	18: {2, 1.5, 1}, 20: {2, 1.5, 1}, 22: {2, 1.5, 1}, 24: {2, 1.5, 1},
	27: {2, 1.5, 1}, 30: {3, 2, 1.5, 1}, 33: {3, 2, 1.5}, 36: {3, 2, 1.5},
	39: {3, 2, 1.5}, 42: {4, 3, 2, 1.5}, 45: {4, 3, 2, 1.5},
	48: {4, 3, 2, 1.5}, 52: {4, 3, 2, 1.5}, 56: {4, 3, 2, 1.5},
	60: {4, 3, 2, 1.5}, 64: {4, 3, 2, 1.5},
}

// trapezoidal is the pitches of each ISO 2904 diameter that ISO 2902
// gives, coarse, preferred and fine; the preferred one is the pitch
// of a designation without one.
var trapezoidal = []struct {
	d, pitch float64
	others   []float64
}{
	{8, 1.5, nil}, {10, 2, []float64{1.5}}, {12, 3, []float64{2}},
	{14, 3, []float64{2}}, {16, 4, []float64{2}}, {18, 4, []float64{2}},
	{20, 4, []float64{2}}, {22, 5, []float64{8, 3}}, {24, 5, []float64{8, 3}},
	{26, 5, []float64{8, 3}}, {28, 5, []float64{8, 3}}, {30, 6, []float64{10, 3}},
	{32, 6, []float64{10, 3}}, {36, 6, []float64{10, 3}}, {40, 7, []float64{10, 3}},
	{44, 7, []float64{12, 3}}, {48, 8, []float64{12, 3}}, {52, 8, []float64{12, 3}},
	{60, 9, []float64{14, 3}},
}

// inchSize is a size of an inch series: its name and diameter in
// inches, and its threads per inch.
type inchSize struct {
	name string
	d    float64
	tpi  float64
}

// bsw and bsf are the British Standard Whitworth and Fine series of
// BS 84.
var bsw = []inchSize{
	{"1/8", 1.0 / 8, 40}, {"3/16", 3.0 / 16, 24}, {"1/4", 1.0 / 4, 20},
	{"5/16", 5.0 / 16, 18}, {"3/8", 3.0 / 8, 16}, {"7/16", 7.0 / 16, 14},
	{"1/2", 1.0 / 2, 12}, {"9/16", 9.0 / 16, 12}, {"5/8", 5.0 / 8, 11},
	{"3/4", 3.0 / 4, 10}, {"7/8", 7.0 / 8, 9}, {"1", 1, 8},
	{"1 1/8", 1 + 1.0/8, 7}, {"1 1/4", 1 + 1.0/4, 7}, {"1 1/2", 1 + 1.0/2, 6},
	{"1 3/4", 1 + 3.0/4, 5}, {"2", 2, 4.5},
}

var bsf = []inchSize{
	{"3/16", 3.0 / 16, 32}, {"7/32", 7.0 / 32, 28}, {"1/4", 1.0 / 4, 26},
	{"9/32", 9.0 / 32, 26}, {"5/16", 5.0 / 16, 22}, {"3/8", 3.0 / 8, 20},
	{"7/16", 7.0 / 16, 18}, {"1/2", 1.0 / 2, 16}, {"9/16", 9.0 / 16, 16},
	{"5/8", 5.0 / 8, 14}, {"11/16", 11.0 / 16, 14}, {"3/4", 3.0 / 4, 12},
	{"7/8", 7.0 / 8, 11}, {"1", 1, 10}, {"1 1/8", 1 + 1.0/8, 9},
	{"1 1/4", 1 + 1.0/4, 9}, {"1 1/2", 1 + 1.0/2, 8}, {"1 3/4", 1 + 3.0/4, 7},
	{"2", 2, 7},
}

// pipeThread is a pipe thread's size: its name, its major diameter
// in millimetres and its threads per inch.
type pipeThread struct {
	name   string
	d, tpi float64
}

// pipe is the parallel pipe thread of ISO 228-1 (BSP, G): the name of
// each size, which is not its diameter, and the major diameter in
// millimetres.
var pipe = []pipeThread{
	{"1/16", 7.723, 28}, {"1/8", 9.728, 28}, {"1/4", 13.157, 19},
	{"3/8", 16.662, 19}, {"1/2", 20.955, 14}, {"5/8", 22.911, 14},
	{"3/4", 26.441, 14}, {"7/8", 30.201, 14}, {"1", 33.249, 11},
	{"1 1/8", 37.897, 11}, {"1 1/4", 41.910, 11}, {"1 1/2", 47.803, 11},
	{"1 3/4", 53.746, 11}, {"2", 59.614, 11}, {"2 1/2", 75.184, 11},
	{"3", 87.884, 11}, {"4", 113.030, 11},
}

// acme is the preferred general purpose ACME series of ASME B1.5.
var acme = []inchSize{
	{"1/4", 1.0 / 4, 16}, {"5/16", 5.0 / 16, 14}, {"3/8", 3.0 / 8, 12},
	{"7/16", 7.0 / 16, 12}, {"1/2", 1.0 / 2, 10}, {"5/8", 5.0 / 8, 8},
	{"3/4", 3.0 / 4, 6}, {"7/8", 7.0 / 8, 6}, {"1", 1, 5},
	{"1 1/8", 1 + 1.0/8, 5}, {"1 1/4", 1 + 1.0/4, 5}, {"1 3/8", 1 + 3.0/8, 4},
	{"1 1/2", 1 + 1.0/2, 4}, {"1 3/4", 1 + 3.0/4, 4}, {"2", 2, 4},
	{"2 1/4", 2 + 1.0/4, 3}, {"2 1/2", 2 + 1.0/2, 3}, {"2 3/4", 2 + 3.0/4, 3},
	{"3", 3, 2}, {"3 1/2", 3 + 1.0/2, 2}, {"4", 4, 2}, {"4 1/2", 4 + 1.0/2, 2},
	{"5", 5, 2},
}

// unified is the unified inch series: the threads per inch of the
// coarse (UNC) and fine (UNF) thread of each size, 0 where the series
// has none. Diameters are in inches.
var unified = []struct {
	name     string
	d        float64
	unc, unf float64
}{
	{"#0", 0.060, 0, 80}, {"#1", 0.073, 64, 72}, {"#2", 0.086, 56, 64},
	{"#3", 0.099, 48, 56}, {"#4", 0.112, 40, 48}, {"#5", 0.125, 40, 44},
	{"#6", 0.138, 32, 40}, {"#8", 0.164, 32, 36}, {"#10", 0.190, 24, 32},
	{"#12", 0.216, 24, 28},
	{"1/4", 1.0 / 4, 20, 28}, {"5/16", 5.0 / 16, 18, 24}, {"3/8", 3.0 / 8, 16, 24},
	{"7/16", 7.0 / 16, 14, 20}, {"1/2", 1.0 / 2, 13, 20}, {"9/16", 9.0 / 16, 12, 18},
	{"5/8", 5.0 / 8, 11, 18}, {"3/4", 3.0 / 4, 10, 16}, {"7/8", 7.0 / 8, 9, 14},
	{"1", 1, 8, 12}, {"1 1/8", 1 + 1.0/8, 7, 12}, {"1 1/4", 1 + 1.0/4, 7, 12},
	{"1 3/8", 1 + 3.0/8, 6, 12}, {"1 1/2", 1 + 1.0/2, 6, 12},
	{"1 3/4", 1 + 3.0/4, 5, 0}, {"2", 2, 4.5, 0},
}

// mmPerInch is the length of an inch.
const mmPerInch = 25.4

// same reports whether two lengths are the same size of a table.
func same(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

// CoarsePitch is the coarse pitch of the ISO metric thread of diameter
// d, and whether the series has that diameter.
func CoarsePitch(d float64) (float64, bool) {
	for _, m := range metricCoarse {
		if same(m.d, d) {
			return m.pitch, true
		}
	}
	return 0, false
}

// TrapezoidalPitch is the preferred pitch of the ISO metric trapezoidal
// thread of diameter d, and whether the series has that diameter.
func TrapezoidalPitch(d float64) (float64, bool) {
	for _, m := range trapezoidal {
		if same(m.d, d) {
			return m.pitch, true
		}
	}
	return 0, false
}

// Nearest is the ISO metric thread whose nominal diameter is nearest
// to d: what a circle of that diameter is likely meant to carry.
func Nearest(d float64) Spec {
	best := metricCoarse[0]
	for _, m := range metricCoarse {
		if math.Abs(m.d-d) < math.Abs(best.d-d) {
			best = m
		}
	}
	return Spec{Form: FormMetric, Diameter: best.d, Pitch: best.pitch}
}
