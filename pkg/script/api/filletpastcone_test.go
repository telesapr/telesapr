package api

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// conePlate is a solid on z = 0, its top at z = 4, whose top a circular
// fillet's foot runs off into a side on a cone (a cylinder at a draft of
// 0), and the circle on the top it rounds.
type conePlate struct {
	name string
	// drafts are the drafts the sweep tries, in degrees, and sizes the
	// fillets' sizes.
	drafts, sizes []float64
	// make draws the solid at a draft and gives its last group and
	// volume.
	make func(d *Doc, draft float64) (Group, float64)
	// c and rho are the rounded circle's centre and radius; hole is set
	// where it is a hole's rim, not a rod's foot.
	c    Vec
	rho  float64
	hole bool
	// rim is the side's radius about the z axis at height z, at a draft;
	// the solid lies inside it, or outside it where bore is set.
	rim  func(z, draft float64) float64
	bore bool
}

// tanDeg is the tangent of deg degrees.
func tanDeg(deg float64) float64 { return math.Tan(deg * math.Pi / 180) }

// frustum is the volume of a cone's frustum of height h and radii a, b.
func frustum(a, b, h float64) float64 { return math.Pi * h * (a*a + a*b + b*b) / 3 }

// draftedDisc draws a disc of radius 8 on z = 0, 4 high, its side
// leaning in toward the top by draft degrees, and gives its volume.
func draftedDisc(d *Doc, draft float64) float64 {
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(V(0, 0, 0), 8)
	d.Features.Extrude(4, "disc").AsExtrude().SetDraft(draft)
	return frustum(8, 8-4*tanDeg(draft), 4)
}

// ledgeUnder draws a ledge of radius 10, 1 high, under a drafted disc
// drawn after it, and gives the volume the ledge adds.
func ledgeUnder(d *Doc, draft float64) float64 {
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(V(0, 0, 0), 10)
	d.Features.Extrude(1, "ledge")
	d.Features.Sketch3D("disc")
	return math.Pi*100 - frustum(8, 8-tanDeg(draft), 1)
}

// holeIn cuts a hole of radius 4 at (2, 0) through the solid, 4 high,
// and gives its last group and the volume it takes.
func holeIn(d *Doc) (Group, float64) { return holeAt(d, 2) }

// holeAt is holeIn with the hole at (x, 0).
func holeAt(d *Doc, x float64) (Group, float64) {
	d.Features.Sketch3D("hole")
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(V(x, 0, 0), 4)
	g := d.Features.Extrude(4, "hole")
	g.SetCombine(Difference)
	return g, math.Pi * 16 * 4
}

// discRim is the drafted disc's radius at height z.
func discRim(z, draft float64) float64 { return 8 - z*tanDeg(draft) }

// boreRim is the drafted bore's radius at height z.
func boreRim(z, draft float64) float64 { return 8 + z*tanDeg(draft) }

