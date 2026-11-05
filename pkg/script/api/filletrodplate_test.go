package api

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/fixtures"
)

// rodThroughPlate is a plate 12 wide, 30 long and 4 thick with a rod of
// radius 4 through it, joined, and the circle where the rod leaves the
// plate's top.
func rodThroughPlate(d *Doc) (Group, SolidEdge) {
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(V(-6, -15, 0), V(6, 15, 0))
	d.Features.Extrude(4, "plate")
	d.Features.Sketch3D("under")
	d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 0, -6), V(1, 0, 0), V(0, 1, 0)), "")
	d.Sketch.Circle(V(0, 0, -6), 4)
	rod := d.Features.Extrude(16, "rod")
	return rod, rod.Solid().EdgeAt(V(4, 0, 4))
}

// ringPastSides is the volume of the ring a fillet (or chamfer, flat)
// of size r adds round a rod of radius 4 standing on a plane at height
// 4 (up), or takes from a hole's rim of radius 4 there (down), within
// the plate's sides at x = ±6: its cross section integrated round the
// axis, at each radius over the angle within the sides.
func ringPastSides(r float64, flat, down bool) float64 {
	const n = 1000
	h := r / n
	var v float64
	for i := range n {
		rho := 4 + (float64(i)+0.5)*h
		turn := 2 * math.Pi
		if rho > 6 {
			turn -= 4 * math.Acos(6/rho)
		}
		for j := range n {
			s := (float64(j) + 0.5) * h // height above (or depth below) the plane
			dr, ds := rho-(4+r), s-r
			if flat && (rho-4)+s > r || !flat && dr*dr+ds*ds < r*r {
				continue
			}
			v += rho * turn * h * h
		}
	}
	if down {
		return -v
	}
	return v
}

// The closed fillet, or chamfer, round a rod through a plate, where the
// rod leaves the plate's top. Up to a size of 2 its foot on the top, a
// circle of radius 4 + r, lies inside the top. Past 2 it runs past the
// plate's sides, whose planes cut it, the sides grown up by its
// sections, and the top split in two: made by widening the plate, then
// cutting it back (filletPastSides). At 2 the foot just reaches the
// sides and pinches the top to a point at each, two faces touching
// there; sizes within a tenth of the chord tolerance (0.00085 mm) of 2
// are made as 2. It was made open (69 naked edges at a fillet of 2, 256
// past it) until 2026-11-03, then refused within a hundredth of 2 until
// 2026-11-04. A fillet running only just past the sides may still be
// refused (TestFilletRodJustPastSides).
func TestFilletRodThroughPlate(t *testing.T) {
	t.Parallel()
	for _, flat := range []bool{false, true} {
		for _, r := range []float64{1, 1.9, 1.99, 1.999, 1.9999, 2, 2.00001, 2.0001, 2.005, 2.01, 2.1, 2.5, 3.5} {
			what := "fillet"
			if flat {
				what = "chamfer"
			}
			t.Run(fmt.Sprintf("%s %g", what, r), func(t *testing.T) {
				t.Parallel()
				vol, msg := rodPlateRounded(r, flat)
				if msg != "" {
					t.Fatalf("not made: %s", msg)
				}
				want := rodPlateVolume + ringPastSides(r, flat, false)
				if rel := math.Abs(vol-want) / want; rel > 5e-4 {
					t.Errorf("volume %.4f, want %.4f (off by %.1e)", vol, want, rel)
				}
			})
		}
	}
}

// rodPlateVolume is rodThroughPlate's volume.
const rodPlateVolume = 12.0*30*4 + math.Pi*16*16 - math.Pi*16*4

// rodPlateRounded is the volume of rodThroughPlate with the fillet, or
// chamfer, of size r round the rod at the plate's top, at a chord
// tolerance of 0.003; or why it was not made.
func rodPlateRounded(r float64, flat bool) (vol float64, msg string) {
	vol, msg, _ = rodPlateRoundedNote(r, flat)
	return vol, msg
}

// rodPlateRoundedNote is rodPlateRounded and the group's Note.
func rodPlateRoundedNote(r float64, flat bool) (vol float64, msg, notice string) {
	d := New()
	d.Edit.SetChordTolerance(0.003)
	rod, e := rodThroughPlate(d)
	if p := rod.Problem(); p != "" {
		return 0, p, ""
	}
	defer func() {
		if x := recover(); x != nil {
			msg = fmt.Sprint(x)
		}
	}()
	var g Group
	if flat {
		g = d.Features.ChamferSolid([]SolidEdge{e}, r, "")
	} else {
		g = d.Features.FilletSolid([]SolidEdge{e}, r, "")
	}
	if p := g.Problem(); p != "" {
		return 0, p, ""
	}
	if rep := g.Analyze().Check(); !rep.Sound() {
		return 0, "unsound: " + rep.Summary(), ""
	}
	return g.Analyze().Volume(), "", g.Note()
}

