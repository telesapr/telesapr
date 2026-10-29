package thread

import (
	"math"
	"slices"
	"testing"
)

// TestButtressDimensions checks the buttress profiles against DIN 513's
// and ASME B1.9's figures: the diameters, the crests' widths (e and F),
// the flanks' angles and the screw's ridge filling the nut's groove.
func TestButtressDimensions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                            string
		pitchD, minor, extMinor, intMaj float64
		crest                           float64 // in pitches
		load, trailing                  float64
	}{
		// DIN 513: d2 = d - 0.75 P, D1 = d - 1.5 P, d3 = d - 1.73554 P.
		{"S20x4", 17, 14, 20 - 1.73554*4, 20, 0.26384, 3, 30},
		{"S48x8", 42, 36, 48 - 1.73554*8, 48, 0.26384, 3, 30},
		// ASME B1.9: E = D - 0.6 p, K_n = D - 1.2 p, K_s = D - 1.32542 p,
		// the nut's major diameter D + 0.12542 p.
		{"1-12 BUTT", 25.4 - 0.6*25.4/12, 25.4 - 1.2*25.4/12, 25.4 - 1.32542*25.4/12, 25.4 + 0.12542*25.4/12, 0.16316, 7, 45},
	} {
		s, err := Parse(tc.name)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		check := func(what string, got, want, tol float64) {
			t.Helper()
			if math.Abs(got-want) > tol {
				t.Errorf("%s: %s %.6g, want %.6g", tc.name, what, got, want)
			}
		}
		check("pitch diameter", s.PitchDiameter(), tc.pitchD, 1e-9)
		check("minor diameter", s.MinorDiameter(), tc.minor, 1e-9)
		check("external minor diameter", s.ExternalMinorDiameter(), tc.extMinor, 1e-9)
		check("internal major diameter", s.InternalMajorDiameter(), tc.intMaj, 1e-9)
		check("flank angle", s.FlankAngle(), tc.load+tc.trailing, 0)
		if !s.Buttress() {
			t.Errorf("%s: not a buttress thread", tc.name)
		}

		ext, in := s.Profile(false, 0), s.Profile(true, 0)
		check("external crest width", ext.CrestWidth, tc.crest*s.Pitch, 2e-5*s.Pitch)
		check("internal crest width", in.CrestWidth, tc.crest*s.Pitch, 2e-5*s.Pitch)
		// The screw's load flank faces along the thread; the nut's
		// faces back, onto it.
		check("external upper flank", ext.UpperFlank, tc.load, 0)
		check("internal upper flank", in.UpperFlank, tc.trailing, 0)
		if ext.Symmetric() || in.Symmetric() {
			t.Errorf("%s: a buttress ridge is symmetric", tc.name)
		}
		pitchR := s.PitchDiameter() / 2
		for _, p := range []Profile{ext, in} {
			check("width at the pitch diameter", p.WidthAt(pitchR), s.Pitch/2, 1e-9)
			check("width at the root", p.WidthAt(p.RootRadius), p.RootWidth, 1e-9)
			if p.RootWidth >= s.Pitch || p.CrestWidth <= 0 {
				t.Errorf("%s: ridge %g wide at the root, %g at the crest", tc.name, p.RootWidth, p.CrestWidth)
			}
		}
		// Where both ridges reach, the nut's lies in the screw's groove:
		// the screw's ridge at height z spans [lo, hi], the nut's, half a
		// pitch on at the pitch line, spans [hi, lo + P] at every radius.
		for _, r := range []float64{in.CrestRadius, pitchR, ext.CrestRadius} {
			eLo, eHi := ridgeSpan(ext, r, 0)
			nutMid := ext.MiddleShift(pitchR) - in.MiddleShift(pitchR) + s.Pitch/2
			nLo, nHi := ridgeSpan(in, r, nutMid)
			check("nut's lower flank on the screw's upper", nLo, eHi, 1e-9)
			check("nut's upper flank on the next ridge's lower", nHi, eLo+s.Pitch, 1e-9)
		}
	}
}

// ridgeSpan is where along the axis the ridge p, its crest's middle at
// z, lies at radius r.
func ridgeSpan(p Profile, r, z float64) (lo, hi float64) {
	mid, half := z+p.MiddleShift(r), p.WidthAt(r)/2
	return mid - half, mid + half
}