// conePlates are the solids the sweep tries: a disc of radius 8 drafted
// in or out with a rod of radius 4 off its centre, or a hole of radius
// 4, whose foot runs past the disc's rim; a block whose bore of radius
// 8 widens toward the top, beside a rod of radius 3, also so close that
// the bore's cone carried on above the top runs into it; and the drafted or
// upright disc standing on a wider ledge, the solid going on below the
// side.
var conePlates = []conePlate{
	{
		name: "disc rod", c: V(2, 0, 4), rho: 4, rim: discRim,
		drafts: []float64{5, -5, 10}, sizes: []float64{2.2, 2.5, 3, 3.5},
		make: func(d *Doc, draft float64) (Group, float64) {
			v := draftedDisc(d, draft)
			g, rod := rodUp(d, 2, 0, 4)
			return g, v + rod
		},
	},
	{
		name: "disc hole", c: V(2, 0, 4), rho: 4, hole: true, rim: discRim,
		drafts: []float64{5, 10}, sizes: []float64{2.2, 2.5, 3, 3.5},
		make: func(d *Doc, draft float64) (Group, float64) {
			v := draftedDisc(d, draft)
			g, hole := holeIn(d)
			return g, v - hole
		},
	},
	{
		name: "bore rod", c: V(13, 0, 4), rho: 3, rim: boreRim, bore: true,
		drafts: []float64{5, 10}, sizes: []float64{2.2, 2.5, 3, 3.5},
		make: func(d *Doc, draft float64) (Group, float64) {
			g, v := draftedBore(d, draft, 13)
			return g, v
		},
	},
	{
		// The rod 9 from the axis at its nearest, the bore's cone,
		// widening at 10°, 9 across 1.67 above the face: carried on, it
		// runs into the rod, which the cutter is cut back by
		// (curvedSide.lessAbove).
		name: "bore rod close", c: V(12, 0, 4), rho: 3, rim: boreRim, bore: true,
		drafts: []float64{10}, sizes: []float64{2.2, 2.5, 3, 3.5},
		make: func(d *Doc, draft float64) (Group, float64) {
			g, v := draftedBore(d, draft, 12)
			return g, v
		},
	},
	{
		name: "ledge rod", c: V(2, 0, 4), rho: 4, rim: discRim,
		drafts: []float64{0, 5}, sizes: []float64{2.2, 2.5, 3, 3.5},
		make: func(d *Doc, draft float64) (Group, float64) {
			ledge := ledgeUnder(d, draft)
			v := draftedDisc(d, draft)
			g, rod := rodUp(d, 2, 0, 4)
			return g, ledge + v + rod
		},
	},
	{
		// A hole's rim taking material away down to 2.2 to 3.5 below
		// the top, into the ledge 3 below it for the larger, but within
		// the disc's side there (curvedSide.volumePastBelow).
		name: "ledge hole", c: V(2, 0, 4), rho: 4, hole: true, rim: discRim,
		drafts: []float64{0, 5}, sizes: []float64{2.2, 2.5, 3, 3.5},
		make: func(d *Doc, draft float64) (Group, float64) {
			ledge := ledgeUnder(d, draft)
			v := draftedDisc(d, draft)
			g, hole := holeIn(d)
			return g, ledge + v - hole
		},
	},
	{
		// A hole 0.3 from the disc's rim: the slab's wall toward the
		// solid, inset by 0.4, filled the hole's sliver past it, and the
		// slab was refused (curvedSide.clearance, 2026-11-19). The chamfer
		// of 3.5, whose cut into the ledge reaches past the side, is
		// refused (TestFilletPastConeSideRefused).
		name: "ledge hole near", c: V(3.7, 0, 4), rho: 4, hole: true, rim: discRim,
		drafts: []float64{0}, sizes: []float64{2.2, 2.5, 3},
		make: func(d *Doc, draft float64) (Group, float64) {
			ledge := ledgeUnder(d, draft)
			v := draftedDisc(d, draft)
			g, hole := holeAt(d, 3.7)
			return g, ledge + v - hole
		},
	},
	// Holes 0.5 and 0.8 from the rim of a disc drafted 5° at its top,
	// rounded 3.5: a face of the slab's outer wall was kept by the cut
	// back, the raycast taking a point inside the cutter for outside, its
	// ray crossing the cap just inside the far arc of 156°, whose 32
	// chords sag past the chord tolerance (srf.curvedClearance,
	// 2026-11-19).
	ledgeHoleAt("ledge hole drafted 0.5", 5, 0.5),
	// The hole 0.5 from an upright disc's rim, the chamfer's cone
	// meeting the ledge's top in a circle that touches the foot's: made
	// where the circles cross, at a size within the chord tolerance
	// (topo.footClearSize, 2026-11-19). The Booleans fail where the
	// circles touch exactly; nearer than that 0.0005 they are made at
	// the size asked for (TestFilletPastConeSideNearFoot).
	ledgeHoleAt("ledge hole touching", 0, 0.5),
	ledgeHoleAt("ledge hole drafted 0.8", 5, 0.8),
}

