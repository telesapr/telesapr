package thread

import (
	"math"
	"slices"
	"strings"
	"testing"
)

// sameSpec reports whether two threads are the same but for the last
// bits of their lengths, as a gauge size read back may be.
func sameSpec(a, b Spec) bool {
	return a.Form == b.Form && math.Abs(a.Diameter-b.Diameter) < 1e-9 && math.Abs(a.Pitch-b.Pitch) < 1e-9 &&
		a.NumStarts() == b.NumStarts() && a.LeftHand == b.LeftHand
}

// TestSeriesSizes checks that every size and every pitch of each
// series can be made, reads back from its designation as itself, and
// belongs to its series, smallest size and coarsest pitch first.
func TestSeriesSizes(t *testing.T) {
	t.Parallel()
	for _, sr := range AllSeries() {
		sizes := sr.Sizes()
		if len(sizes) < 15 {
			t.Errorf("%s: only %d sizes", sr, len(sizes))
		}
		for i, s := range sizes {
			if i > 0 && s.Diameter <= sizes[i-1].Diameter {
				t.Errorf("%s: %s is out of order", sr, s)
			}
			if SeriesOf(s) != sr {
				t.Errorf("%s: %s is of the %s series", sr, s, SeriesOf(s))
			}
			pitches := Pitches(s)
			if len(pitches) == 0 || !slices.ContainsFunc(pitches, func(p Spec) bool { return sameSpec(p, s) }) {
				t.Errorf("%s: %s is not among its pitches %v", sr, s, pitches)
			}
			for j, p := range pitches {
				if err := p.Validate(); err != nil {
					t.Errorf("%s: %v", p, err)
				}
				if j > 0 && p.Pitch >= pitches[j-1].Pitch {
					t.Errorf("%s: pitches %v are out of order", s, pitches)
				}
				if got, err := Parse(p.String()); err != nil || !sameSpec(got, p) {
					t.Errorf("%s: read back as %+v, %v", p, got, err)
				}
			}
		}
	}
}

func TestPitches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ size, want string }{
		{"M8", "M8 M8x1 M8x0.75"},
		{"M8x1-LH", "M8-LH M8x1-LH M8x0.75-LH"},
		{"M10", "M10 M10x1.25 M10x1 M10x0.75"},
		{"M9x1", ""},
		{"Tr22", "Tr22x8 Tr22x5 Tr22x3"},
		{"Tr8", "Tr8x1.5"},
		{"1/4 UNC", "1/4-20 UNC 1/4-28 UNF"},
		{"#0 UNF", "#0-80 UNF"},
		{"1/4 BSW", "1/4 BSW 1/4 BSF"},
		{"7/32 BSF", "7/32 BSF"},
		{"G1/2", "G1/2"},
		{"1/2 ACME (2 starts)", "1/2-10 ACME (2 starts)"},
		{"0.3-20 BSW", ""},
	} {
		s, err := Parse(tc.size)
		if err != nil {
			t.Fatalf("%s: %v", tc.size, err)
		}
		var got []string
		for _, p := range Pitches(s) {
			got = append(got, p.String())
		}
		if strings.Join(got, " ") != tc.want {
			t.Errorf("Pitches(%s) = %v, want %s", tc.size, got, tc.want)
		}
	}
}

func TestSeriesNames(t *testing.T) {
	t.Parallel()
	for _, sr := range AllSeries() {
		if got, err := ParseSeries(strings.ToUpper(sr.String())); err != nil || got != sr {
			t.Errorf("%s: read back as %v, %v", sr, got, err)
		}
	}
	if _, err := ParseSeries("BSPT"); err == nil {
		t.Error("BSPT is read as a series")
	}
	if got := Series(99).String(); got != "Series(99)" {
		t.Errorf("Series(99) is %q", got)
	}
}

// TestUnknownSizeHints checks that a size the series lacks is refused
// with a pitch that makes a thread and the nearest standard size.
func TestUnknownSizeHints(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ text, try, near string }{
		{"M7.5", "M7.5x1", "M7"},
		{"M0.5", "M0.5x0.25", "M1"},
		{"M100", "M100x6", "M64"},
		{"Tr21", "Tr21x4", "Tr20x4"},
		{"Tr23", "Tr23x5", "Tr22x5"},
	} {
		_, err := Parse(tc.text)
		if err == nil {
			t.Fatalf("%s: read", tc.text)
		}
		want := "give its pitch, as in " + tc.try + ", or take the nearest, " + tc.near
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", tc.text, err, want)
		}
		for _, hint := range []string{tc.try, tc.near} {
			if _, err := Parse(hint); err != nil {
				t.Errorf("%s: the hint %s is refused: %v", tc.text, hint, err)
			}
		}
	}
}

func TestNote(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ text, want string }{
		{"M8", ""},
		{" m8 lh", ""},
		{"M8x1.25", "'M8x1.25' is written M8"},
		{"M7x1", "'M7x1' is written M7"},
		{"Tr20", "'Tr20' is written Tr20x4"},
		{"1/2-10 ACME-2G", "'1/2-10 ACME-2G' is written 1/2-10 ACME"},
		{"G1/2", ""},
	} {
		s, err := Parse(tc.text)
		if err != nil {
			t.Fatalf("%s: %v", tc.text, err)
		}
		if got := Note(tc.text, s); got != tc.want {
			t.Errorf("Note(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

// TestNewFormsProfiles checks the Whitworth and ACME ridges: their
// flank angles, and that a ridge fills its mate's groove.
func TestNewFormsProfiles(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		text  string
		flank float64
	}{{"1/2 BSW", 55}, {"G1", 55}, {"1-5 ACME", 29}} {
		s, err := Parse(tc.text)
		if err != nil {
			t.Fatal(err)
		}
		if s.FlankAngle() != tc.flank {
			t.Errorf("%s: flanks at %g", tc.text, s.FlankAngle())
		}
		ext, in := s.Profile(false, 0), s.Profile(true, 0)
		if math.Abs(ext.WidthAt(in.CrestRadius)+in.WidthAt(in.CrestRadius)-s.Pitch) > 1e-9 {
			t.Errorf("%s: the ridges don't fill each other's grooves", tc.text)
		}
	}
	if err := (Spec{Form: Form(9), Diameter: 8, Pitch: 1}).Validate(); err == nil {
		t.Error("an unknown form is valid")
	}
}