// TestSymmetricMiddleShift checks that a symmetric ridge's middle is
// its crest's at every radius.
func TestSymmetricMiddleShift(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"M8", "Tr20x4", "1/2-10 ACME", "1/4 BSW"} {
		s, _ := Parse(name)
		p := s.Profile(false, 0)
		if !p.Symmetric() || p.MiddleShift(p.RootRadius) != 0 {
			t.Errorf("%s: shifted %g at the root", name, p.MiddleShift(p.RootRadius))
		}
	}
}

// TestButtressDesignations reads and writes the buttress designations.
func TestButtressDesignations(t *testing.T) {
	t.Parallel()
	const in = mmPerInch
	for _, tc := range []struct {
		text, written string
		want          Spec
	}{
		{"S20x4", "S20x4", Spec{Form: FormButtress, Diameter: 20, Pitch: 4}},
		{"s20", "S20x4", Spec{Form: FormButtress, Diameter: 20, Pitch: 4}},
		{"S 40 x 14 (P7)-LH", "S40x14(P7)-LH", Spec{Form: FormButtress, Diameter: 40, Pitch: 7, Starts: 2, LeftHand: true}},
		{"S25x3", "S25x3", Spec{Form: FormButtress, Diameter: 25, Pitch: 3}},
		{"1-12 BUTT", "1-12 BUTT", Spec{Form: FormButtressInch, Diameter: in, Pitch: in / 12}},
		{"1 butt", "1-12 BUTT", Spec{Form: FormButtressInch, Diameter: in, Pitch: in / 12}},
		{"2-1/2-8 BUTT (2 starts)", "2 1/2-8 BUTT (2 starts)", Spec{Form: FormButtressInch, Diameter: 2.5 * in, Pitch: in / 8, Starts: 2}},
		{"0.6-20 BUTT", "0.6-20 BUTT", Spec{Form: FormButtressInch, Diameter: 0.6 * in, Pitch: in / 20}},
		{"1-12 PUSH-BUTT", "1-12 PUSH-BUTT", Spec{Form: FormButtressInch, Diameter: in, Pitch: in / 12, Load: LoadPush}},
		{"1-12 pull butt-LH", "1-12 PULL-BUTT-LH", Spec{Form: FormButtressInch, Diameter: in, Pitch: in / 12, Load: LoadPull, LeftHand: true}},
		{"1 PULL-BUTT", "1-12 PULL-BUTT", Spec{Form: FormButtressInch, Diameter: in, Pitch: in / 12, Load: LoadPull}},
	} {
		got, err := Parse(tc.text)
		if err != nil {
			t.Errorf("%q: %v", tc.text, err)
			continue
		}
		if math.Abs(got.Diameter-tc.want.Diameter) > 1e-9 || math.Abs(got.Pitch-tc.want.Pitch) > 1e-9 ||
			got.Form != tc.want.Form || got.Starts != tc.want.Starts || got.LeftHand != tc.want.LeftHand || got.Load != tc.want.Load {
			t.Errorf("%q: %+v, want %+v", tc.text, got, tc.want)
		}
		if s := got.String(); s != tc.written {
			t.Errorf("%q written %q, want %q", tc.text, s, tc.written)
		}
		if back, err := Parse(got.String()); err != nil || back != got {
			t.Errorf("%q: %q reads back as %+v, %v", tc.text, got.String(), back, err)
		}
	}
	for _, bad := range []string{"S21", "5 BUTT", "S", "1-12 PUSH", "1-12 PUSH-ACME"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q read", bad)
		}
	}
}