// ledgeHoleAt is the ledged disc, drafted by draft, with a hole of
// radius 4 gap from its rim at the top, rounded 3.5.
func ledgeHoleAt(name string, draft, gap float64) conePlate {
	x := discRim(4, draft) - 4 - gap
	return conePlate{
		name: name, c: V(x, 0, 4), rho: 4, hole: true, rim: discRim,
		drafts: []float64{draft}, sizes: []float64{3.5},
		make: func(d *Doc, draft float64) (Group, float64) {
			ledge := ledgeUnder(d, draft)
			v := draftedDisc(d, draft)
			g, hole := holeAt(d, x)
			return g, ledge + v - hole
		},
	}
}

// draftedBore draws a block 40 square and 4 high with a bore of radius 8
// at its centre widening toward the top by draft degrees, and a rod of
// radius 3 at (x, 0); and gives its last group and volume.
func draftedBore(d *Doc, draft, x float64) (Group, float64) {
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Rect(V(-20, -20, 0), V(20, 20, 0))
	d.Features.Extrude(4, "block")
	d.Features.Sketch3D("bore")
	d.Features.SketchIn(d.Workplane("XY"), "")
	d.Sketch.Circle(V(0, 0, 0), 8)
	// A cut leaning out of the solid widens toward its far end.
	bore := d.Features.Extrude(4, "bore")
	bore.SetCombine(Difference)
	bore.AsExtrude().SetDraft(-draft)
	g, rod := rodUp(d, x, 0, 3)
	return g, 1600*4 - frustum(8, boreRim(4, draft), 4) + rod
}

// ringPastCone is the volume of the ring a fillet (or chamfer, flat) of
// size r adds round a circle of radius rho centred at (cx, cy) on the
// plane z = 4 (a rod's foot), or takes from it below (a hole's rim),
// within the solid: its cross section integrated round the axis, at each
// radius and angle over the heights at which the point lies on the
// solid's side of the side's cone, rim giving its radius about the z
// axis at a height and bore set where the solid lies outside it. The
// cone's radius is linear in the height, so those heights are an
// interval, found exactly.
func ringPastCone(cx, cy, rho, r float64, flat, below bool, rim func(z float64) float64, bore bool) float64 {
	const n, m = 2000, 1440
	h := r / n
	k := rim(1) - rim(0)
	var v float64
	for i := range n {
		d := rho + (float64(i)+0.5)*h
		// The section's height at d: under the chamfer's line, or
		// outside the fillet's circle about (rho + r, r).
		s := r - (d - rho)
		if !flat {
			dr := d - (rho + r)
			s = r - math.Sqrt(r*r-dr*dr)
		}
		z0, z1 := 4.0, 4+s
		if below {
			z0, z1 = 4-s, 4
		}
		var area float64
		for j := range m {
			phi := (float64(j) + 0.5) / m * 2 * math.Pi
			q := math.Hypot(cx+d*math.Cos(phi), cy+d*math.Sin(phi))
			// Inside where f(z) = rim(z) - q is positive (negative for a
			// bore), f linear in z.
			f := func(z float64) float64 {
				if bore {
					return q - rim(z)
				}
				return rim(z) - q
			}
			a, b := z0, z1
			switch {
			case f(z0) > 0 && f(z1) > 0:
			case f(z0) <= 0 && f(z1) <= 0:
				b = a
			default:
				z := z0 + (rim(0)+k*z0-q)/-k
				if f(z0) > 0 {
					b = z
				} else {
					a = z
				}
			}
			area += b - a
		}
		v += d * (2 * math.Pi / m) * h * area
	}
	return v
}

