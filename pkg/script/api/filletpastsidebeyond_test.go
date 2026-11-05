package api

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

// beyondSide is what stands past the plate's side at x = 6 that a
// fillet's foot round the rod or the hole runs into: blocks on the
// plane z = 0 beyond it, from x = 6 out, each a rectangle in y and a
// height.
type beyondSide struct {
	name   string
	blocks []beyondBlock
}

// beyondBlock is a block standing on z = 0 past the side, from x0 to x1
// and y0 to y1, h high.
type beyondBlock struct{ x0, x1, y0, y1, h float64 }

// beyondSides are the solids past the side the sweep tries: a ledge
// the whole length of the side, low, half and high; a ledge along part
// of it; a tab; and ledges past both sides.
var beyondSides = []beyondSide{
	{"ledge 1", []beyondBlock{{6, 10, -15, 15, 1}}},
	{"ledge 2", []beyondBlock{{6, 10, -15, 15, 2}}},
	{"ledge 3", []beyondBlock{{6, 10, -15, 15, 3}}},
	{"part ledge", []beyondBlock{{6, 10, -15, -5, 2}}},
	{"tab", []beyondBlock{{6, 8, 10, 14, 2}}},
	{"both ledges", []beyondBlock{{6, 10, -15, 15, 2}, {-10, -6, -15, 15, 1.5}}},
}

// plateBeyond is a plate 12 wide, 30 long and 4 thick with a rod of
// radius 4 through it (or, hole, a hole of radius 4), the blocks of b
// joined past its sides, the circle where the rod leaves the top (or
// the hole's rim on it), and the solid's volume.
func plateBeyond(d *Doc, b beyondSide, hole bool) (Group, SolidEdge, float64) {
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(V(-6, -15, 0), V(6, 15, 0))
	vol := 12.0 * 30 * 4
	if hole {
		d.Sketch.Circle(V(0, 0, 0), 4)
		vol -= math.Pi * 16 * 4
	}
	g := d.Features.Extrude(4, "plate")
	if !hole {
		d.Features.Sketch3D("under")
		d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 0, -6), V(1, 0, 0), V(0, 1, 0)), "")
		d.Sketch.Circle(V(0, 0, -6), 4)
		g = d.Features.Extrude(16, "rod")
		vol += math.Pi*16*16 - math.Pi*16*4
	}
	for i, k := range b.blocks {
		d.Features.Sketch3D(fmt.Sprintf("beyond %d", i))
		d.Features.SketchIn(d.Workplane("XY"), "")
		d.Sketch.Rect(V(k.x0, k.y0, 0), V(k.x1, k.y1, 0))
		g = d.Features.Extrude(k.h, fmt.Sprintf("block %d", i))
		vol += (k.x1 - k.x0) * (k.y1 - k.y0) * k.h
	}
	return g, g.Solid().EdgeAt(V(4, 0, 4)), vol
}

// A circular fillet or chamfer round a rod through a plate, or a hole's
// rim in it, whose foot runs past the plate's side where the solid goes
// on beyond it, below the side or beside it: a ledge, a tab. The
// fillet is cut back by the side's plane as on a plate with nothing
// past it (filletPastSides), and what goes on past the side is left as
// it was (beyondCutter): where the fillet takes material away and cuts
// into it past the side's plane (a chamfer of 3.5 on the hole's rim
// cuts 1.5 down at the plane, through the riser above the ledge 3
// high), the solid before it is put back there (keepBeyond). Of the 72
// cases all are made sound, their volumes within 5e-4 of the ring's
// integral, as on a plate with nothing past its sides; refused before
// 2026-11-05 as running off the top.
func TestFilletPastSideSolidBeyond(t *testing.T) {
	t.Parallel()
	for _, b := range beyondSides {
		for _, hole := range []bool{false, true} {
			for _, flat := range []bool{false, true} {
				for _, r := range []float64{2.01, 2.5, 3.5} {
					// Under -short, a tab past a rod's fillet and the
					// chamfer that cuts into the ledge past a hole's rim.
					if testing.Short() && !(b.name == "tab" && !hole && !flat && r == 2.5) &&
						!(b.name == "ledge 3" && hole && flat && r == 3.5) {
						continue
					}
					what := map[bool]string{false: "fillet", true: "chamfer"}[flat]
					where := map[bool]string{false: "rod", true: "hole"}[hole]
					t.Run(fmt.Sprintf("%s/%s/%s %g", b.name, where, what, r), func(t *testing.T) {
						t.Parallel()
						checkFilletBeyond(t, b, hole, flat, r)
					})
				}
			}
		}
	}
}

