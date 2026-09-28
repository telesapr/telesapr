// Command libmcheck compares pkg/libm with the system's libm, through
// cgo, on millions of random inputs over each function's branches, and
// on special values. It's a separate module so that the main one needs
// no C toolchain.
//
// Usage (from this directory):
//
//	go run . [-rounds N] sin|cos|tan|asin|acos|atan|atan2|special|all
//
// Each round is 2^20 inputs per range.
package main

/*
#cgo LDFLAGS: -lm
#include <math.h>
static void vsin(const double *x, double *r, int n) { for (int i = 0; i < n; i++) r[i] = sin(x[i]); }
static void vcos(const double *x, double *r, int n) { for (int i = 0; i < n; i++) r[i] = cos(x[i]); }
static void vtan(const double *x, double *r, int n) { for (int i = 0; i < n; i++) r[i] = tan(x[i]); }
static void vasin(const double *x, double *r, int n) { for (int i = 0; i < n; i++) r[i] = asin(x[i]); }
static void vacos(const double *x, double *r, int n) { for (int i = 0; i < n; i++) r[i] = acos(x[i]); }
static void vatan(const double *x, double *r, int n) { for (int i = 0; i < n; i++) r[i] = atan(x[i]); }
static void vatan2(const double *y, const double *x, double *r, int n) { for (int i = 0; i < n; i++) r[i] = atan2(y[i], x[i]); }
*/
import "C"

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"unsafe"

	"github.com/spacesolve/spacesolve/pkg/libm"
)

const n = 1 << 20

type fn1 struct {
	g func(float64) float64
	c func(x, r *C.double, n C.int)
	// ranges are the [lo, hi] magnitudes to sample, one per branch.
	ranges [][2]float64
}

var (
	trig = [][2]float64{{1e-300, 1e-8}, {1e-8, 0.126}, {0.126, 0.855469}, {0.855469, 2.426265}, {2.426265, 30}, {30, 1e8}, {1e8, 1e300}, {0, 7}}
	inv  = [][2]float64{{1e-300, 0.125}, {0.125, 0.5}, {0.5, 0.75}, {0.75, 0.921875}, {0.921875, 0.953125}, {0.953125, 0.96875}, {0.96875, 1}, {0.999999, 1}, {0, 1}}
	fns  = map[string]fn1{
		"sin":  {libm.Sin, func(x, r *C.double, n C.int) { C.vsin(x, r, n) }, trig},
		"cos":  {libm.Cos, func(x, r *C.double, n C.int) { C.vcos(x, r, n) }, trig},
		"tan":  {libm.Tan, func(x, r *C.double, n C.int) { C.vtan(x, r, n) }, trig},
		"asin": {libm.Asin, func(x, r *C.double, n C.int) { C.vasin(x, r, n) }, inv},
		"acos": {libm.Acos, func(x, r *C.double, n C.int) { C.vacos(x, r, n) }, inv},
		"atan": {libm.Atan, func(x, r *C.double, n C.int) { C.vatan(x, r, n) },
			[][2]float64{{1e-300, 1e-8}, {1e-8, 1.0 / 16}, {1.0 / 16, 1}, {1, 16}, {16, 1e16}, {1e16, 1e300}, {0, 3}}},
	}
)

func cptr(s []float64) *C.double { return (*C.double)(unsafe.Pointer(&s[0])) }

// sample is in [lo, hi], log-uniform when that spans decades, with a
// random sign when signed.
func sample(r *rand.Rand, lo, hi float64, signed bool) float64 {
	var x float64
	if lo > 0 && hi/lo > 16 {
		x = math.Exp(math.Log(lo) + r.Float64()*(math.Log(hi)-math.Log(lo)))
	} else {
		x = lo + r.Float64()*(hi-lo)
	}
	if signed && r.Intn(2) == 0 {
		x = -x
	}
	return x
}

func same(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || math.IsNaN(a) && math.IsNaN(b)
}

var failed bool

func report(name string, bad, total int) {
	fmt.Printf("%s: %d/%d differ\n", name, bad, total)
	if bad > 0 {
		failed = true
	}
}

func check1(name string, f fn1, rounds int) {
	r := rand.New(rand.NewSource(42))
	x := make([]float64, n)
	want := make([]float64, n)
	for _, g := range f.ranges {
		bad, total := 0, 0
		for k := 0; k < rounds; k++ {
			for i := range x {
				x[i] = sample(r, g[0], g[1], true)
			}
			f.c(cptr(x), cptr(want), n)
			for i := range x {
				got := f.g(x[i])
				total++
				if !same(got, want[i]) {
					if bad < 3 {
						fmt.Printf("  %s(%x) = %x, want %x\n", name, x[i], got, want[i])
					}
					bad++
				}
			}
		}
		report(fmt.Sprintf("%s [%g, %g]", name, g[0], g[1]), bad, total)
	}
}

