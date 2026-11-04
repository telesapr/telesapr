package api

import (
	"fmt"
	"math"
	"strings"
	"testing"
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
// sides, which would pinch the top to points, and is refused, as a
// size within a hundredth of it may be; it was made open (69 naked
// edges at a fillet of 2, 256 past it) until 2026-11-03, then refused
// at every size past 2.
func TestFilletRodThroughPlate(t *testing.T) {
	t.Parallel()
	plate := 12.0*30*4 + math.Pi*16*16 - math.Pi*16*4
	for _, flat := range []bool{false, true} {
		for _, r := range []float64{1, 1.9, 2, 2.1, 2.5, 3.5} {
			what := "fillet"
			if flat {
				what = "chamfer"
			}
			t.Run(fmt.Sprintf("%s %g", what, r), func(t *testing.T) {
				t.Parallel()
				d := New()
				d.Edit.SetChordTolerance(0.003)
				rod, e := rodThroughPlate(d)
				if p := rod.Problem(); p != "" {
					t.Fatal(p)
				}
				var vol float64
				msg := func() (msg string) {
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
						return p
					}
					if rep := g.Analyze().Check(); !rep.Sound() {
						return "unsound: " + rep.Summary()
					}
					vol = g.Analyze().Volume()
					return ""
				}()
				if r == 2 {
					if !strings.Contains(msg, "just reaches the side") {
						t.Errorf("not refused as just reaching the plate's side: %q", msg)
					}
					return
				}
				if msg != "" {
					t.Fatalf("not made: %s", msg)
				}
				want := plate + ringPastSides(r, flat, false)
				if rel := math.Abs(vol-want) / want; rel > 5e-4 {
					t.Errorf("volume %.4f, want %.4f (off by %.1e)", vol, want, rel)
				}
			})
		}
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
