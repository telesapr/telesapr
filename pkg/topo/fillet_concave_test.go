package topo_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// notch is a prism of height lean (an edge's direction and length) over
// a block with a wedge of angle beta cut from its top: the wedge's edge
// at the origin is concave, its faces 10 long.
func notch(beta float64, lean geom.Vector) *srf.SShell {
	a := v(10*math.Cos(math.Pi/2-beta/2), 10*math.Sin(math.Pi/2-beta/2), 0)
	b := v(10*math.Cos(math.Pi/2+beta/2), 10*math.Sin(math.Pi/2+beta/2), 0)
	var sh srf.SShell
	sh.MakePrism([]geom.Vector{v(0, 0, 0), b, v(b.X, -10, 0), v(a.X, -10, 0), a}, v(0, 0, 0), lean,
		core.NewRgbaColor(100, 100, 100, 255))
	return &sh
}

// pocketed is a 40 x 30 x 20 block with a 20 x 10 pocket 12 deep in its
// top.
func pocketed() *srf.SShell {
	var block, pocket, out srf.SShell
	block.MakePrism([]geom.Vector{v(0, 0, 0), v(40, 0, 0), v(40, 30, 0), v(0, 30, 0)}, v(0, 0, 0), v(0, 0, 20),
		core.NewRgbaColor(100, 100, 100, 255))
	pocket.MakePrism([]geom.Vector{v(10, 10, 8), v(30, 10, 8), v(30, 20, 8), v(10, 20, 8)}, v(0, 0, 0), v(0, 0, 13),
		core.NewRgbaColor(100, 100, 100, 255))
	out.MakeFromDifferenceOf(&block, &pocket)
	return &out
}

// bossOnPlate is a 40 x 40 x 5 plate with a cylinder of radius rad and
// height h standing on its middle.
func bossOnPlate(rad, h float64) *srf.SShell {
	var plate, boss, out srf.SShell
	plate.MakePrism([]geom.Vector{v(-20, -20, 0), v(20, -20, 0), v(20, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 5),
		core.NewRgbaColor(100, 100, 100, 255))
	boss.MakeFromTransformationOf(cylinder(rad, h+1), v(0, 0, 4), geom.QuaternionIdentity, 1)
	out.MakeFromUnionOf(&plate, &boss)
	return &out
}

// blindHole is a 40 x 40 x 10 plate with a hole of radius rad, depth
// deep, in its top.
func blindHole(rad, deep float64) *srf.SShell {
	var plate, hole, out srf.SShell
	plate.MakePrism([]geom.Vector{v(-20, -20, 0), v(20, -20, 0), v(20, 20, 0), v(-20, 20, 0)}, v(0, 0, 0), v(0, 0, 10),
		core.NewRgbaColor(100, 100, 100, 255))
	hole.MakeFromTransformationOf(cylinder(rad, deep+1), v(0, 0, 10-deep), geom.QuaternionIdentity, 1)
	out.MakeFromDifferenceOf(&plate, &hole)
	return &out
}

// meshVolume is the volume of sh's mesh.
func meshVolume(sh *srf.SShell) float64 {
	var m srf.SMesh
	sh.TriangulateInto(&m)
	return m.CalculateVolume()
}

// A concave edge's fillet fills the corner with the area a convex one
// would take: the wedge's edge, square and leaning.
func TestFilletConcaveEdge(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.001)
	for _, c := range []struct {
		beta, r float64
		lean    geom.Vector
	}{{math.Pi / 2, 2, v(0, 0, 10)}, {math.Pi / 3, 1, v(0, 0, 10)}, {2.5, 3, v(0, 3, 10)}} {
		sh := notch(c.beta, c.lean)
		before := meshVolume(sh)
		if err := topo.FilletEdges(sh, [][2]geom.Vector{{v(0, 0, 0), c.lean}}, c.r, 1<<30); err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
		got := sound(t, sh) - before
		e := c.lean.WithMagnitude(1)
		ua, ub := v(math.Cos(math.Pi/2-c.beta/2), math.Sin(math.Pi/2-c.beta/2), 0), v(math.Cos(math.Pi/2+c.beta/2), math.Sin(math.Pi/2+c.beta/2), 0)
		ua, ub = ua.Minus(e.ScaledBy(ua.Dot(e))).WithMagnitude(1), ub.Minus(e.ScaledBy(ub.Dot(e))).WithMagnitude(1)
		beta := math.Acos(ua.Dot(ub))
		want := filletLoss(c.r, beta, c.lean.Magnitude())
		if allow := 2 * 0.001 * c.r * (math.Pi - beta) * c.lean.Magnitude(); math.Abs(got-want) > allow {
			t.Errorf("%+v: added %g, want %g within %g", c, got, want, allow)
		}
	}
}

