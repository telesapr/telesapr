package cfmt

import (
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/golden"
)

// TestSprintfGolden pins what Sprintf prints for values and verbs of
// the kinds the file writers use, rounding, signs, NaNs and widths
// among them, since the goldens record that output byte for byte
// (testdata/sprintf.golden; scripts/regen-golden.sh sprintf rewrites
// it).
func TestSprintfGolden(t *testing.T) {
	t.Parallel()
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
	golden.Check(t, "testdata/sprintf.golden", []byte(b.String()))
}