// checkFilletBeyond rounds plateBeyond's edge at size r and checks the
// result is sound and holds the ring within the plate's sides.
func checkFilletBeyond(t *testing.T, b beyondSide, hole, flat bool, r float64) {
	t.Helper()
	d := New()
	d.Edit.SetChordTolerance(0.003)
	base, e, vol := plateBeyond(d, b, hole)
	if p := base.Problem(); p != "" {
		t.Fatal(p)
	}
	msg, g := func() (msg string, g Group) {
		defer func() {
			if x := recover(); x != nil {
				msg = fmt.Sprint(x)
			}
		}()
		if flat {
			g = d.Features.ChamferSolid([]SolidEdge{e}, r, "")
		} else {
			g = d.Features.FilletSolid([]SolidEdge{e}, r, "")
		}
		return g.Problem(), g
	}()
	if msg != "" {
		t.Fatalf("not made: %s", msg)
	}
	if rep := g.Analyze().Check(); !rep.Sound() {
		t.Fatalf("unsound: %s", rep.Summary())
	}
	want := vol + ringPastSides(r, flat, hole)
	if got := g.Analyze().Volume(); math.Abs(got-want)/want > 5e-4 {
		t.Errorf("volume %.4f, want %.4f (off by %.1e)", got, want, math.Abs(got-want)/want)
	}
}

// A side with a hole in it: a cross bore of radius 1 drilled into the
// plate's side at x = 6, 3 deep, away from the rod's fillet, or (near)
// under it. The fillet is cut back by the side's plane as where the
// side has none; the bore's mouth stays open. Refused before 2026-11-05
// as running off the top.
func TestFilletPastSideWithHole(t *testing.T) {
	t.Parallel()
	for _, y := range []float64{10, 2} {
		for _, flat := range []bool{false, true} {
			if testing.Short() && (y != 2 || !flat) {
				continue // the chamfer above the bore, a few seconds
			}
			what := map[bool]string{false: "fillet", true: "chamfer"}[flat]
			t.Run(fmt.Sprintf("bore at %g/%s", y, what), func(t *testing.T) {
				t.Parallel()
				d := New()
				d.Edit.SetChordTolerance(0.003)
				rodThroughPlate(d)
				d.Features.Sketch3D("bore")
				d.Features.SketchIn(d.Sketch.NewWorkplane(V(6, y, 2), V(0, 1, 0), V(0, 0, 1)), "")
				d.Sketch.Circle(V(6, y, 2), 1)
				g := d.Features.Extrude(-3, "bore")
				g.SetCombine(Difference)
				if p := g.Problem(); p != "" {
					t.Fatal(p)
				}
				vol := rodPlateVolume - math.Pi*3
				r := 2.5
				msg, f := func() (msg string, f Group) {
					defer func() {
						if x := recover(); x != nil {
							msg = fmt.Sprint(x)
						}
					}()
					e := g.Solid().EdgeAt(V(4, 0, 4))
					if flat {
						f = d.Features.ChamferSolid([]SolidEdge{e}, r, "")
					} else {
						f = d.Features.FilletSolid([]SolidEdge{e}, r, "")
					}
					return f.Problem(), f
				}()
				if msg != "" {
					t.Fatalf("not made: %s", msg)
				}
				if rep := f.Analyze().Check(); !rep.Sound() {
					t.Fatalf("unsound: %s", rep.Summary())
				}
				want := vol + ringPastSides(r, flat, false)
				if got := f.Analyze().Volume(); math.Abs(got-want)/want > 5e-4 {
					t.Errorf("volume %.4f, want %.4f (off by %.1e)", got, want, math.Abs(got-want)/want)
				}
			})
		}
	}
}

// slantedPlate is rodThroughPlate (or, hole, a plate with a hole of 4)
// whose two long sides lean out going down by k across per unit of
// height (k < 0 leans them in): its top 12 wide at z = 4, its bottom
// 12 + 8k. It is extruded along y from a trapezoid in the XZ plane.
// It gives the edge to round and the solid's volume.
func slantedPlate(d *Doc, k float64, hole bool) (Group, SolidEdge, float64) {
	d.Features.Sketch3D("datum")
	d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 15, 0), V(1, 0, 0), V(0, 0, 1)), "")
	d.Sketch.Polyline(true, V(-6-4*k, 15, 0), V(6+4*k, 15, 0), V(6, 15, 4), V(-6, 15, 4))
	d.Features.Extrude(30, "plate")
	vol := (12 + 4*k) * 4 * 30
	d.Features.Sketch3D("under")
	d.Features.SketchIn(d.Sketch.NewWorkplane(V(0, 0, -6), V(1, 0, 0), V(0, 1, 0)), "")
	d.Sketch.Circle(V(0, 0, -6), 4)
	g := d.Features.Extrude(16, "rod")
	if hole {
		g.SetCombine(Difference)
		vol -= math.Pi * 16 * 4
	} else {
		vol += math.Pi*16*16 - math.Pi*16*4
	}
	return g, g.Solid().EdgeAt(V(4, 0, 4)), vol
}