func checkAtan2(rounds int) {
	r := rand.New(rand.NewSource(7))
	y := make([]float64, n)
	x := make([]float64, n)
	want := make([]float64, n)
	for _, br := range []struct {
		name       string
		xneg, ygtx bool
		lo, hi     float64
	}{
		{"i small", false, false, 1e-6, 1.0 / 16}, {"i table", false, false, 1.0 / 16, 1},
		{"ii small", false, true, 1e-6, 1.0 / 16}, {"ii table", false, true, 1.0 / 16, 1},
		{"iii small", true, true, 1e-6, 1.0 / 16}, {"iii table", true, true, 1.0 / 16, 1},
		{"iv small", true, false, 1e-6, 1.0 / 16}, {"iv table", true, false, 1.0 / 16, 1},
	} {
		bad, total := 0, 0
		for k := 0; k < rounds; k++ {
			for i := range x {
				a := sample(r, 1e-3, 1e3, false)
				b := a * sample(r, br.lo, br.hi, false)
				if br.ygtx {
					y[i], x[i] = a, b
				} else {
					y[i], x[i] = b, a
				}
				if br.xneg {
					x[i] = -x[i]
				}
				if r.Intn(2) == 0 {
					y[i] = -y[i]
				}
			}
			C.vatan2(cptr(y), cptr(x), cptr(want), n)
			for i := range x {
				got := libm.Atan2(y[i], x[i])
				total++
				if !same(got, want[i]) {
					if bad < 3 {
						fmt.Printf("  atan2(%x, %x) = %x, want %x\n", y[i], x[i], got, want[i])
					}
					bad++
				}
			}
		}
		report("atan2 "+br.name, bad, total)
	}
}

func checkSpecial() {
	base := []float64{0, math.Inf(1), math.NaN(), 1, 0.5, 2, math.Pi, math.Pi / 2, math.Pi / 4,
		1e-300, 5e-324, math.MaxFloat64, 0.125, 0.0625, 16, 0.126, 0.855469, 2.426265, 25, 1e8, 105414350,
		0.0608, 0.787, 1.259e-8, 0.75, 0.921875, 0.953125, 0.96875, 2.77556e-17, 1.29e-8, 5.805e15, 3 * math.Pi / 4}
	var xs []float64
	for _, b := range base {
		for _, v := range []float64{b, -b} {
			xs = append(xs, v, math.Nextafter(v, math.Inf(1)), math.Nextafter(v, math.Inf(-1)))
		}
	}
	// The high words glibc's branches compare with.
	for _, e := range []uint32{0x3e500000, 0x3feb6000, 0x400368fd, 0x419921FB, 0x3e400000, 0x3fc00000,
		0x3fe00000, 0x3fd00000, 0x3fe80000, 0x3fed8000, 0x3fee8000, 0x3fef0000, 0x3ff00000, 0x3c880000} {
		v := math.Float64frombits(uint64(e) << 32)
		xs = append(xs, v, math.Nextafter(v, 0), -v, -math.Nextafter(v, 0))
	}
	want := make([]float64, len(xs))
	bad := 0
	for _, name := range []string{"sin", "cos", "tan", "asin", "acos", "atan"} {
		f := fns[name]
		f.c(cptr(xs), cptr(want), C.int(len(xs)))
		for i, x := range xs {
			if got := f.g(x); !same(got, want[i]) {
				fmt.Printf("  %s(%x) = %x, want %x\n", name, x, got, want[i])
				bad++
			}
		}
	}
	var ys, xs2 []float64
	for _, y := range xs {
		for _, x := range xs {
			ys = append(ys, y)
			xs2 = append(xs2, x)
		}
	}
	w2 := make([]float64, len(ys))
	C.vatan2(cptr(ys), cptr(xs2), cptr(w2), C.int(len(ys)))
	for i := range ys {
		if got := libm.Atan2(ys[i], xs2[i]); !same(got, w2[i]) {
			fmt.Printf("  atan2(%x, %x) = %x, want %x\n", ys[i], xs2[i], got, w2[i])
			bad++
		}
	}
	report("special", bad, 6*len(xs)+len(ys))
}

func main() {
	rounds := flag.Int("rounds", 4, "rounds of 2^20 inputs per range")
	flag.Parse()
	which := flag.Arg(0)
	if which == "" {
		which = "all"
	}
	for _, name := range []string{"special", "sin", "cos", "tan", "asin", "acos", "atan", "atan2"} {
		if which != "all" && which != name {
			continue
		}
		switch name {
		case "special":
			checkSpecial()
		case "atan2":
			checkAtan2(*rounds)
		default:
			check1(name, fns[name], *rounds)
		}
	}
	if failed {
		os.Exit(1)
	}
}
