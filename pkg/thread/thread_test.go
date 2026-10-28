package thread

import (
	"math"
	"testing"
)

// TestBasicDimensions compares the pitch and minor diameters with the
// tables of ISO 724 (metric), ASME B1.1 (unified) and ISO 2904
// (trapezoidal).
func TestBasicDimensions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                 string
		pitch, minor, tol    float64
		extMinor, intMajor   float64
		crest, root, intRoot float64 // in pitches; 0 to skip
	}{
		{"M3", 2.675, 2.459, 0.0006, 2.459, 3, 1.0 / 8, 3.0 / 4, 7.0 / 8},
		{"M6", 5.350, 4.917, 0.0006, 4.917, 6, 1.0 / 8, 3.0 / 4, 7.0 / 8},
		{"M8", 7.188, 6.647, 0.0006, 6.647, 8, 1.0 / 8, 3.0 / 4, 7.0 / 8},
		{"M8x1", 7.350, 6.917, 0.0006, 6.917, 8, 0, 0, 0},
		{"M10", 9.026, 8.376, 0.0006, 8.376, 10, 0, 0, 0},
		{"M20", 18.376, 17.294, 0.0006, 17.294, 20, 0, 0, 0},
		// 0.2175 and 0.1959 inch.
		{"1/4-20 UNC", 5.5245, 4.976, 0.002, 4.976, 6.35, 1.0 / 8, 3.0 / 4, 7.0 / 8},
		{"Tr20x4", 18, 16, 1e-9, 15.5, 20.5, 0, 0, 0},
		{"Tr40x7", 36.5, 33, 1e-9, 32, 41, 0, 0, 0},
		// BS 84: effective and minor diameters 0.2180 and 0.1860 inch;
		// the crest truncated a sixth of the sharp triangle's height.
		{"1/4 BSW", 5.5368, 4.7237, 0.001, 4.7237, 6.35, 1.0 / 6, 0, 0},
		// ASME B1.5: 0.450 inch pitch diameter, 0.380 and 0.520 inch at
		// the external root and the internal one.
		{"1/2-10 ACME", 11.43, 10.16, 1e-9, 9.652, 13.208, 0, 0, 0},
		// Finer than 10 threads per inch, half the clearance: 0.1775 and
		// 0.2600 inch.
		{"1/4-16 ACME", 5.55625, 4.7625, 1e-9, 4.5085, 6.604, 0, 0, 0},
	} {
		s, err := Parse(tc.name)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		check := func(what string, got, want float64) {
			if math.Abs(got-want) > tc.tol {
				t.Errorf("%s: %s %g, want %g", tc.name, what, got, want)
			}
		}
		check("pitch diameter", s.PitchDiameter(), tc.pitch)
		check("minor diameter", s.MinorDiameter(), tc.minor)
		check("external minor diameter", s.ExternalMinorDiameter(), tc.extMinor)
		check("internal major diameter", s.InternalMajorDiameter(), tc.intMajor)

		ext, in := s.Profile(false, 0), s.Profile(true, 0)
		check("external crest radius", ext.CrestRadius, s.Diameter/2)
		check("external root radius", ext.RootRadius, tc.extMinor/2)
		check("internal crest radius", in.CrestRadius, tc.minor/2)
		check("internal root radius", in.RootRadius, tc.intMajor/2)
		if tc.crest != 0 {
			check("external crest width", ext.CrestWidth, tc.crest*s.Pitch)
		}
		if tc.root != 0 {
			check("external root width", ext.RootWidth, tc.root*s.Pitch)
			check("internal crest width", in.CrestWidth, s.Pitch/4)
			check("internal root width", in.RootWidth, tc.intRoot*s.Pitch)
		}
		// The ridge of one thread fills the groove of the other at the
		// pitch diameter, and its width follows the flanks.
		for _, p := range []Profile{ext, in} {
			if got := p.WidthAt(s.PitchDiameter() / 2); math.Abs(got-s.Pitch/2) > 1e-9 {
				t.Errorf("%s: width at the pitch diameter %g, want %g", tc.name, got, s.Pitch/2)
			}
			if got := p.WidthAt(p.RootRadius); math.Abs(got-p.RootWidth) > 1e-9 {
				t.Errorf("%s: width at the root %g, want %g", tc.name, got, p.RootWidth)
			}
			if p.RootWidth >= s.Pitch || p.CrestWidth <= 0 {
				t.Errorf("%s: ridge %g wide at the root, %g at the crest", tc.name, p.RootWidth, p.CrestWidth)
			}
		}
	}
}

