package thread

import (
	"math"
	"testing"
)

// TestCrests checks the shaped crests against the flanks they sit on:
// a truncated crest lies on them a quarter of the height lower, and a
// rounded one's arc touches both, its top at the standard crest.
func TestCrests(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"M8", "Tr20x4", "1/4-20 UNC"} {
		s, err := Parse(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, internal := range []bool{false, true} {
			std := s.Profile(internal, 0)
			toRoot := math.Copysign(1, std.RootRadius-std.CrestRadius)

			tr := std.WithCrest(CrestTruncated)
			if got, want := tr.Height(), 0.75*std.Height(); math.Abs(got-want) > 1e-12 {
				t.Errorf("%s internal %v: truncated height %g, want %g", name, internal, got, want)
			}
			if got, want := tr.CrestWidth, std.WidthAt(tr.CrestRadius); math.Abs(got-want) > 1e-12 {
				t.Errorf("%s internal %v: truncated crest %g wide, flanks %g", name, internal, got, want)
			}
			if tr.RootWidth != std.RootWidth {
				t.Errorf("%s internal %v: the root moved", name, internal)
			}

			ro := std.WithCrest(CrestRounded)
			half := ro.FlankAngle / 2 * math.Pi / 180
			// The arc's centre, its foot on a flank and the flank's
			// direction: the radius to the foot is square to the flank.
			centre := ro.CrestRadius + toRoot*ro.CrestRoundRadius
			foot := [2]float64{ro.RoundFootRadius - centre, ro.CrestWidth / 2}
			if r := math.Hypot(foot[0], foot[1]); math.Abs(r-ro.CrestRoundRadius) > 1e-12 {
				t.Errorf("%s internal %v: foot %g from the centre, radius %g", name, internal, r, ro.CrestRoundRadius)
			}
			flank := [2]float64{toRoot * math.Cos(half), math.Sin(half)}
			if dot := foot[0]*flank[0] + foot[1]*flank[1]; math.Abs(dot) > 1e-12 {
				t.Errorf("%s internal %v: the arc crosses the flank (%g)", name, internal, dot)
			}
			if got, want := ro.CrestWidth, std.WidthAt(ro.RoundFootRadius); math.Abs(got-want) > 1e-12 {
				t.Errorf("%s internal %v: arc's feet %g apart, flanks %g", name, internal, got, want)
			}
			if got, want := ro.WidthAt(ro.RootRadius), std.RootWidth; math.Abs(got-want) > 1e-12 {
				t.Errorf("%s internal %v: rounded root %g wide, want %g", name, internal, got, want)
			}
			if std.WithCrest(CrestStandard) != std {
				t.Errorf("%s internal %v: the standard crest changed the profile", name, internal)
			}
		}
	}
}

func TestParseCrest(t *testing.T) {
	t.Parallel()
	for _, c := range []Crest{CrestStandard, CrestTruncated, CrestRounded} {
		if got, err := ParseCrest(c.String()); err != nil || got != c {
			t.Errorf("%v: %v %v", c, got, err)
		}
	}
	if c, err := ParseCrest(""); err != nil || c != CrestStandard {
		t.Errorf("empty: %v %v", c, err)
	}
	if _, err := ParseCrest("pointed"); err == nil {
		t.Error("pointed crest parsed")
	}
	if s := Crest(7).String(); s != "Crest(7)" {
		t.Errorf("unknown crest %q", s)
	}
}
