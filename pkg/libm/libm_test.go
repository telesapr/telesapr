package libm

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLibmVsGlibc requires the bits glibc computes for inputs spread
// over each function's branches, and for special values
// (testdata/libm/glibc.txt, from scripts/regen-libm-golden.sh).
func TestLibmVsGlibc(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "libm", "glibc.txt"))
	if err != nil {
		t.Fatal(err)
	}
	fns := map[string]func(float64) float64{
		"sin": Sin, "cos": Cos, "tan": Tan, "asin": Asin, "acos": Acos, "atan": Atan,
	}
	f64 := func(s string) float64 {
		b, err := strconv.ParseUint(s, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		return math.Float64frombits(b)
	}
	same := func(a, b float64) bool {
		return math.Float64bits(a) == math.Float64bits(b) || math.IsNaN(a) && math.IsNaN(b)
	}
	failed := map[string]int{}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		f := strings.Fields(line)
		var got, want float64
		var args []float64
		switch f[0] {
		case "atan2":
			args = []float64{f64(f[1]), f64(f[2])}
			got, want = Atan2(args[0], args[1]), f64(f[3])
		case "hypot":
			args = []float64{f64(f[1]), f64(f[2])}
			got, want = Hypot(args[0], args[1]), f64(f[3])
		default:
			args = []float64{f64(f[1])}
			got, want = fns[f[0]](args[0]), f64(f[2])
		}
		if !same(got, want) {
			if failed[f[0]] < 5 {
				t.Errorf("%s%v = %x, want %x", f[0], args, got, want)
			}
			failed[f[0]]++
		}
	}
	for name, n := range failed {
		t.Errorf("%s: %d values differ", name, n)
	}
	if len(lines) < 10000 {
		t.Fatalf("only %d values", len(lines))
	}
}

// mulAdd is x*y + z, computed the way the engine's code is.
//
//go:noinline
func mulAdd(x, y, z float64) float64 { return x*y + z }

// TestNoFusedMultiplyAdd fails when the compiler fuses x*y + z, as Go
// does for GOAMD64=v3 and on arm64: the ports (and the whole engine)
// round every product where C++ does, and use math.FMA where it fuses.
func TestNoFusedMultiplyAdd(t *testing.T) {
	t.Parallel()
	if got := mulAdd(1+0x1p-30, 1-0x1p-30, -1); got != 0 {
		t.Errorf("x*y + z = %g, want 0: this build fuses multiply-adds (build with GOAMD64=v1 or v2)", got)
	}
}
