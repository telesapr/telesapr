package thread

import (
	"math"
	"strings"
	"testing"
)

// TestClassClearance checks the clearance a class gives against what
// the standards' tables make of it: half of the deviation plus half the
// tolerance, on the diameter, as a radius.
func TestClassClearance(t *testing.T) {
	t.Parallel()
	const in = 25400 // µm
	for _, tc := range []struct {
		thread, class string
		internal      bool
		// The deviation and the pitch diameter tolerance from the
		// tables, in µm, and how far the clearance may be from them: 0
		// but for a near tie of the unified formula.
		es, td2, tol float64
	}{
		// ISO 965-1:1998 Tables 1, 5 and 6, across the diameter ranges.
		{"M3", "6g", false, 20, 75, 0},
		{"M3", "6H", true, 0, 100, 0},
		{"M6", "6g", false, 26, 112, 0},
		{"M10", "6g", false, 32, 132, 0},
		{"M10", "6H", true, 0, 180, 0}, // the formula rounds to 170
		{"M10", "4h", false, 0, 85, 0},
		{"M10", "6e", false, 67, 132, 0},
		{"M10", "7H", true, 0, 224, 0},
		{"M20", "6g", false, 42, 170, 0},
		{"M20", "6H", true, 0, 224, 0},
		{"M20", "6e", false, 80, 170, 0},
		{"M20", "6G", true, 42, 224, 0},
		// A pitch the tables don't list for M10: the formula's Td2,
		// 90 x 0.5^0.4 x 7.92^0.1 = 83.9, to R40.
		{"M10x0.5", "6g", false, 20, 85, 0},
		// ISO 2903 (classes 7e to 8H of obliczenia.icad.pl's calculator).
		{"Tr20x4", "7e", false, 95, 265, 0},
		{"Tr20x4", "8e", false, 95, 335, 0},
		{"Tr20x4", "7H", true, 0, 355, 0},
		{"Tr10x2", "7e", false, 71, 190, 0},
		{"Tr40x7", "7e", false, 125, 355, 0},
		// ASME B1.1 1/4-20 UNC, every class (machiningdoctor.com's
		// thread data, pitch diameters 0.2164 to 0.2108 for 1A, to
		// 0.2127 for 2A, 0.2175 to 0.2147 for 3A, and 0.2175 to 0.2248,
		// 0.2224 and 0.2211 for 1B, 2B and 3B).
		{"1/4-20 UNC", "1A", false, 0.0011 * in, 0.0056 * in, 0},
		{"1/4-20 UNC", "2A", false, 0.0011 * in, 0.0037 * in, 0},
		{"1/4-20 UNC", "3A", false, 0, 0.0028 * in, 0},
		{"1/4-20 UNC", "1B", true, 0, 0.0073 * in, 0},
		// 2B's tolerance, 1.3 x 0.0037306 = 0.0048498, rounds to 0.0048
		// where the table has 0.0049.
		{"1/4-20 UNC", "2B", true, 0, 0.0049 * in, 0.0001 * in / 4},
		{"1/4-20 UNC", "3B", true, 0, 0.0036 * in, 0},
		// steelmasters.co.nz's UNC 2A table.
		{"#10-24 UNC", "2A", false, 0.0010 * in, 0.0033 * in, 0},
		{"1/2-13 UNC", "2A", false, 0.0015 * in, 0.0050 * in, 0},
		// SAAMI's 1/2-28 UNEF-2A, 0.4757 to 0.4720 on a basic 0.4768.
		{"1/2-28 UNEF", "2A", false, 0.0011 * in, 0.0037 * in, 0},
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
		if math.Abs(got-want) > tc.tol/1000+1e-9 {
			t.Errorf("%s %s: clearance %.5f mm, the table's %.5f", tc.thread, tc.class, got, want)
		}
	}
}

