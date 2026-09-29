package cfmt

import (
	"math"
	"os"
	"strings"
	"testing"
)

// TestSprintfVsGlibc compares Sprintf with glibc on the cases
// scripts/cfmt-glibc.c prints.
func TestSprintfVsGlibc(t *testing.T) {
	nan := math.NaN()
	vals := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, 1e-5, 123456789, 1.0 / 3, 2.5, 3.5, -2.675, 1e21, 1e-300, nan, math.Copysign(nan, -1), math.Inf(1), math.Inf(-1), 0.000123456, 99999.95, 12.3456789}
	fmts := []string{"%f", "%.3f", "%.10f", "%g", "%.6g", "%e", "%.2e", "%10.3f", "%-10.3f|", "%+.2f", "% .2f", "%.0f", "%#.3g", "%lg"}
	var b strings.Builder
	for _, v := range vals {
		for _, f := range fmts {
			b.WriteString(Sprintf(f, v) + "\n")
		}
	}
	ivals := []int{0, 5, -5, 255, 123456}
	ifmts := []string{"%d", "%5d", "%-5d|", "%05d", "%x", "%08x", "%02x", "%X", "%u", "%010d"}
	for _, v := range ivals {
		for _, f := range ifmts {
			b.WriteString(Sprintf(f, v) + "\n")
		}
	}
	b.WriteString(Sprintf("%c|%5s|%-5s|%.2s|%%\n", 65, "ab", "ab", "abcdef"))
	want, _ := os.ReadFile("testdata/glibc.txt")
	w := strings.Split(string(want), "\n")
	g := strings.Split(b.String(), "\n")
	for i := range w {
		if i >= len(g) || w[i] != g[i] {
			t.Errorf("line %d: C %q Go %q", i, w[i], g[min(i, len(g)-1)])
		}
	}
}