// A circular fillet or chamfer whose foot runs off its face into a side
// on a cone about an axis square to the face: past a drafted disc's rim
// from a rod's foot or a hole's rim, into a drafted bore beside a rod
// (fillet_past_side_cone.go), the bore's cone carried on above the top
// running into the rod where it widens at 10° (refused before
// 2026-11-19); and past a disc's rim, upright or drafted, where the
// solid goes on below it as a ledge (the cutter starting at the side's
// foot), a hole there 0.3 from the side (refused before 2026-11-19),
// and holes 0.5 and 0.8 from it rounded 3.5 (refused before 2026-11-19).
// Refused as running off the face before 2026-11-09. Every case sound
// and within 3e-4 of the ring's integral within the cone. The ordinary
// run takes one case of each solid and draft, the size and the kind
// turning from case to case; -short the hardest, a rod off centre on a
// drafted disc, the rod the widening bore runs into, and a ledge's
// chamfer; make test-sweeps all 108.
func TestFilletPastConeSide(t *testing.T) {
	t.Parallel()
	combo := 0
	for _, p := range conePlates {
		for _, draft := range p.drafts {
			combo++
			for _, flat := range []bool{false, true} {
				for k, r := range p.sizes {
					switch {
					case sweep.Full():
					case testing.Short():
						if !(p.name == "disc rod" && draft == 5 && !flat && r == 3) &&
							!(p.name == "ledge rod" && draft == 5 && flat && r == 2.5) &&
							!(p.name == "bore rod close" && !flat && r == 3) {
							continue
						}
					case flat != (combo%2 == 1) || k != combo%len(p.sizes):
						continue
					}
					what := map[bool]string{false: "fillet", true: "chamfer"}[flat]
					t.Run(fmt.Sprintf("%s %g/%s %g", p.name, draft, what, r), func(t *testing.T) {
						t.Parallel()
						checkFilletCone(t, p, draft, flat, r)
					})
				}
			}
		}
	}
}

// filletOrChamfer rounds the circle of base at c.Plus(rho, 0, 0) at size
// r, a chamfer where flat, and gives the group and its problem, or what
// the call failed with.
func filletOrChamfer(d *Doc, base Group, at Vec, flat bool, r float64) (g Group, msg string) {
	e := base.Solid().EdgeAt(at)
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
	return g, g.Problem()
}

// checkFilletCone rounds p's circle at size r and checks the result is
// sound and holds the ring within p's cone.
func checkFilletCone(t *testing.T, p conePlate, draft float64, flat bool, r float64) {
	t.Helper()
	checkFilletConeAt(t, p, draft, flat, r, 0.003)
}

// checkFilletConeAt is checkFilletCone at the chord tolerance chord, and
// gives the rounded group.
func checkFilletConeAt(t *testing.T, p conePlate, draft float64, flat bool, r, chord float64) Group {
	t.Helper()
	d := New()
	d.Edit.SetChordTolerance(chord)
	base, vol := p.make(d, draft)
	if msg := base.Problem(); msg != "" {
		t.Fatal(msg)
	}
	g, msg := filletOrChamfer(d, base, p.c.Plus(V(p.rho, 0, 0)), flat, r)
	if msg != "" {
		t.Fatalf("not made: %s", msg)
	}
	if rep := g.Analyze().Check(); !rep.Sound() {
		t.Fatalf("unsound: %s", rep.Summary())
	}
	// Against the base as meshed, so that the chords of its own curves
	// don't count.
	was := base.Analyze().Volume()
	if math.Abs(was-vol)/vol > 3e-4 {
		t.Fatalf("base volume %.4f, want %.4f", was, vol)
	}
	ring := ringPastCone(p.c.X, p.c.Y, p.rho, r, flat, p.hole, func(z float64) float64 { return p.rim(z, draft) }, p.bore)
	want := was + ring
	if p.hole {
		want = was - ring
	}
	got := g.Analyze().Volume()
	if off := math.Abs(got-want) / want; off > 3e-4 {
		t.Errorf("volume %.4f, want %.4f (off by %.1e)", got, want, off)
	}
	t.Logf("ring %.4f, off by %.1e of the solid", ring, math.Abs(got-want)/want)
	return g
}