// TestUnifiedTable checks class 2A's pitch diameter tolerance and
// allowance against a table of every UNC size (steelmasters.co.nz,
// "External Thread Dimensions for Unified UNC Screw Threads"): the
// tolerance is the pitch diameter's range, the allowance how far the
// major diameter's maximum lies under the size.
func TestUnifiedTable(t *testing.T) {
	t.Parallel()
	for _, r := range []struct{ d, tpi, majorMax, pdMax, pdMin float64 }{
		{0.073, 64, 0.0724, 0.0623, 0.0603}, {0.086, 56, 0.0854, 0.0738, 0.0717},
		{0.099, 48, 0.0983, 0.0848, 0.0825}, {0.112, 40, 0.1112, 0.0950, 0.0925},
		{0.125, 40, 0.1242, 0.1080, 0.1054}, {0.138, 32, 0.1372, 0.1169, 0.1141},
		{0.164, 32, 0.1631, 0.1428, 0.1399}, {0.190, 24, 0.1890, 0.1619, 0.1586},
		{0.216, 24, 0.2150, 0.1879, 0.1845}, {1.0 / 4, 20, 0.2489, 0.2164, 0.2127},
		{5.0 / 16, 18, 0.3113, 0.2752, 0.2712}, {3.0 / 8, 16, 0.3737, 0.3331, 0.3287},
		{7.0 / 16, 14, 0.4361, 0.3897, 0.3850}, {1.0 / 2, 13, 0.4985, 0.4485, 0.4435},
		{9.0 / 16, 12, 0.5609, 0.5068, 0.5016}, {5.0 / 8, 11, 0.6234, 0.5644, 0.5589},
		{3.0 / 4, 10, 0.7482, 0.6832, 0.6773}, {7.0 / 8, 9, 0.8731, 0.8009, 0.7946},
		{1, 8, 0.9980, 0.9168, 0.9100}, {1 + 1.0/8, 7, 1.1228, 1.0300, 1.0228},
		{1 + 1.0/4, 7, 1.2478, 1.1550, 1.1476}, {1 + 3.0/8, 6, 1.3726, 1.2643, 1.2563},
		{1 + 1.0/2, 6, 1.4976, 1.3893, 1.3812}, {1 + 3.0/4, 5, 1.7473, 1.6174, 1.6085},
		{2, 4 + 1.0/2, 1.9971, 1.8528, 1.8433}, {2 + 1.0/4, 4 + 1.0/2, 2.2471, 2.1028, 2.0931},
		{2 + 1.0/2, 4, 2.4969, 2.3345, 2.3241}, {2 + 3.0/4, 4, 2.7468, 2.5844, 2.5739},
		{3, 4, 2.9968, 2.8344, 2.8237}, {3 + 1.0/4, 4, 3.2467, 3.0843, 3.0734},
		{3 + 1.0/2, 4, 3.4967, 3.3343, 3.3233}, {3 + 3.0/4, 4, 3.7466, 3.5842, 3.5730},
		{4, 4, 3.9966, 3.8342, 3.8229},
	} {
		tolerance, allowance := unifiedTolerance(r.d, 1/r.tpi, "2A")
		wantTol, wantAllow := r.pdMax-r.pdMin, r.d-r.majorMax
		// The formula's two near ties, a ten-thousandth over the table.
		switch {
		case same(r.d, 1+1.0/8):
			wantTol += 0.0001
		case same(r.d, 5.0/8):
			wantAllow += 0.0001
		}
		if math.Abs(tolerance-wantTol) > 1e-9 || math.Abs(allowance-wantAllow) > 1e-9 {
			t.Errorf("%g-%g UNC 2A: tolerance %.4f, allowance %.4f; the table's %.4f, %.4f",
				r.d, r.tpi, tolerance, allowance, wantTol, wantAllow)
		}
	}
}