// A pocket's floor edges meet in concave mitres, and with its side edges
// in concave spheres; the volume filled is what fillets of the pocket's
// box would take from it.
func TestFilletPocket(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	floor := [][2]geom.Vector{{v(10, 10, 8), v(30, 10, 8)}, {v(30, 10, 8), v(30, 20, 8)},
		{v(30, 20, 8), v(10, 20, 8)}, {v(10, 20, 8), v(10, 10, 8)}}
	var all [][2]geom.Vector
	all = append(all, floor...)
	for _, p := range []geom.Vector{v(10, 10, 8), v(30, 10, 8), v(30, 20, 8), v(10, 20, 8)} {
		all = append(all, [2]geom.Vector{p, v(p.X, p.Y, 20)})
	}
	for _, c := range []struct {
		name            string
		segs            [][2]geom.Vector
		length          float64
		mitres, spheres int
	}{
		{"floor", floor, 60, 4, 0},
		{"all", all, 108, 0, 4},
	} {
		for _, r := range []float64{1, 3} {
			tol := 0.003 * r
			srf.SetChordTolMm(tol)
			sh := pocketed()
			before := meshVolume(sh)
			if err := topo.FilletEdges(sh, c.segs, r, 1<<30); err != nil {
				t.Errorf("%s at %g: %v", c.name, r, err)
				continue
			}
			got := sound(t, sh) - before
			want := cornerLoss(r, c.length, c.mitres, c.spheres)
			area := math.Pi/2*r*c.length + float64(c.spheres)*math.Pi*r*r/2
			if math.Abs(got-want) > tol*area {
				t.Errorf("%s at %g: added %g, want %g within %g", c.name, r, got, want, tol*area)
			}
		}
	}
}

// A boss's foot and a blind hole's floor: concave circular edges, filled
// with a torus; by Pappus, the spandrel's area times the length of the
// circle its centroid follows.
func TestFilletConcaveCircle(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	for _, c := range []struct {
		hole   bool
		rad, r float64
	}{{false, 5, 1}, {false, 8, 4}, {true, 6, 1}, {true, 6, 4}} {
		tol := 0.002 * c.r
		srf.SetChordTolMm(tol)
		var sh *srf.SShell
		// Off the cylinder's seams, which run up from its quarters.
		at := v(c.rad*math.Cos(1), c.rad*math.Sin(1), 5)
		if c.hole {
			sh, at = blindHole(c.rad, 7), v(c.rad*math.Cos(1), c.rad*math.Sin(1), 3)
		} else {
			sh = bossOnPlate(c.rad, 10)
		}
		before := meshVolume(sh)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, c.r, 1<<30, false); err != nil {
			t.Fatalf("%+v: %v", c, err)
		}
		a, cd := spandrel(c.r)
		want := a * 2 * math.Pi * (c.rad + cd)
		if c.hole {
			want = a * 2 * math.Pi * (c.rad - cd)
		}
		got := sound(t, sh) - before
		allow := tol * 2 * math.Pi * (c.rad + c.r) * 4 * c.r
		if math.Abs(got-want) > allow {
			t.Errorf("%+v: added %g, want %g within %g", c, got, want, allow)
		}
	}
}

func TestFilletConcaveRefused(t *testing.T) {
	// The notch's concave edge and a convex top edge at one corner.
	sh := notch(math.Pi/2, v(0, 0, 10))
	a := v(10*math.Cos(math.Pi/4), 10*math.Sin(math.Pi/4), 10)
	err := topo.FilletEdges(sh, [][2]geom.Vector{{v(0, 0, 0), v(0, 0, 10)}, {v(0, 0, 10), a}}, 1, 1<<30)
	if err == nil || !strings.Contains(err.Error(), "convex edge's fillet meets a concave") {
		t.Errorf("mixed corner: %v", err)
	}
}

