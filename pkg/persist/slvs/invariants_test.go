package slvs_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/check"
	"github.com/spacesolve/spacesolve/pkg/persist/slvs"
)

// invariantsPath is the baseline TestInvariants compares with.
var invariantsPath = filepath.Join("..", "..", "..", "testdata", "invariants.txt")

// invariantsRelTol is how far a volume, centre of mass or bounding box
// coordinate may move, relative to the larger of 1 and the recorded
// value. It is tight while the kernel's output still matches C++; it
// is loosened when the kernel starts to diverge (ROADMAP-JUMP.md,
// package 5.1).
const invariantsRelTol = 1e-9

// invariantFixtures lists every .slvs file under testdata, as paths
// from testdata with forward slashes.
func invariantFixtures(t *testing.T) []string {
	root := filepath.Join("..", "..", "..", "testdata")
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".slvs") {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// invariantLines loads a fixture, regenerates it, and returns one line
// per group: the fixture, the group's handle and its check.Report. A
// fixture that does not load, on purpose, gives one line with the error.
func invariantLines(fixture string) []string {
	doc, err := slvs.LoadFile(filepath.Join("..", "..", "..", "testdata", filepath.FromSlash(fixture)))
	if err != nil {
		return []string{fmt.Sprintf("%s error %q", fixture, err.Error())}
	}
	sk := doc.Sketch
	sk.GenerateAll()
	var lines []string
	for _, hg := range sk.GroupOrder {
		r := check.Group(sk, sk.GetGroup(hg))
		lines = append(lines, fmt.Sprintf("%s g%x %s", fixture, hg.V, r.String()))
	}
	return lines
}

// TestInvariants loads every fixture, regenerates it, checks the solid
// of each group, and compares the reports with testdata/invariants.txt.
// Counts and flags must match exactly; volume, centre of mass and
// bounding box to invariantsRelTol. SPACESOLVE_REGEN_INVARIANTS=1
// rewrites the file; review its diff before committing.
//
// The baseline was recorded while the kernel's output was verified
// byte for byte against C++ SolveSpace, so it is the judge of every
// later kernel change.
func TestInvariants(t *testing.T) {
	fixtures := invariantFixtures(t)
	var got []string
	for _, f := range fixtures {
		got = append(got, invariantLines(f)...)
	}

	if os.Getenv("SPACESOLVE_REGEN_INVARIANTS") != "" {
		data := "# Invariant baseline: one line per fixture and group, from TestInvariants\n" +
			"# in pkg/persist/slvs. Regenerate with SPACESOLVE_REGEN_INVARIANTS=1.\n" +
			strings.Join(got, "\n") + "\n"
		if err := os.WriteFile(invariantsPath, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	wantB, err := os.ReadFile(invariantsPath)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{}
	var order []string
	for _, line := range strings.Split(string(wantB), "\n") {
		if line == "" || line[0] == '#' {
			continue
		}
		key := invariantKey(line)
		want[key] = line
		order = append(order, key)
	}
	seen := map[string]bool{}
	for _, line := range got {
		key := invariantKey(line)
		seen[key] = true
		w, ok := want[key]
		if !ok {
			t.Errorf("not in the baseline: %s", line)
			continue
		}
		if msg := compareInvariants(line, w); msg != "" {
			t.Errorf("%s: %s\n got: %s\nwant: %s", key, msg, line, w)
		}
	}
	for _, key := range order {
		if !seen[key] {
			t.Errorf("in the baseline but not produced: %s", want[key])
		}
	}
}

// invariantKey is the fixture and group a line is about.
func invariantKey(line string) string {
	f := strings.Fields(line)
	if len(f) < 2 {
		return line
	}
	return f[0] + " " + f[1]
}

// compareInvariants compares two report lines field by field and says
// what differs, or returns "".
func compareInvariants(got, want string) string {
	gf, wf := invariantFields(got), invariantFields(want)
	var diffs []string
	for _, name := range []string{"tri", "faces", "naked", "inter", "failed", "vol", "com", "min", "max"} {
		g, w := gf[name], wf[name]
		if g == w {
			continue
		}
		switch name {
		case "vol", "com", "min", "max":
			if numbersNear(g, w) {
				continue
			}
		}
		diffs = append(diffs, fmt.Sprintf("%s %q, want %q", name, g, w))
	}
	return strings.Join(diffs, "; ")
}

func invariantFields(line string) map[string]string {
	m := map[string]string{}
	for _, f := range strings.Fields(line)[2:] {
		if k, v, ok := strings.Cut(f, "="); ok {
			m[k] = v
		}
	}
	return m
}

// numbersNear compares comma-separated numbers to invariantsRelTol.
func numbersNear(got, want string) bool {
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
		if math.Abs(g-w) > invariantsRelTol*math.Max(1, math.Abs(w)) {
			return false
		}
	}
	return true
}

// TestInvariantsCompare checks the comparison itself.
func TestInvariantsCompare(t *testing.T) {
	base := "a.slvs g4 tri=12 faces=6 naked=0 inter=0 failed=false vol=24 com=1,2,3 min=0,0,0 max=2,3,4"
	for _, c := range []struct {
		got  string
		same bool
	}{
		{base, true},
		{strings.Replace(base, "vol=24", "vol=24.000000000001", 1), true},
		{strings.Replace(base, "vol=24", "vol=24.0001", 1), false},
		{strings.Replace(base, "com=1,2,3", "com=1,2,3.1", 1), false},
		{strings.Replace(base, "com=1,2,3", "com=1,2", 1), false},
		{strings.Replace(base, "max=2,3,4", "max=2,x,4", 1), false},
		{strings.Replace(base, "naked=0", "naked=1", 1), false},
		{strings.Replace(base, "failed=false", "failed=true", 1), false},
		{"a.slvs g4 tri=0 faces=0 naked=0 inter=0 failed=false", false},
	} {
		if msg := compareInvariants(c.got, base); (msg == "") != c.same {
			t.Errorf("compare(%q) = %q, want same=%t", c.got, msg, c.same)
		}
	}
	if k := invariantKey("lonely"); k != "lonely" {
		t.Errorf("invariantKey = %q", k)
	}
}
