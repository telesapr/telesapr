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

// trapezoidal is the preferred pitch of each ISO 2904 diameter.
var trapezoidal = []struct{ d, pitch float64 }{
	{8, 1.5}, {10, 2}, {12, 3}, {14, 3}, {16, 4}, {18, 4}, {20, 4},
	{22, 5}, {24, 5}, {26, 5}, {28, 5}, {30, 6}, {32, 6}, {36, 6},
	{40, 7}, {44, 7}, {48, 8}, {52, 8}, {60, 9},
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