// TestFilletConcaveSweep fills a wedge's edge over its angle, the
// radius and its lean, a pocket's floor or all its edges over the
// radius, one foot edge of a square boss over its length and the
// radius, and a round boss's foot or a blind hole's floor over the
// circle's radius and the fillet's, against the volumes by formula; the
// rate allowed, 0, was set before.
func TestFilletConcaveSweep(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("shape", "wedge", "pocket", "foot", "boss", "hole"),
			sweep.Floats("size", 0.3, 0.6, 0.9),
			sweep.Floats("k", 0.1, 0.4, 0.8),
			sweep.Floats("lean", 0, 0.5),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		size, k, lean := p.Float("size"), p.Float("k"), p.Float("lean")
		var sh *srf.SShell
		var picks []topo.FilletPick
		var want, tol, area float64
		switch p.String("shape") {
		case "wedge":
			// The wedge's angle from 30 to 160 degrees; the radius whose
			// tangent lines reach k of its 10 long faces.
			beta := (30 + size*145) * math.Pi / 180
			l := v(0, 6*lean, 10)
			sh = notch(beta, l)
			e := l.WithMagnitude(1)
			ua := v(math.Cos(math.Pi/2-beta/2), math.Sin(math.Pi/2-beta/2), 0)
			ub := v(math.Cos(math.Pi/2+beta/2), math.Sin(math.Pi/2+beta/2), 0)
			ua, ub = ua.Minus(e.ScaledBy(ua.Dot(e))).WithMagnitude(1), ub.Minus(e.ScaledBy(ub.Dot(e))).WithMagnitude(1)
			b := math.Acos(ua.Dot(ub))
			da := v(math.Cos(math.Pi/2-beta/2), math.Sin(math.Pi/2-beta/2), 0)
			r := k * 10 * da.Dot(ua) * math.Tan(b/2)
			picks = []topo.FilletPick{{A: v(0, 0, 0), B: l}}
			want = filletLoss(r, b, l.Magnitude())
			tol = 0.002 * r
			area = r * (math.Pi - b) * l.Magnitude()
			srf.SetChordTolMm(tol)
			before := meshVolume(sh)
			if err := topo.FilletPicks(sh, picks, r, 1<<30, false); err != nil {
				return err
			}
			want += before
		case "pocket":
			// The radius k of the pocket's half width; all its edges, or
			// its floor's when it leans.
			r := k * 5
			sh = pocketed()
			for _, q := range [][2]geom.Vector{{v(10, 10, 8), v(30, 10, 8)}, {v(30, 10, 8), v(30, 20, 8)},
				{v(30, 20, 8), v(10, 20, 8)}, {v(10, 20, 8), v(10, 10, 8)}} {
				picks = append(picks, topo.FilletPick{A: q[0], B: q[1]})
			}
			length, mitres, spheres := 60.0, 4, 0
			if lean == 0 {
				for _, q := range []geom.Vector{v(10, 10, 8), v(30, 10, 8), v(30, 20, 8), v(10, 20, 8)} {
					picks = append(picks, topo.FilletPick{A: q, B: v(q.X, q.Y, 20)})
				}
				length, mitres, spheres = 108, 0, 4
			}
			tol = 0.003 * r
			area = math.Pi/2*r*length + float64(spheres)*math.Pi*r*r/2
			srf.SetChordTolMm(tol)
			before := meshVolume(sh)
			if err := topo.FilletPicks(sh, picks, r, 1<<30, false); err != nil {
				return err
			}
			want = before + cornerLoss(r, length, mitres, spheres)
		case "foot":
			l := 4 + size*30
			r := k * 5
			var block, boss srf.SShell
			block.MakePrism([]geom.Vector{v(-40, -40, 0), v(40, -40, 0), v(40, 40, 0), v(-40, 40, 0)}, v(0, 0, 0), v(0, 0, 10),
				core.NewRgbaColor(100, 100, 100, 255))
			boss.MakePrism([]geom.Vector{v(0, 0, 9), v(l, 0, 9), v(l, 6, 9), v(0, 6, 9)}, v(0, 0, 0), v(0, 0, 6+6*lean),
				core.NewRgbaColor(100, 100, 100, 255))
			sh = &srf.SShell{}
			sh.MakeFromUnionOf(&block, &boss)
			tol = 0.002 * r
			srf.SetChordTolMm(tol)
			before := meshVolume(sh)
			if err := topo.FilletEdges(sh, [][2]geom.Vector{{v(0, 0, 10), v(l, 0, 10)}}, r, 1<<30); err != nil {
				return err
			}
			a, _ := spandrel(r)
			want = before + a*l
			area = math.Pi / 2 * r * l
		default:
			hole := p.String("shape") == "hole"
			rad := 2 + size*12
			room := 20 - rad
			if hole {
				room = rad
			}
			r := k * math.Min(room, 5)
			tol = 0.002 * r
			srf.SetChordTolMm(tol)
			ang := 1 + lean
			at := v(rad*math.Cos(ang), rad*math.Sin(ang), 5)
			if hole {
				sh = blindHole(rad, 6)
				at.Z = 4
			} else {
				sh = bossOnPlate(rad, 6)
			}
			before := meshVolume(sh)
			if err := topo.FilletPicks(sh, []topo.FilletPick{{At: at, Arc: true}}, r, 1<<30, false); err != nil {
				return err
			}
			a, cd := spandrel(r)
			if hole {
				want = before + a*2*math.Pi*(rad-cd)
			} else {
				want = before + a*2*math.Pi*(rad+cd)
			}
			area = 2 * math.Pi * (rad + r) * 4 * r
		}
		tp, err := topo.Build(sh)
		if err != nil {
			return err
		}
		if probs := tp.Validate(); len(probs) > 0 {
			return fmt.Errorf("%s", probs[0].Msg)
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(sh, &m)
		switch {
		case !rep.Sound():
			return fmt.Errorf("unsound: %s", rep.String())
		case math.Abs(rep.Volume-want) > tol*area+1e-9*want:
			return fmt.Errorf("volume %g, want %g within %g", rep.Volume, want, tol*area)
		}
		return nil
	}).Require(t)
}