// ringSlanted is ringPastSides within sides that lean out going down
// by k: above the top (up, a rod's ring) within |x| < 6 - k s at height
// s, below it (a hole's) within 6 + k s at depth s.
func ringSlanted(r float64, flat, down bool, k float64) float64 {
	const n = 1000
	h := r / n
	sign := -1.0
	if down {
		sign = 1
	}
	var v float64
	for i := range n {
		rho := 4 + (float64(i)+0.5)*h
		for j := range n {
			s := (float64(j) + 0.5) * h
			dr, ds := rho-(4+r), s-r
			if flat && (rho-4)+s > r || !flat && dr*dr+ds*ds < r*r {
				continue
			}
			turn := 2 * math.Pi
			if x := 6 + sign*k*s; rho > x {
				turn -= 4 * math.Acos(x/rho)
			}
			v += rho * turn * h * h
		}
	}
	if down {
		return -v
	}
	return v
}

// A circular fillet or chamfer whose foot runs past its flat face into
// sides that lean to it, a plate's long sides drafted out or in going
// down: the plate widened past them along the top, the fillet made,
// and cut back by the sides' own planes, which grow up (or down, round
// a hole) by its sections. Of 32, 28 are made sound within 5e-4 of the
// integral, leaning in by 0.4 to out by 1 (45°). Refused, as running
// off the face: a rod's past sides leaning out by 1, whose planes run
// on above the top into the rod (at x = 6 - (z - 4), through its axis
// at z = 10), so that cutting back by them would cut the rod.
func TestFilletPastSlantedSides(t *testing.T) {
	t.Parallel()
	for _, k := range []float64{-0.4, 0.1, 0.5, 1} {
		for _, hole := range []bool{false, true} {
			for _, flat := range []bool{false, true} {
				for _, r := range []float64{2.5, 3.5} {
					what := map[bool]string{false: "fillet", true: "chamfer"}[flat]
					where := map[bool]string{false: "rod", true: "hole"}[hole]
					if testing.Short() && (k != 0.5 || hole || flat || r != 2.5) {
						continue // a rod's fillet past sides leaning out
					}
					t.Run(fmt.Sprintf("lean %g/%s/%s %g", k, where, what, r), func(t *testing.T) {
						t.Parallel()
						checkFilletSlanted(t, k, hole, flat, r, k == 1 && !hole)
					})
				}
			}
		}
	}
}

// checkFilletSlanted rounds slantedPlate's edge at size r and checks
// the result is sound and holds the ring within the sides.
func checkFilletSlanted(t *testing.T, k float64, hole, flat bool, r float64, refused bool) {
	t.Helper()
	d := New()
	d.Edit.SetChordTolerance(0.003)
	base, e, vol := slantedPlate(d, k, hole)
	if p := base.Problem(); p != "" {
		t.Fatal(p)
	}
	if got := base.Analyze().Volume(); math.Abs(got-vol)/vol > 1e-4 {
		t.Fatalf("plate's volume %.4f, want %.4f", got, vol)
	}
	msg, g := func() (msg string, g Group) {
		defer func() {
			if x := recover(); x != nil {
				msg = fmt.Sprint(x)
			}
		}()
		if flat {
			g = d.Features.ChamferSolid([]SolidEdge{e}, r, "")
		} else {
			g = d.Features.FilletSolid([]SolidEdge{e}, r, "")
		}
		return g.Problem(), g
	}()
	if refused {
		if !strings.Contains(msg, "runs off the face") {
			t.Errorf("not refused as running off the face: %q", msg)
		}
		return
	}
	if msg != "" {
		t.Fatalf("not made: %s", msg)
	}
	if rep := g.Analyze().Check(); !rep.Sound() {
		t.Fatalf("unsound: %s", rep.Summary())
	}
	want := vol + ringSlanted(r, flat, hole, k)
	if got := g.Analyze().Volume(); math.Abs(got-want)/want > 5e-4 {
		t.Errorf("volume %.4f, want %.4f (off by %.1e)", got, want, math.Abs(got-want)/want)
	}
}