// TestButtressSeries checks the buttress series' sizes and pitches.
func TestButtressSeries(t *testing.T) {
	t.Parallel()
	sizes := SeriesButtress.Sizes()
	if len(sizes) == 0 || sizes[0].String() != "S10x2" {
		t.Errorf("buttress sizes begin %v", sizes)
	}
	var names []string
	for _, p := range Pitches(Spec{Form: FormButtress, Diameter: 30, Pitch: 6}) {
		names = append(names, p.String())
	}
	if want := []string{"S30x10", "S30x6", "S30x3"}; !slices.Equal(names, want) {
		t.Errorf("S30's pitches %v, want %v", names, want)
	}
	inch := SeriesButtressInch.Sizes()
	if len(inch) == 0 || inch[0].String() != "1/2-16 BUTT" {
		t.Errorf("buttress inch sizes begin %v", inch)
	}
	names = nil
	for _, p := range Pitches(Spec{Form: FormButtressInch, Diameter: mmPerInch, Pitch: mmPerInch / 12}) {
		names = append(names, p.String())
	}
	if want := []string{"1-10 BUTT", "1-12 BUTT", "1-16 BUTT"}; !slices.Equal(names, want) {
		t.Errorf("1 BUTT's pitches %v, want %v", names, want)
	}
	if got := Pitches(Spec{Form: FormButtressInch, Diameter: 5 * mmPerInch, Pitch: 4}); len(got) != 0 {
		t.Errorf("5 inch buttress has pitches %v", got)
	}
	if SeriesOf(sizes[0]) != SeriesButtress || SeriesOf(inch[0]) != SeriesButtressInch {
		t.Error("buttress sizes are of another series")
	}
	if Classes(sizes[0], false) != nil || DefaultClass(inch[0], true) != "" {
		t.Error("a buttress thread has classes")
	}
	if _, err := ClassClearance(sizes[0], false, "7e"); err == nil {
		t.Error("a buttress thread took a class")
	}
}

// TestButtressTurned checks that turning a buttress ridge swaps its
// flanks and keeps its widths, that its pitch radius moves with the
// clearance, and that rounding its crest leaves its flanks.
func TestButtressTurned(t *testing.T) {
	t.Parallel()
	s, _ := Parse("S20x4")
	p := s.Profile(false, 0.1)
	if got, want := p.PitchRadius, s.PitchDiameter()/2-0.1; math.Abs(got-want) > 1e-12 {
		t.Errorf("pitch radius %g, want %g", got, want)
	}
	q := p.Turned()
	if q.UpperFlank != 30 || q.CrestWidth != p.CrestWidth || q.WidthAt(q.RootRadius) != p.WidthAt(p.RootRadius) {
		t.Errorf("turned: %+v from %+v", q, p)
	}
	if q.MiddleShift(q.RootRadius) != -p.MiddleShift(p.RootRadius) || p.MiddleShift(p.RootRadius) >= 0 {
		t.Errorf("turned ridge leans %g, from %g", q.MiddleShift(q.RootRadius), p.MiddleShift(p.RootRadius))
	}
	if q.Turned() != p {
		t.Error("turned twice isn't the ridge")
	}
	m, _ := Parse("M8")
	if mp := m.Profile(true, 0); mp.Turned() != mp {
		t.Error("a symmetric ridge turned changed")
	}
	if r := p.WithCrest(CrestRounded); r.CrestWidth != p.CrestWidth || r.UpperFlank != p.UpperFlank ||
		r.WidthAt(r.RootRadius) != p.WidthAt(p.RootRadius) || r.MiddleShift(r.RootRadius) != p.MiddleShift(p.RootRadius) {
		t.Errorf("a buttress crest rounded moved its flanks: %+v", r)
	}
	if tr := p.WithCrest(CrestTruncated); !(tr.CrestWidth > p.CrestWidth) || tr.UpperFlank != p.UpperFlank {
		t.Errorf("a buttress crest truncated: %+v", tr)
	}
}

