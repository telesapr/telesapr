package libm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"testing"
)

// valueTable prints, one per line as "name x [y] result" with the bits
// in hex, each function's result for 600 inputs per branch of its
// argument reduction and polynomial choice, and for special values and
// their neighbours. The inputs come from a splitmix64 generator, so the
// table is the same on every run.
func valueTable() []byte {
	state := uint64(1)
	next := func() uint64 { // splitmix64
		state += 0x9e3779b97f4a7c15
		z := state
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		return z ^ (z >> 31)
	}
	unif := func() float64 { return float64(next()>>11) * 0x1p-53 }
	// sample is in [lo, hi], of either sign. When that spans decades
	// it is about log-uniform: a binary exponent drawn uniformly and a
	// uniform mantissa, redrawn until it falls in the range. Ldexp and
	// Ilogb are exact, where math.Exp and math.Log are assembly that
	// differs between architectures, so the inputs are the same on all.
	sample := func(lo, hi float64) float64 {
		var x float64
		if lo > 0 && hi/lo > 16 {
			elo, ehi := math.Ilogb(lo), math.Ilogb(hi)
			for {
				e := elo + int(next()%uint64(ehi-elo+1))
				x = math.Ldexp(1+unif(), e)
				if x >= lo && x <= hi {
					break
				}
			}
		} else {
			x = lo + unif()*(hi-lo)
		}
		if next()&1 != 0 {
			return -x
		}
		return x
	}
	bits := math.Float64bits
	// result is a result's bits, every NaN written as one: which NaN an
	// operation on two of them gives, and its sign, is the compiler's
	// choice, and differs between builds (the race detector's for one).
	result := func(v float64) uint64 {
		if math.IsNaN(v) {
			return 0x7ff8000000000001
		}
		return bits(v)
	}
	var out []byte
	const n = 600
	sp := []float64{0, math.Inf(1), math.NaN(), 1, 0.5, 2, math.Pi, math.Pi / 2, math.Pi / 4, 1e-300, 5e-324,
		math.MaxFloat64, 0.125, 0.0625, 16, 0.126, 0.855469, 2.426265, 25, 1e8, 105414350, 0.0608, 0.787,
		1.259e-8, 0.75, 0.921875, 0.953125, 0.96875, 2.77556e-17, 1.29e-8, 5.805e15}
	one := func(name string, f func(float64) float64, r []float64) {
		for i := 0; i+1 < len(r); i += 2 {
			for range n {
				x := sample(r[i], r[i+1])
				out = fmt.Appendf(out, "%s %016x %016x\n", name, bits(x), result(f(x)))
			}
		}
		for _, s := range sp {
			for _, x := range []float64{s, -s, math.Nextafter(s, math.Inf(1)), math.Nextafter(s, math.Inf(-1))} {
				out = fmt.Appendf(out, "%s %016x %016x\n", name, bits(x), result(f(x)))
			}
		}
	}
	trig := []float64{1e-300, 1e-8, 1e-8, 0.126, 0.126, 0.855469, 0.855469, 2.426265, 2.426265, 30,
		30, 1e8, 1e8, 1e300, 0, 7}
	inv := []float64{1e-300, 0.125, 0.125, 0.5, 0.5, 0.75, 0.75, 0.921875, 0.921875, 0.953125,
		0.953125, 0.96875, 0.96875, 1, 0.999999, 1}
	at := []float64{1e-300, 1e-8, 1e-8, 0.0625, 0.0625, 1, 1, 16, 16, 1e16, 1e16, 1e300}
	one("sin", Sin, trig)
	one("cos", Cos, trig)
	one("tan", Tan, trig)
	one("asin", Asin, inv)
	one("acos", Acos, inv)
	one("atan", Atan, at)
	// atan2: each branch (sign of x, which of |x| and |y| is larger,
	// the ratio below or above 1/16), then pairs of special values.
	for br := range 8 {
		for range n {
			a := math.Abs(sample(1e-3, 1e3))
			var b float64
			if br&1 != 0 {
				b = a * math.Abs(sample(1.0/16, 1))
			} else {
				b = a * math.Abs(sample(1e-6, 1.0/16))
			}
			y, x := b, a
			if br&2 != 0 {
				y, x = a, b
			}
			if br&4 != 0 {
				x = -x
			}
			if next()&1 != 0 {
				y = -y
			}
			out = fmt.Appendf(out, "atan2 %016x %016x %016x\n", bits(y), bits(x), result(Atan2(y, x)))
		}
	}
	sp2 := []float64{0, math.Inf(1), math.NaN(), 1, 2, 1e-300, 1e300, 5e-324}
	pairs := func(name string, f func(a, b float64) float64) {
		for _, u := range sp2 {
			for _, v := range sp2 {
				for s := range 4 {
					a, b := u, v
					if s&1 != 0 {
						a = -a
					}
					if s&2 != 0 {
						b = -b
					}
					out = fmt.Appendf(out, "%s %016x %016x %016x\n", name, bits(a), bits(b), result(f(a, b)))
				}
			}
		}
	}
	pairs("atan2", Atan2)
	// hypot: both kernel branches (the ratio of the smaller to the
	// larger above or below about 0.5), huge and tiny inputs, widely
	// different ones, then pairs of special values.
	for br := range 5 {
		for range n {
			x := math.Abs(sample(1e-3, 1e3))
			var y float64
			switch br {
			case 0:
				y = x * math.Abs(sample(0.5, 1))
			case 1:
				y = x * math.Abs(sample(1e-8, 0.5))
			case 2:
				x *= 0x1p+600
				y = x * math.Abs(sample(1e-3, 1))
			case 3:
				x *= 0x1p-600
				y = x * math.Abs(sample(1e-3, 1))
			default:
				y = x * math.Abs(sample(1e-20, 1e-15))
			}
			if next()&1 != 0 {
				x = -x
			}
			if next()&1 != 0 {
				y = -y
			}
			out = fmt.Appendf(out, "hypot %016x %016x %016x\n", bits(x), bits(y), result(Hypot(x, y)))
		}
	}
	pairs("hypot", Hypot)
	return out
}

// TestLibmValues pins the bits every function gives over valueTable:
// the goldens and the exports record the last bits of the kernel's
// trigonometry, so a change here moves them. The table was checked
// line for line against glibc 2.44's FMA variants when it was made.
func TestLibmValues(t *testing.T) {
	t.Parallel()
	const want = "d58bf6b73f1a26febd6c50c82ab10addcdf4068169f41594b8babdeb31e74599"
	sum := sha256.Sum256(valueTable())
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("hash %s, want %s", got, want)
	}
}

// mulAdd is x*y + z, computed the way the engine's code is.
//
//go:noinline
func mulAdd(x, y, z float64) float64 { return x*y + z }

// TestNoFusedMultiplyAdd fails when the compiler fuses x*y + z, as Go
// does for GOAMD64=v3 and on arm64: the ports (and the whole engine)
// round every product the code writes as one, and use math.FMA where
// a fused multiply-add is meant, so a fusing build moves the goldens.
func TestNoFusedMultiplyAdd(t *testing.T) {
	t.Parallel()
	if got := mulAdd(1+0x1p-30, 1-0x1p-30, -1); got != 0 {
		t.Errorf("x*y + z = %g, want 0: this build fuses multiply-adds (build with GOAMD64=v1 or v2)", got)
	}
}