// A chamfer at a boss's foot fills it with a cone: a right triangle of
// side d turned about the axis.
func TestChamferBossFoot(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.002)
	for _, d := range []float64{1, 3} {
		sh := bossOnPlate(5, 10)
		before := meshVolume(sh)
		if err := topo.FilletPicks(sh, []topo.FilletPick{{At: v(5*math.Cos(1), 5*math.Sin(1), 5), Arc: true}}, d, 1<<30, true); err != nil {
			t.Fatal(err)
		}
		want := d * d / 2 * 2 * math.Pi * (5 + d/3)
		if got := sound(t, sh) - before; math.Abs(got-want) > 0.002*2*math.Pi*(5+d)*4*d {
			t.Errorf("chamfer %g: added %g, want %g", d, got, want)
		}
	}
}

// One foot edge of a square boss: the fill runs on to the planes of
// the boss's sides, which it ends on, the whole spandrel times the
// edge's length.
func TestFilletBossFootEdge(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.002)
	var block, boss, sh srf.SShell
	block.MakePrism([]geom.Vector{v(0, 0, 0), v(40, 0, 0), v(40, 30, 0), v(0, 30, 0)}, v(0, 0, 0), v(0, 0, 10),
		core.NewRgbaColor(100, 100, 100, 255))
	boss.MakePrism([]geom.Vector{v(10, 10, 9), v(30, 10, 9), v(30, 20, 9), v(10, 20, 9)}, v(0, 0, 0), v(0, 0, 11),
		core.NewRgbaColor(100, 100, 100, 255))
	sh.MakeFromUnionOf(&block, &boss)
	before := meshVolume(&sh)
	for _, r := range []float64{1, 4} {
		var cp srf.SShell
		cp.MakeFromCopyOf(&sh)
		if err := topo.FilletEdges(&cp, [][2]geom.Vector{{v(10, 10, 10), v(30, 10, 10)}}, r, 1<<30); err != nil {
			t.Fatalf("%g: %v", r, err)
		}
		a, _ := spandrel(r)
		if got := sound(t, &cp) - before; math.Abs(got-a*20) > 0.002*r*2*20 {
			t.Errorf("%g: added %g, want %g", r, got, a*20)
		}
	}
}