// The hole chamfered 3.5 on the upright disc on a ledge, 0.5005 to 0.504
// from the rim: the chamfer's cone meets the ledge's top in a circle of
// radius 4.5 just inside the foot's of radius 8, 0.0005 to 0.004 from
// it. The cutter's circle there, a long arc in 32 chords, sags 0.005
// off its arc, and the chords crossed the cone where the circle does
// not, which the cut back took for crossings, and it lost the ledge's
// top and the side on one half of the disc (srf.refineNearMisses,
// 2026-11-19). Made at the size asked for since 2026-11-19, moved by
// footClearSize or refused before. The hole 0.5 from the rim, the
// circles touching, is made at a size within the chord tolerance
// (TestFilletPastConeSide). Each case takes 20 s of processor: -short
// leaves them to srf.TestBooleanTouchingRingSweep.
func TestFilletPastConeSideNearFoot(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("holes near a ledged disc's foot, 20 s of processor each")
	}
	for _, gap := range []float64{0.5005, 0.501, 0.502, 0.503, 0.504} {
		t.Run(fmt.Sprint(gap), func(t *testing.T) {
			t.Parallel()
			g := checkFilletConeAt(t, ledgeHoleAt("", 0, gap), 0, true, 3.5, 0.003)
			if note := g.Note(); note != "" {
				t.Errorf("made at another size: %s", note)
			}
		})
	}
}

// Holes in the disc on a ledge at a chord tolerance of 0.001, 2 and 0.5
// from the rim, rounded and chamfered: the slab past the rim was refused
// as filling more than lies past the side, the allowance taken from the
// chord tolerance (0.00018) where the rim's mesh, its quarters in 32
// chords each, sags 0.0024 inside it (curvedSide.sag, 2026-11-19); each
// was refused as running off the face, and made at the coarser default.
// -short takes one.
func TestFilletPastConeSideFineChord(t *testing.T) {
	t.Parallel()
	for _, gap := range []float64{2, 0.5} {
		for _, flat := range []bool{false, true} {
			if testing.Short() && (gap != 2 || !flat) {
				continue
			}
			t.Run(fmt.Sprintf("%g %v", gap, flat), func(t *testing.T) {
				t.Parallel()
				checkFilletConeAt(t, ledgeHoleAt("", 0, gap), 0, flat, 2.5, 0.001)
			})
		}
	}
}

// What the kernel refuses past a side on a cone, saying why: a hole's
// rim 0.3 from the disc's side chamfered 3.5 deep, through the disc 3
// above the ledge into the ledge past the side, which the cutter
// starting at the side's foot would leave cut, refused by name since
// 2026-11-19 (curvedSide.cutsBelow; it failed before in the slab, which
// filled the hole's sliver past its wall). Chamfered so deep 2 from the
// side, refused too before 2026-11-19, its cut into the ledge lies
// within the side and is made, as the hole 0.3 from the side is to 3
// (TestFilletPastConeSide). A hole's rim 0.502 from the side chamfered
// 3.5, its cone's circle on the ledge's top 0.002 inside the foot's,
// refused by name on 2026-11-19 (topo.footClearSize), is made since the
// same day (TestFilletPastConeSideNearFoot).
func TestFilletPastConeSideRefused(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		make func(d *Doc) Group
		at   Vec
		flat bool
		r    float64
		want string
	}{
		{"hole's rim into the ledge past the side", func(d *Doc) Group {
			ledgeUnder(d, 0)
			draftedDisc(d, 0)
			g, _ := holeAt(d, 3.7)
			return g
		}, V(7.7, 0, 4), true, 3.5, "cuts into the solid below the side's foot"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			d := New()
			d.Edit.SetChordTolerance(0.003)
			base := c.make(d)
			if _, msg := filletOrChamfer(d, base, c.at, c.flat, c.r); !strings.Contains(msg, c.want) {
				t.Errorf("got %q, want it refused saying %q", msg, c.want)
			}
		})
	}
}