// TestISOTables checks the tables as typed in against the formulae
// they were rounded from: every value within one step of the R40
// series (some 6%) of its formula's, which a misplaced digit is not.
func TestISOTables(t *testing.T) {
	t.Parallel()
	near := func(table int, formula float64) bool {
		return math.Abs(math.Log(float64(table)/formula)) < 1.5*math.Log(math.Pow(10, 1.0/40))
	}
	td26 := func(over, p float64) float64 {
		_, mean := isoRangeOf(math.Nextafter(over, math.Inf(1)))
		return 90 * math.Pow(p, 0.4) * math.Pow(mean, 0.1)
	}
	for _, r := range metricTolerances {
		for i, v := range r.ext {
			if v > 0 && !near(v, externalGrades[i+3]*td26(r.over, r.pitch)) {
				t.Errorf("metric over %g, P %g: Td2(%d) %d", r.over, r.pitch, i+3, v)
			}
		}
		for i, v := range r.in {
			if v > 0 && !near(v, internalGrades[i+4]*td26(r.over, r.pitch)) {
				t.Errorf("metric over %g, P %g: TD2(%d) %d", r.over, r.pitch, i+4, v)
			}
		}
	}
	for _, r := range trapezoidalTolerances {
		for i, v := range r.ext {
			if !near(v, externalGrades[i+7]*td26(r.over, r.pitch)) {
				t.Errorf("Tr over %g, P %g: Td2(%d) %d", r.over, r.pitch, i+7, v)
			}
			if !near(r.in[i], internalGrades[i+7]*td26(r.over, r.pitch)) {
				t.Errorf("Tr over %g, P %g: TD2(%d) %d", r.over, r.pitch, i+7, r.in[i])
			}
		}
	}
	for _, r := range metricDeviations {
		for _, d := range []struct {
			v    int
			base float64
		}{{r.g, 15}, {r.f, 30}, {r.e, 50}} {
			// ISO 965-1 footnotes e at 0.5 mm as an exception.
			if d.v > 0 && !(same(r.pitch, 0.5) && d.base == 50) && !near(d.v, d.base+11*r.pitch) {
				t.Errorf("metric P %g: deviation %d, formula %g", r.pitch, d.v, d.base+11*r.pitch)
			}
		}
	}
	// ISO 2903's e is ISO 965-1's up to 6 mm, and grows more slowly
	// after; it never shrinks.
	for i, r := range trapezoidalDeviations {
		if r.pitch <= 6 && !near(r.e, 50+11*r.pitch) {
			t.Errorf("Tr P %g: deviation %d", r.pitch, r.e)
		}
		if i > 0 && r.e < trapezoidalDeviations[i-1].e {
			t.Errorf("Tr P %g: deviation %d shrinks", r.pitch, r.e)
		}
	}
	if got := roundR40(171.8); got != 170 {
		t.Errorf("R40 of 171.8: %g", got)
	}
	if got := roundR40(83.87); got != 85 {
		t.Errorf("R40 of 83.87: %g", got)
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
	acme, _ := Parse("1-5 ACME")
	g2, _ := ClassClearance(acme, false, "2G")
	g5, _ := ClassClearance(acme, false, "5G")
	n2, _ := ClassClearance(acme, true, "2G")
	if !(g2 > n2 && n2 > g5 && g5 > 0) {
		t.Errorf("ACME 2G %v, 2G nut %v, 5G %v", g2, n2, g5)
	}
	sb, _ := Parse("S40x7")
	c8, _ := ClassClearance(sb, false, "8c")
	se8, _ := ClassClearance(sb, false, "8e")
	if !(c8 > se8 && se8 > 0) {
		t.Errorf("S 8c %v, 8e %v", c8, se8)
	}
}

// TestClassRefused checks the classes a thread doesn't take.
func TestClassRefused(t *testing.T) {
	t.Parallel()
	m, _ := Parse("M8")
	w, _ := Parse("1/4 BSW")
	b, _ := Parse("1-12 BUTT")
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
		{b, false, "2B", "internal"},
		{b, false, "1A", "unknown"},
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
	for _, name := range []string{"M8", "1/4-20 UNC", "Tr20x4", "1/4 BSW", "1/2-10 ACME", "S20x4", "1-12 BUTT"} {
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

// TestACMETable checks ASME B1.5's general purpose classes against
// Engineers Edge's table of external ACME threads (pitch diameters, in
// inches, of 1-5 and 3/4-6 in classes 2G to 5G) and Machining Doctor's
// 3/4-6 4G nut, 0.6667 to 0.6725: the allowance is how far the
// maximum lies under the basic pitch diameter, D - p/2, and the
// tolerance the range.
func TestACMETable(t *testing.T) {
	t.Parallel()
	for _, r := range []struct {
		d, tpi       float64
		class        string
		pdMax, pdMin float64
	}{
		{1, 5, "2G", 0.8920, 0.8726}, {1, 5, "3G", 0.8940, 0.8849},
		{1, 5, "4G", 0.8960, 0.8895}, {1, 5, "5G", 0.9000, 0.8948},
		{0.75, 6, "2G", 0.6598, 0.6424}, {0.75, 6, "3G", 0.6615, 0.6534},
		{0.75, 6, "4G", 0.6632, 0.6574}, {0.75, 6, "5G", 0.6667, 0.6620},
	} {
		p := 1 / r.tpi
		tolerance, allowance := acmeTolerance(r.d, p, r.class)
		basic := roundTenThousandth(r.d - p/2)
		if math.Abs(tolerance-(r.pdMax-r.pdMin)) > 1e-9 || math.Abs(allowance-(basic-r.pdMax)) > 1e-9 {
			t.Errorf("%g-%g ACME %s: tolerance %.4f, allowance %.4f; the table's %.4f, %.4f",
				r.d, r.tpi, r.class, tolerance, allowance, r.pdMax-r.pdMin, basic-r.pdMax)
		}
	}
	s, _ := Parse("3/4-6 ACME")
	got, err := ClassClearance(s, true, "4G")
	if want := (0.6725 - 0.6667) / 4 * mmPerInch; err != nil || math.Abs(got-want) > 1e-9 {
		t.Errorf("3/4-6 ACME 4G nut: %v, %v; want %v", got, err, want)
	}
}

// TestButtressInchTable checks ASME B1.9's classes against NBS Handbook
// H28 (1957) Part III, Tables XIV.2 and XIV.3 (class 3 and class 2
// pitch diameter tolerances, in inches) and Table XIV.5 (the
// allowances, class 3's), and Machinery's Handbook's 2-4 class 2
// example: an allowance of 0.0074 and a tolerance of 0.0112. H28's
// class 3 cell of 11/16 to 1 inch at 16 threads per inch reads 0.0042
// where its sum gives 0.0041, the one cell of the rows here that
// disagrees, and is left out.
func TestButtressInchTable(t *testing.T) {
	t.Parallel()
	for _, r := range []struct {
		d, tpi         float64
		class2, class3 float64
	}{
		{0.5, 20, 0.0056, 0.0037}, {0.5, 16, 0.0060, 0.0040}, {0.625, 12, 0.0067, 0.0044},
		{0.875, 12, 0.0069, 0.0046}, {1, 10, 0.0074, 0.0049},
		{1.25, 16, 0.0065, 0.0043}, {1.25, 12, 0.0071, 0.0048}, {1.5, 10, 0.0076, 0.0051},
		{1.375, 8, 0.0083, 0.0055}, {1.5, 6, 0.0092, 0.0061},
		{2, 16, 0.0068, 0.0046}, {2, 12, 0.0075, 0.0050}, {2, 10, 0.0080, 0.0053},
		{2, 8, 0.0086, 0.0058}, {2, 6, 0.0096, 0.0064}, {2.5, 5, 0.0103, 0.0068}, {2, 4, 0.0112, 0.0074},
		{3, 16, 0.0073, 0.0049}, {3, 6, 0.0100, 0.0067}, {4, 4, 0.0116, 0.0077},
		{5, 3, 0.0134, 0.0089},
	} {
		p := 1 / r.tpi
		t2, a2 := buttressInchTolerance(r.d, p, 2)
		t3, a3 := buttressInchTolerance(r.d, p, 3)
		if math.Abs(t2-r.class2) > 1e-9 || math.Abs(t3-r.class3) > 1e-9 || a2 != t3 || a3 != t3 {
			t.Errorf("%g-%g BUTT: class 2 %.4f, class 3 %.4f, allowances %.4f, %.4f; H28's %.4f, %.4f",
				r.d, r.tpi, t2, t3, a2, a3, r.class2, r.class3)
		}
	}
	s, _ := Parse("2-4 BUTT")
	got, err := ClassClearance(s, false, "2A")
	if want := (0.0074 + 0.0112/2) / 2 * mmPerInch; err != nil || math.Abs(got-want) > 1e-9 {
		t.Errorf("2-4 BUTT 2A: %v, %v; want %v", got, err, want)
	}
	if got, _ := ClassClearance(s, true, "2B"); math.Abs(got-0.0112/4*mmPerInch) > 1e-9 {
		t.Errorf("2-4 BUTT 2B: %v", got)
	}
}

// TestButtressMetricTable checks DIN 513-3:2020's classes: the
// deviations of Table 2 and the tolerances of Tables 8 and 9, a row of
// each range, and the formulae for what they lack; and that its grades
// 7 and 8 are ISO 2903's where both list a row.
func TestButtressMetricTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		thread, class string
		internal      bool
		es, td2       float64
	}{
		{"S10x2", "7e", false, 71, 190},
		{"S10x2", "9c", false, 150, 300},
		{"S10x2", "9H", true, 0, 400},
		{"S20x4", "7e", false, 95, 265},
		{"S20x4", "8c", false, 190, 335},
		{"S20x4", "7H", true, 0, 355},
		{"S40x7", "8e", false, 125, 450},
		{"S70x10", "9c", false, 300, 670},
		{"S120x14", "8H", true, 0, 900},
		{"S300x24", "7e", false, 236, 710},
		{"S400x12", "9c", false, 335, 900},
		{"S400x44", "9H", true, 0, 2000},
		// Two starts of 7 mm: the tolerance times DIN 513-3's 1.12,
		// the deviation of the pitch.
		{"S40x14(P7)", "7e", false, 125, 355 * 1.12},
		// A pitch Table 2 lacks: -(50 + 11 P) for e, 5 + 94.12 √P for c,
		// each to R40, and the tolerances by 90 P^0.4 d^0.1 of the
		// range's geometric mean, 31.75, times 1.25 (7) and 2 (9).
		{"S30x1.5", "7e", false, 67, 190},
		{"S30x13", "9c", false, 335, 710},
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
		if want := (tc.es + tc.td2/2) / 2 / 1000; math.Abs(got-want) > 1e-9 {
			t.Errorf("%s %s: clearance %.5f mm, the table's %.5f", tc.thread, tc.class, got, want)
		}
	}
	for _, r := range buttressTolerances {
		for _, tr := range trapezoidalTolerances {
			if tr.over == r.over && same(tr.pitch, r.pitch) &&
				(tr.ext != [2]int{r.ext[0], r.ext[1]} || tr.in != [2]int{r.in[0], r.in[1]}) {
				t.Errorf("over %g, P%g: DIN 513-3 %v %v, ISO 2903 %v %v", r.over, r.pitch, r.ext, r.in, tr.ext, tr.in)
			}
		}
	}
	for _, r := range buttressDeviations {
		e := 47.48 * math.Sqrt(r.pitch)
		if r.pitch <= 6 {
			e = 50 + 11*r.pitch
		}
		c := 5 + 94.12*math.Sqrt(r.pitch)
		if r.pitch <= 2 {
			c = 125 + 11*r.pitch
		}
		// DIN 513-3 says the table differs from the formulae by 3% at
		// most; e at 12 mm is 170 for the formula's 164, 3.4%.
		if math.Abs(float64(r.e)-e) > 0.035*e || math.Abs(float64(r.c)-c) > 0.03*c {
			t.Errorf("P%g: c %d, e %d; the formulae's %.0f, %.0f", r.pitch, r.c, r.e, c, e)
		}
	}
}