// TestProfileClearance checks that the clearance moves the profile
// away from the mating thread without changing its shape.
func TestProfileClearance(t *testing.T) {
	t.Parallel()
	s := Spec{Diameter: 8, Pitch: 1.25}
	for _, internal := range []bool{false, true} {
		p, c := s.Profile(internal, 0), s.Profile(internal, 0.1)
		want := -0.1
		if internal {
			want = 0.1
		}
		if d := c.CrestRadius - p.CrestRadius; math.Abs(d-want) > 1e-12 {
			t.Errorf("internal %v: crest moved %g, want %g", internal, d, want)
		}
		if d := c.RootRadius - p.RootRadius; math.Abs(d-want) > 1e-12 {
			t.Errorf("internal %v: root moved %g, want %g", internal, d, want)
		}
		if c.CrestWidth != p.CrestWidth || c.RootWidth != p.RootWidth || math.Abs(c.Height()-p.Height()) > 1e-12 {
			t.Errorf("internal %v: the clearance changed the shape", internal)
		}
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	const in = mmPerInch
	for _, tc := range []struct {
		text string
		want Spec
	}{
		{"M8", Spec{Form: FormMetric, Diameter: 8, Pitch: 1.25}},
		{" m8 ", Spec{Form: FormMetric, Diameter: 8, Pitch: 1.25}},
		{"M8x1", Spec{Form: FormMetric, Diameter: 8, Pitch: 1}},
		{"M8×0.75", Spec{Form: FormMetric, Diameter: 8, Pitch: 0.75}},
		{"M2,5", Spec{Form: FormMetric, Diameter: 2.5, Pitch: 0.45}},
		{"M9x1.25", Spec{Form: FormMetric, Diameter: 9, Pitch: 1.25}},
		{"M8-LH", Spec{Form: FormMetric, Diameter: 8, Pitch: 1.25, LeftHand: true}},
		{"M8x1 lh", Spec{Form: FormMetric, Diameter: 8, Pitch: 1, LeftHand: true}},
		{"M8-RH", Spec{Form: FormMetric, Diameter: 8, Pitch: 1.25}},
		{"M16xPh3P1.5", Spec{Form: FormMetric, Diameter: 16, Pitch: 1.5, Starts: 2}},
		{"M16xPh3P1.5-LH", Spec{Form: FormMetric, Diameter: 16, Pitch: 1.5, Starts: 2, LeftHand: true}},
		{"Tr20", Spec{Form: FormTrapezoidal, Diameter: 20, Pitch: 4}},
		{"Tr20x4", Spec{Form: FormTrapezoidal, Diameter: 20, Pitch: 4}},
		{"TR40x14(P7)", Spec{Form: FormTrapezoidal, Diameter: 40, Pitch: 7, Starts: 2}},
		{"Tr40x14P7-LH", Spec{Form: FormTrapezoidal, Diameter: 40, Pitch: 7, Starts: 2, LeftHand: true}},
		{"1/4-20", Spec{Form: FormUnified, Diameter: in / 4, Pitch: in / 20}},
		{"1/4-20 UNC", Spec{Form: FormUnified, Diameter: in / 4, Pitch: in / 20}},
		{"1/4-20UNC", Spec{Form: FormUnified, Diameter: in / 4, Pitch: in / 20}},
		{"1/4 unf", Spec{Form: FormUnified, Diameter: in / 4, Pitch: in / 28}},
		{"1/4-32 UNEF", Spec{Form: FormUnified, Diameter: in / 4, Pitch: in / 32}},
		{"#10-32", Spec{Form: FormUnified, Diameter: 0.190 * in, Pitch: in / 32}},
		{"#10 UNC", Spec{Form: FormUnified, Diameter: 0.190 * in, Pitch: in / 24}},
		{"1-8", Spec{Form: FormUnified, Diameter: in, Pitch: in / 8}},
		{"1 1/8-7 UNC", Spec{Form: FormUnified, Diameter: 1.125 * in, Pitch: in / 7}},
		{"1-1/8-7", Spec{Form: FormUnified, Diameter: 1.125 * in, Pitch: in / 7}},
		{"1-1/8 UNC", Spec{Form: FormUnified, Diameter: 1.125 * in, Pitch: in / 7}},
		{"2-4.5 UNC", Spec{Form: FormUnified, Diameter: 2 * in, Pitch: in / 4.5}},
		{"0.3-20", Spec{Form: FormUnified, Diameter: 0.3 * in, Pitch: in / 20}},
		{"1/2-13 UNC-LH", Spec{Form: FormUnified, Diameter: in / 2, Pitch: in / 13, LeftHand: true}},
		{"1/2-13 UNC (2 starts)", Spec{Form: FormUnified, Diameter: in / 2, Pitch: in / 13, Starts: 2}},
		{"1/4 BSW", Spec{Form: FormWhitworth, Diameter: in / 4, Pitch: in / 20}},
		{"1/4 bsf", Spec{Form: FormWhitworth, Diameter: in / 4, Pitch: in / 26}},
		{"3/8-20 BSW", Spec{Form: FormWhitworth, Diameter: 3 * in / 8, Pitch: in / 20}},
		{"1 1/2 BSW-LH", Spec{Form: FormWhitworth, Diameter: 1.5 * in, Pitch: in / 6, LeftHand: true}},
		{"G1/2", Spec{Form: FormWhitworth, Diameter: 20.955, Pitch: in / 14}},
		{"g 1 1/2", Spec{Form: FormWhitworth, Diameter: 47.803, Pitch: in / 11}},
		{"G3/4 B", Spec{Form: FormWhitworth, Diameter: 26.441, Pitch: in / 14}},
		{"G1/4 (2 starts)", Spec{Form: FormWhitworth, Diameter: 13.157, Pitch: in / 19, Starts: 2}},
		{"1/2-10 ACME", Spec{Form: FormACME, Diameter: in / 2, Pitch: in / 10}},
		{"1/2 acme", Spec{Form: FormACME, Diameter: in / 2, Pitch: in / 10}},
		{"1/2-10 ACME-2G", Spec{Form: FormACME, Diameter: in / 2, Pitch: in / 10}},
		{"1-4 ACME-3G-LH", Spec{Form: FormACME, Diameter: in, Pitch: in / 4, LeftHand: true}},
		{"3/4-6 ACME (2 starts)", Spec{Form: FormACME, Diameter: 3 * in / 4, Pitch: in / 6, Starts: 2}},
	} {
		got, err := Parse(tc.text)
		if err != nil {
			t.Errorf("%q: %v", tc.text, err)
			continue
		}
		if got.Form != tc.want.Form || math.Abs(got.Diameter-tc.want.Diameter) > 1e-9 ||
			math.Abs(got.Pitch-tc.want.Pitch) > 1e-9 || got.NumStarts() != tc.want.NumStarts() ||
			got.LeftHand != tc.want.LeftHand {
			t.Errorf("%q: got %+v, want %+v", tc.text, got, tc.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"", "M", "Mx1", "M9", "M8x", "M8x0", "M8x-1", "M8x20", "M0", "M-8", "M8xPh3", "M16xPh4P1.5",
		"Tr9", "Tr20x", "Tr20x8(P3)", "bolt", "1/4", "1/0-20", "#3/4", "#99-20", "1/4-0", "1/4-x",
		"3 UNC", "1 3/4 UNF", "1/4-20 (0 starts)", "1/4-20 (two)", "M8LH", "NaN", "MInf",
		"3/32 BSW", "5/32 BSF", "G5/16", "G", "Gx", "G1/2 (x starts)", "11/16 ACME", "1/2-0 ACME",
	} {
		if s, err := Parse(text); err == nil {
			t.Errorf("%q: read as %+v, want an error", text, s)
		}
	}
}

// TestStringRoundTrip checks that a designation reads back as the
// thread it was written for, and is written the way it is usually read.
func TestStringRoundTrip(t *testing.T) {
	t.Parallel()
	for _, text := range []string{
		"M8", "M8x1", "M8-LH", "M9x1.25", "M2.5", "M16xPh3P1.5", "M16xPh4.5P1.5-LH",
		"Tr20x4", "Tr20x4-LH", "Tr40x14(P7)", "Tr9x2",
		"1/4-20 UNC", "1/4-28 UNF", "1/4-32 UN", "#10-32 UNF", "#0-80 UNF", "1 1/8-7 UNC",
		"2-4.5 UNC", "0.3-20 UN", "1/2-13 UNC-LH", "1/2-13 UNC (2 starts)",
		"1/4 BSW", "1/4 BSF", "1 1/8 BSW", "0.3-20 BSW", "1/2-14 BSW", "G1/2", "G1 1/2-LH",
		"G1/4 (2 starts)", "1/2-10 ACME", "1 1/2-6 ACME-LH", "3/4-6 ACME (2 starts)",
	} {
		s, err := Parse(text)
		if err != nil {
			t.Errorf("%q: %v", text, err)
			continue
		}
		if got := s.String(); got != text {
			t.Errorf("%q: written as %q", text, got)
		}
	}
	// Every size of the tables.
	for _, m := range metricCoarse {
		s := Spec{Form: FormMetric, Diameter: m.d, Pitch: m.pitch}
		if got, err := Parse(s.String()); err != nil || got != s {
			t.Errorf("%s: read back as %+v, %v", s, got, err)
		}
	}
	for _, m := range trapezoidal {
		s := Spec{Form: FormTrapezoidal, Diameter: m.d, Pitch: m.pitch}
		if got, err := Parse(s.String()); err != nil || got != s {
			t.Errorf("%s: read back as %+v, %v", s, got, err)
		}
	}
	for _, u := range unified {
		for _, tpi := range []float64{u.unc, u.unf} {
			if tpi == 0 {
				continue
			}
			s := Spec{Form: FormUnified, Diameter: u.d * mmPerInch, Pitch: mmPerInch / tpi}
			got, err := Parse(s.String())
			if err != nil || math.Abs(got.Diameter-s.Diameter) > 1e-9 || math.Abs(got.Pitch-s.Pitch) > 1e-9 {
				t.Errorf("%s: read back as %+v, %v", s, got, err)
			}
		}
	}
}

// TestTablesAreValid checks every standard size for a possible thread,
// and the tables for their order.
func TestTablesAreValid(t *testing.T) {
	t.Parallel()
	for i, m := range metricCoarse {
		if err := (Spec{Diameter: m.d, Pitch: m.pitch}).Validate(); err != nil {
			t.Errorf("M%g: %v", m.d, err)
		}
		if i > 0 && (m.d <= metricCoarse[i-1].d || m.pitch < metricCoarse[i-1].pitch) {
			t.Errorf("M%g is out of order", m.d)
		}
	}
	for i, m := range trapezoidal {
		if err := (Spec{Form: FormTrapezoidal, Diameter: m.d, Pitch: m.pitch}).Validate(); err != nil {
			t.Errorf("Tr%g: %v", m.d, err)
		}
		if i > 0 && (m.d <= trapezoidal[i-1].d || m.pitch < trapezoidal[i-1].pitch) {
			t.Errorf("Tr%g is out of order", m.d)
		}
	}
	for i, u := range unified {
		if i > 0 && (u.d <= unified[i-1].d || u.unc > unified[i-1].unc && unified[i-1].unc != 0 ||
			u.unf > unified[i-1].unf && u.unf != 0) {
			t.Errorf("%s is out of order", u.name)
		}
	}
}

func TestNearest(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		d    float64
		want string
	}{{8, "M8"}, {8.1, "M8"}, {7.2, "M7"}, {0.5, "M1"}, {100, "M64"}, {6.8, "M7"}, {9, "M8"}} {
		if got := Nearest(tc.d).String(); got != tc.want {
			t.Errorf("Nearest(%g) = %s, want %s", tc.d, got, tc.want)
		}
	}
}

func TestNearestForHole(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		d    float64
		want string
	}{{10, "M10"}, {8.5, "M10"}, {8, "M8"}, {6.8, "M8"}, {5, "M5"}, {4.9, "M6"}, {100, "M64"}} {
		if got := NearestForHole(tc.d).String(); got != tc.want {
			t.Errorf("NearestForHole(%g) = %s, want %s", tc.d, got, tc.want)
		}
	}
}
