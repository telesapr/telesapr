package check

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

// A baseline is a text file of report lines, one per fixture and
// group: "<fixture> g<handle> <Report.String()>", as
// testdata/invariants.txt holds. Lines starting with # are comments.

// BaselineKey is the fixture and group a baseline line is about.
func BaselineKey(line string) string {
	f := strings.Fields(line)
	if len(f) < 2 {
		return line
	}
	return f[0] + " " + f[1]
}

// ReadBaseline reads a baseline file into its lines by key, and the
// keys in file order.
func ReadBaseline(path string) (map[string]string, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	lines := map[string]string{}
	var order []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		key := BaselineKey(line)
		lines[key] = line
		order = append(order, key)
	}
	return lines, order, nil
}

// BaselineRelTol is how far a volume, centre of mass or bounding box
// coordinate may move from the baseline, relative to the larger of 1
// and the recorded value. It was 1e-9 while the kernel's output matched
// SolveSpace's byte for byte, and was loosened when that comparison was
// frozen (ADR 0001), so that kernel fixes can move the last digits
// without a new baseline.
const BaselineRelTol = 1e-6

// CompareBaseline compares two baseline lines field by field and says
// what differs, or returns "". Counts and flags must match exactly;
// volume, centre of mass and bounding box to relTol, relative to the
// larger of 1 and the wanted value.
func CompareBaseline(got, want string, relTol float64) string {
	gf, wf := baselineFields(got), baselineFields(want)
	var diffs []string
	for _, name := range []string{"tri", "faces", "naked", "inter", "failed", "vol", "com", "min", "max"} {
		g, w := gf[name], wf[name]
		if g == w {
			continue
		}
		switch name {
		case "vol", "com", "min", "max":
			if numbersNear(g, w, relTol) {
				continue
			}
		}
		diffs = append(diffs, fmt.Sprintf("%s %q, want %q", name, g, w))
	}
	return strings.Join(diffs, "; ")
}

func baselineFields(line string) map[string]string {
	m := map[string]string{}
	f := strings.Fields(line)
	if len(f) < 2 {
		return m
	}
	for _, kv := range f[2:] {
		if k, v, ok := strings.Cut(kv, "="); ok {
			m[k] = v
		}
	}
	return m
}

// numbersNear compares comma-separated numbers to relTol.
func numbersNear(got, want string, relTol float64) bool {
	gs, ws := strings.Split(got, ","), strings.Split(want, ",")
	if len(gs) != len(ws) {
		return false
	}
	for i := range gs {
		g, err1 := strconv.ParseFloat(gs[i], 64)
		w, err2 := strconv.ParseFloat(ws[i], 64)
		if err1 != nil || err2 != nil {
			return false
		}
		if math.Abs(g-w) > relTol*math.Max(1, math.Abs(w)) {
			return false
		}
	}
	return true
}
