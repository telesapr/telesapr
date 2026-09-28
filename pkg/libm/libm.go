// Package libm is a port of the glibc (2.44) functions C++ SolveSpace
// calls for its trigonometry: sin, cos, tan, asin, acos, atan and atan2.
// Go's math package computes them differently, and its results differ
// from glibc's in the last bit for a fifth to a third of arguments,
// which shows in the last digits of saved files.
//
// On x86-64 glibc picks, at load time, the variant of each function
// compiled for the CPU. These are the FMA variants (s_sin-fma.c and so
// on), which a CPU with FMA and AVX2 gets: GCC compiled them with
// -mfma -mavx2 and contracted a*b+c into fused multiply-adds, so the
// ports use math.FMA where GCC did. GCC fuses a product into each
// addition or subtraction that uses it, provided every use of the
// product is one; where one sum adds two products, only the first is
// fused. The results are checked bit for bit against the system's libm
// (see libm_test.go).
package libm

import "math"

// f64s makes a table of float64s from their bits.
func f64s(bits []uint64) []float64 {
	f := make([]float64, len(bits))
	for i, b := range bits {
		f[i] = math.Float64frombits(b)
	}
	return f
}

// hi is the high word of x's bits (glibc's u.i[HIGH_HALF]).
func hi(x float64) int32 { return int32(math.Float64bits(x) >> 32) }

// lo is the low word of x's bits (u.i[LOW_HALF]).
func lo(x float64) int32 { return int32(uint32(math.Float64bits(x))) }

// fromBits is a float64 from its bits.
func fromBits(b uint64) float64 { return math.Float64frombits(b) }
