package thread

import (
	"math"
	"strings"
	"testing"
)

// TestClassClearance checks the clearance a class gives against what
// the tables of ISO 965-2 and ASME B1.1 make of it: half of the
// deviation plus half the tolerance, on the diameter, as a radius.
func TestClassClearance(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		thread, class string
		internal      bool
		// es and td2 from the tables, in µm, and how near the formulas
		// come to them.
		es, td2, tol float64
	}{
		// ISO 965-2: M10 6g, d2 8.994 to 8.862; 6H, D2 9.026 to 9.206.
		{"M10", "6g", false, 32, 132, 3},
		{"M10", "6H", true, 0, 180, 8},
		// M6 6g, d2 5.324 to 5.212.
		{"M6", "6g", false, 26, 112, 3},
		// ASME B1.1: 1/4-20 UNC 2A, pitch diameter 0.2164 to 0.2127
		// inch on a basic 0.2175; 2B, 0.2175 to 0.2224. The formula's
		// Td2 is 0.0040 inch, some 7% over the table's.
		{"1/4-20 UNC", "2A", false, 0.0011 * 25400, 0.0037 * 25400, 10},
		{"1/4-20 UNC", "2B", true, 0, 0.0049 * 25400, 10},
		// 3A has no allowance.
		{"1/4-20 UNC", "3A", false, 0, 0.0028 * 25400, 10},
	} {
		s, err := Parse(tc.thread)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ClassClearance(s, tc.internal, tc.class)
		if err != nil {
			t.Errorf("%s %s: %v", tc.thread, tc.class, err)
			continue
		}
		want := (tc.es + tc.td2/2) / 2 / 1000
		if math.Abs(got-want) > tc.tol/2/1000 {
			t.Errorf("%s %s: clearance %.4f mm, the table's %.4f", tc.thread, tc.class, got, want)
		}
	}
}

// TestClassOrder checks that the classes order as their letters say:
// e looser than f than g than h, a coarser grade looser than a finer.
func TestClassOrder(t *testing.T) {
	t.Parallel()
	s, _ := Parse("M12")
	c := func(class string, internal bool) float64 {
		v, err := ClassClearance(s, internal, class)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	if !(c("6e", false) > c("6f", false) && c("6f", false) > c("6g", false) && c("6g", false) > c("6h", false)) {
		t.Error("e, f, g, h don't loosen in order")
	}
	if !(c("4h", false) < c("6h", false) && c("6H", true) < c("7H", true) && c("6H", true) < c("6G", true)) {
		t.Error("grades or G don't loosen")
	}
	u, _ := Parse("1/2-13 UNC")
	a1, _ := ClassClearance(u, false, "1A")
	a3, _ := ClassClearance(u, false, "3A")
	b1, _ := ClassClearance(u, true, "1B")
	b3, _ := ClassClearance(u, true, "3B")
	if !(a1 > a3 && b1 > b3) {
		t.Error("unified classes don't tighten from 1 to 3")
	}
	tr, _ := Parse("Tr20x4")
	e7, _ := ClassClearance(tr, false, "7e")
	e8, _ := ClassClearance(tr, false, "8e")
	if !(e7 > 0 && e8 > e7) {
		t.Errorf("Tr 7e %v, 8e %v", e7, e8)
	}
}

// TestClassRefused checks the classes a thread doesn't take.
func TestClassRefused(t *testing.T) {
	t.Parallel()
	m, _ := Parse("M8")
	w, _ := Parse("1/4 BSW")
	for _, tc := range []struct {
		s        Spec
		internal bool
		class    string
		says     string
	}{
		{m, false, "6H", "internal"},
		{m, true, "6g", "external"},
		{m, false, "2A", "6g"},
		{m, false, "", "unknown"},
		{w, false, "6g", "no tolerance classes"},
	} {
		_, err := ClassClearance(tc.s, tc.internal, tc.class)
		if err == nil || !strings.Contains(err.Error(), tc.says) {
			t.Errorf("%v %q: %v, want one saying %q", tc.s, tc.class, err, tc.says)
		}
	}
}

// TestDefaultClass checks every form's default is among its classes,
// and a form without classes has none.
func TestDefaultClass(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"M8", "1/4-20 UNC", "Tr20x4", "1/4 BSW", "1/2-10 ACME"} {
		s, _ := Parse(name)
		for _, internal := range []bool{false, true} {
			def := DefaultClass(s, internal)
			classes := Classes(s, internal)
			if (def == "") != (len(classes) == 0) {
				t.Errorf("%s internal %v: default %q of %v", name, internal, def, classes)
			}
			if def != "" {
				if _, err := ClassClearance(s, internal, def); err != nil {
					t.Errorf("%s: %v", name, err)
				}
			}
		}
	}
}