// A fillet whose foot runs past the plate's sides by more than a tenth
// of the chord tolerance (0.00085 mm at the setting of 0.003) and less
// than a few times it: its arc stands above the top at the sides' planes
// by about the square of how far past over its diameter, under
// LengthEps, and the cut along them can't tell it from the top. Swept
// 2026-11-04 from 1.997 to 2.006 by 0.0001: of 91 fillets 17 were
// refused, all from 2.0002 to 2.0028, under the CDT as under the ear
// clipper. Since 2026-11-05 each is made at the nearest size the kernel
// can cut that moves its surface by no more than the chord tolerance
// (nearSize): all 91, sound, within the volume of the size asked for.
func TestFilletRodJustPastSides(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("a fillet per size across the band; a minute or more")
	}
	n := 31
	if fixtures.RaceDetector {
		n = 3
	}
	for i := range n {
		r := 2 + 0.0001*float64(i*30/(n-1))
		t.Run(fmt.Sprintf("%.4f", r), func(t *testing.T) {
			t.Parallel()
			vol, msg, notice := rodPlateRoundedNote(r, false)
			if msg != "" {
				t.Fatalf("not made: %s", msg)
			}
			// A size other than the one asked for is said on the group.
			if notice != "" && !strings.Contains(notice, fmt.Sprintf("the nearest the kernel can cut to %s mm",
				strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", r), "0"), "."))) {
				t.Errorf("notice %q does not name the radius asked for", notice)
			}
			want := rodPlateVolume + ringPastSides(r, false, false)
			if rel := math.Abs(vol-want) / want; rel > 5e-4 {
				t.Errorf("volume %.4f, want %.4f (off by %.1e)", vol, want, rel)
			}
		})
	}
}

// A fillet just past the sides that the kernel can't cut at the size
// asked for says on the group what size it was made at; one it can cut
// says nothing.
func TestFilletRodJustPastSidesNote(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("two fillets round a rod through a plate; tens of seconds")
	}
	_, msg, notice := rodPlateRoundedNote(2.001, false)
	if msg != "" {
		t.Fatalf("not made: %s", msg)
	}
	if !strings.HasPrefix(notice, "made at radius ") || !strings.Contains(notice, "the nearest the kernel can cut to 2.001 mm") {
		t.Errorf("notice %q, want the radius made and 2.001 asked for", notice)
	}
	if _, msg, notice := rodPlateRoundedNote(2.5, false); msg != "" || notice != "" {
		t.Errorf("a fillet of 2.5: problem %q, notice %q, want neither", msg, notice)
	}
}

// A fillet just past the sides at a chord setting ten times finer,
// where fewer sizes near it lie within the chord tolerance of the one
// asked for: made sound (it is, 2026-11-05), or refused saying why.
func TestFilletRodJustPastSidesFine(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("a fillet at a fine chord tolerance; tens of seconds")
	}
	d := New()
	d.Edit.SetChordTolerance(0.0003)
	rod, e := rodThroughPlate(d)
	if p := rod.Problem(); p != "" {
		t.Fatal(p)
	}
	msg := func() (msg string) {
		defer func() {
			if x := recover(); x != nil {
				msg = fmt.Sprint(x)
			}
		}()
		g := d.Features.FilletSolid([]SolidEdge{e}, 2.0015, "")
		if p := g.Problem(); p != "" {
			return p
		}
		if rep := g.Analyze().Check(); !rep.Sound() {
			t.Fatalf("made unsound: %s", rep.Summary())
		}
		return ""
	}()
	if msg != "" && !strings.Contains(msg, "only just runs past the side") {
		t.Errorf("refused, but not as only just past the side: %q", msg)
	}
}

// A hole's rim rounded where the plate's sides come closer than the
// fillet's foot: the rounding cut back by the sides' planes, as the
// rod's is.
func TestFilletHoleRimPastSides(t *testing.T) {
	t.Parallel()
	plate := 12.0*30*4 - math.Pi*16*4
	for _, r := range []float64{1.5, 2.5, 3.5} {
		t.Run(fmt.Sprint(r), func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			d.Features.SketchIn(d.Workplane("XY"), "")
			d.Sketch.Rect(V(-6, -15, 0), V(6, 15, 0))
			d.Sketch.Circle(V(0, 0, 0), 4)
			p := d.Features.Extrude(4, "plate")
			g := d.Features.FilletSolid([]SolidEdge{p.Solid().EdgeAt(V(4, 0, 4))}, r, "")
			if pr := g.Problem(); pr != "" {
				t.Fatal(pr)
			}
			if rep := g.Analyze().Check(); !rep.Sound() {
				t.Fatal(rep.Summary())
			}
			want := plate + ringPastSides(r, false, true)
			if vol := g.Analyze().Volume(); math.Abs(vol-want)/want > 5e-4 {
				t.Errorf("volume %.4f, want %.4f", vol, want)
			}
		})
	}
}

// Where the solid goes on past the side the fillet runs into, a tab on
// the plate's side, cutting the widened plate back would cut the tab:
// the fillet is refused as running off the top.
func TestFilletRodPastSideWithTab(t *testing.T) {
	t.Parallel()
	d := New()
	d.Edit.SetChordTolerance(0.003)
	rodThroughPlate(d)
	d.Features.Sketch3D("tab")
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(V(6, 10, 0), V(8, 14, 0))
	tab := d.Features.Extrude(2, "tab")
	msg := func() (msg string) {
		defer func() {
			if x := recover(); x != nil {
				msg = fmt.Sprint(x)
			}
		}()
		return d.Features.FilletSolid([]SolidEdge{tab.Solid().EdgeAt(V(4, 0, 4))}, 2.5, "").Problem()
	}()
	if !strings.Contains(msg, "runs off the face") {
		t.Errorf("not refused as running off the top: %q", msg)
	}
}