// TestButtressRootArcs checks B1.9's rounded roots: each arc is of the
// standard's radius, tangent to its flank, crosses the root into the
// part and lies close to the standard's arc, the circle tangent to both
// flanks of the groove whose bottom is at the root; DIN 513's roots are
// flat.
func TestButtressRootArcs(t *testing.T) {
	t.Parallel()
	s, _ := Parse("1-12 BUTT")
	if r := (Spec{Form: FormButtress, Diameter: 20, Pitch: 4}).RootRound(); r != 0 {
		t.Errorf("DIN 513's root rounded by %g", r)
	}
	if p := s.Profile(false, 0); p.RootRoundRadius != 0 {
		t.Errorf("a profile's root rounded by %g", p.RootRoundRadius)
	}
	round := func(p Profile) Profile {
		p.RootRoundRadius = s.RootRound()
		return p
	}
	dist := func(a, b [2]float64) float64 { return math.Hypot(a[0]-b[0], a[1]-b[1]) }
	for _, p := range []Profile{round(s.Profile(false, 0)), round(s.Profile(true, 0.05)), round(s.Profile(false, 0).Turned()),
		round(s.Profile(true, 0)).WithCrest(CrestTruncated)} {
		r := p.RootRoundRadius
		if math.Abs(r-0.07141*s.Pitch) > 1e-12 {
			t.Fatalf("root radius %g", r)
		}
		u := math.Copysign(1, p.CrestRadius-p.RootRadius)
		lower, upper := p.RootArcs()
		// flank is where the flank of side way crosses radius rr, from the
		// crest's middle.
		flank := func(way, rr float64) float64 {
			lo, hi := ridgeSpan(p, rr, 0)
			if way > 0 {
				return hi
			}
			return lo
		}
		for i, a := range []RootArc{lower, upper} {
			way := float64(2*i - 1)
			for what, pt := range map[string][2]float64{"tangent": a.Tangent, "turn": a.Turn} {
				if d := dist(pt, a.Centre); math.Abs(d-r) > 1e-12 {
					t.Errorf("%s %d: %s %g from the centre, want %g", s, i, what, d, r)
				}
			}
			if z := flank(way, a.Tangent[0]); math.Abs(z-a.Tangent[1]) > 1e-12 {
				t.Errorf("arc %d leaves the flank at %g, which is at %g", i, a.Tangent[1], z)
			}
			// Tangent: the radius to the flank is square to it.
			dr, dz := a.Tangent[0]-a.Centre[0], a.Tangent[1]-a.Centre[1]
			fr, fz := 1.0, flank(way, a.Tangent[0]+1)-a.Tangent[1]
			if c := (dr*fr + dz*fz) / (math.Hypot(dr, dz) * math.Hypot(fr, fz)); math.Abs(c) > 1e-9 {
				t.Errorf("arc %d meets its flank at cos %g", i, c)
			}
			if a.Cross[0] != p.RootRadius || u*(a.End[0]-p.RootRadius) >= 0 || u*(a.Turn[0]-p.RootRadius) <= 0 ||
				u*(a.Tangent[0]-a.Turn[0]) <= 0 {
				t.Errorf("arc %d: %+v doesn't run from the flank across the root at %g", i, a, p.RootRadius)
			}
			// Turn, Cross and End lie on the arc's tangent at Turn.
			tr, tz := a.Turn[0]-a.Centre[0], a.Turn[1]-a.Centre[1]
			for what, pt := range map[string][2]float64{"crossing": a.Cross, "end": a.End} {
				if c := (pt[0]-a.Turn[0])*tr + (pt[1]-a.Turn[1])*tz; math.Abs(c) > 1e-12 {
					t.Errorf("arc %d: %s off its tangent by %g", i, what, c)
				}
			}
			// The model keeps within a quarter of the radius of the
			// standard's arc, which the crossing is furthest from.
			if d := dist(a.Cross, a.Centre) - r; d < 0 || d > 0.26*r {
				t.Errorf("arc %d crosses the root %g of the radius off the arc", i, d/r)
			}
			if z := flank(way, p.RootRadius); math.Abs(z-a.Foot[1]) > 1e-12 || way*(a.Cross[1]-a.Foot[1]) <= 0 {
				t.Errorf("arc %d crosses the root at %g, its flank at %g", i, a.Cross[1], z)
			}
		}
		// The standard's arc of the groove above the ridge is tangent to
		// both flanks, its bottom at the root: the centres of the arcs
		// either side of the groove are one.
		if d := dist([2]float64{lower.Centre[0], lower.Centre[1] + s.Pitch}, upper.Centre); d > 2e-5*s.Pitch {
			t.Errorf("the groove's arcs are %g apart", d)
		}
		if c := upper.Centre[0] - u*r; math.Abs(c-p.RootRadius) > 1e-12 {
			t.Errorf("the arc's bottom at %g, the root at %g", c, p.RootRadius)
		}
		if !(upper.End[1] < lower.End[1]+s.Pitch) {
			t.Errorf("the root arcs' ends cross: %g past %g", upper.End[1], lower.End[1]+s.Pitch)
		}
	}
}
