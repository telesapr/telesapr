package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// setbackSeams is the largest angle, in radians, between the normals of
// the faces either side of an edge that a setback patch has: the
// patches are tangent to the fillets, the faces and one another.
func setbackSeams(t *testing.T, sh *srf.SShell) (float64, int) {
	t.Helper()
	patch := map[int]bool{}
	for i := range sh.Surface {
		s := &sh.Surface[i]
		if s.DegM == 5 && s.DegN == 5 {
			patch[int(s.H.V)] = true
		}
	}
	worst, seams := 0.0, 0
	for _, c := range sh.Curve {
		if !patch[int(c.SurfA.V)] && !patch[int(c.SurfB.V)] {
			continue
		}
		a, b := sh.FindSurface(c.SurfA), sh.FindSurface(c.SurfB)
		if a == nil || b == nil {
			continue
		}
		seams++
		for _, pt := range c.Points {
			var ua, ub geom.Point2d
			a.ClosestPointTo(pt.P, &ua, false)
			b.ClosestPointTo(pt.P, &ub, false)
			if a.PointAtUv(ua).Minus(pt.P).Magnitude() > 1e-4 || b.PointAtUv(ub).Minus(pt.P).Magnitude() > 1e-4 {
				t.Logf("a point of a seam off a face at %v", pt.P)
				continue
			}
			na, nb := a.NormalAtUv(ua).WithMagnitude(1), b.NormalAtUv(ub).WithMagnitude(1)
			worst = math.Max(worst, math.Acos(math.Min(1, math.Abs(na.Dot(nb)))))
		}
	}
	return worst, seams
}

// Corners where fillets of different radii meet are set back and the
// hole filled (roadmap 22.4): the box's top ring larger than its
// uprights, and a corner of three radii. The solid is sound, every seam
// of the patches tangent, and the volume below the box's and above the
// box less every fillet run to the corner and, at each corner, a cube
// of the largest radius, more than a sphere's or a setback's corner
// takes.
func TestFilletSetback(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	const tol = 0.002
	srf.SetChordTolMm(tol)
	cases := []struct {
		name  string
		picks []topo.FilletPick
		taken float64 // the fillets run to the corners, and a cube a corner
	}{
		{"ring 3 over uprights 1", append(boxUprights(1), boxRing(30, 3)...), 4*30*1*1*(1-math.Pi/4) + 60*9*(1-math.Pi/4) + 4*27},
		{"three radii", []topo.FilletPick{
			{A: v(10, 20, 30), B: v(10, 20, 0), R: 1},
			{A: v(10, 20, 30), B: v(0, 20, 30), R: 2},
			{A: v(10, 20, 30), B: v(10, 0, 30), R: 3}},
			30*1*(1-math.Pi/4) + 10*4*(1-math.Pi/4) + 20*9*(1-math.Pi/4) + 27},
	}
	for _, c := range cases {
		sh := box()
		if err := topo.FilletPicks(sh, c.picks, 1, 1<<30, false); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		got := sound(t, sh)
		if got > 6000 || got < 6000-c.taken {
			t.Errorf("%s: volume %g, not within %g and 6000", c.name, got, 6000-c.taken)
		}
		worst, seams := setbackSeams(t, sh)
		if seams == 0 {
			t.Errorf("%s: no setback patches", c.name)
		}
		if worst > 1e-3 {
			t.Errorf("%s: a seam of the patches turns by %.2g rad", c.name, worst)
		}
		t.Logf("%s: volume %.4f, %d seams, worst %.2g rad", c.name, got, seams, worst)
	}
}

// At a corner of one radius the setback takes about what the sphere
// does.
func TestFilletSetbackAgainstSphere(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.002)
	for _, r := range []float64{1, 2, 3} {
		picks := []topo.FilletPick{
			{A: v(10, 20, 30), B: v(10, 20, 0), R: r},
			{A: v(10, 20, 30), B: v(0, 20, 30), R: r},
			{A: v(10, 20, 30), B: v(10, 0, 30), R: r}}
		sphere, setback := box(), box()
		if err := topo.FilletPicks(sphere, picks, 1, 1<<30, false); err != nil {
			t.Fatal(err)
		}
		if err := topo.FilletPicksSetback(setback, picks, 1, 1<<30); err != nil {
			t.Fatalf("r %g: %v", r, err)
		}
		vs, vb := sound(t, sphere), sound(t, setback)
		// The two are different blends of one size: the setback takes
		// about what the sphere does, within a share of r^3 larger than the
		// mesh's error in the difference at this tolerance.
		if k := (vs - vb) / (r * r * r); math.Abs(k) > 0.3 {
			t.Errorf("r %g: the setback takes %.3g r^3 more than the sphere", r, k)
		}
	}
}

// More than three rounded edges at a corner (roadmap 22.4): the four
// edges at a pyramid's apex, of one radius and of four, set back and the
// hole of eight sides filled; sound, tangent all round, less than the
// pyramid.
func TestFilletSetbackApex(t *testing.T) {
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	srf.SetChordTolMm(0.002)
	apex := v(10, 10, 10)
	// Clockwise seen from above.
	base := []geom.Vector{v(0, 0, 0), v(0, 20, 0), v(20, 20, 0), v(20, 0, 0)}
	whole := 20.0 * 20 * 10 / 3
	for _, radii := range [][4]float64{{1, 1, 1, 1}, {1, 1.5, 2, 1.5}} {
		sh, _ := pyramid(t, base, apex)
		if got := sound(t, sh); math.Abs(got-whole) > 1e-3*whole {
			t.Fatalf("the pyramid's volume %g, want %g", got, whole)
		}
		var picks []topo.FilletPick
		for i, b := range base {
			picks = append(picks, topo.FilletPick{A: apex, B: b, R: radii[i]})
		}
		if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
			t.Errorf("radii %v: %v", radii, err)
			continue
		}
		got := sound(t, sh)
		if got >= whole || got < whole-40 {
			t.Errorf("radii %v: volume %g of %g", radii, got, whole)
		}
		worst, seams := setbackSeams(t, sh)
		if seams == 0 || worst > 1e-3 {
			t.Errorf("radii %v: %d seams, the worst turning by %.2g rad", radii, seams, worst)
		}
		t.Logf("radii %v: volume %.4f, %d seams, worst %.2g rad", radii, got, seams, worst)
	}
}

// setbackCase sets back the box's corner of radii r1, r2, r3, or the
// pyramid's apex of r1, r2, r3, r2, and checks the solid: sound and
// tangent all round. refused is whether the fillet was refused.
func setbackCase(t *testing.T, corner string, r1, r2, r3 float64) (refused bool, err error) {
	srf.SetChordTolMm(0.002)
	var sh *srf.SShell
	var picks []topo.FilletPick
	if corner == "box" {
		sh = box()
		picks = []topo.FilletPick{
			{A: v(10, 20, 30), B: v(10, 20, 0), R: r1},
			{A: v(10, 20, 30), B: v(0, 20, 30), R: r2},
			{A: v(10, 20, 30), B: v(10, 0, 30), R: r3}}
	} else {
		apex := v(10, 10, 10)
		base := []geom.Vector{v(0, 0, 0), v(0, 20, 0), v(20, 20, 0), v(20, 0, 0)}
		sh, _ = pyramid(t, base, apex)
		for i, r := range []float64{r1, r2, r3, r2} {
			picks = append(picks, topo.FilletPick{A: apex, B: base[i], R: r})
		}
	}
	if err := topo.FilletPicks(sh, picks, 1, 1<<30, false); err != nil {
		return true, err
	}
	var m srf.SMesh
	sh.TriangulateInto(&m)
	if rep := check.Solid(sh, &m); !rep.Sound() {
		return false, fmt.Errorf("unsound: %s", rep.String())
	}
	if worst, _ := setbackSeams(t, sh); worst > 1e-3 {
		return false, fmt.Errorf("a seam turns by %.2g rad", worst)
	}
	return false, nil
}

// TestFilletSetbackSweep sets back a box's corner of three radii, and a
// pyramid's apex of four, over a grid of radii no more than 3 to 1 apart:
// each made, sound and tangent all round. The rate allowed, 0, was fixed
// before the sweep first ran.
func TestFilletSetbackSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("corner", "box", "apex"),
			sweep.Floats("r1", 0.75, 1.5, 2.25),
			sweep.Floats("r2", 0.75, 1.5, 2.25),
			sweep.Floats("r3", 0.75, 2.25),
		},
		MaxFailRate: 0,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		_, err := setbackCase(t, p.String("corner"), p.Float("r1"), p.Float("r2"), p.Float("r3"))
		return err
	}).Require(t)
}

// TestFilletSetbackExtremeSweep is the same over radii up to 6 to 1
// apart. What is made must be sound and tangent all round; a corner
// whose patches would fold is refused, and those refusals are the rate
// measured: 4 of 36 when this was written (11%: a box's corner of 3,
// 0.5 and 0.75 either way round, and two apexes, each with radii 5 or 6
// to 1 side by side), allowed up to 12%. The allowance was set after
// the first run, unlike the grid above's. The whole grid always runs.
func TestFilletSetbackExtremeSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	old := srf.ChordTolMm()
	defer srf.SetChordTolMm(old)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("corner", "box", "apex"),
			sweep.Floats("r1", 0.5, 1.5, 3),
			sweep.Floats("r2", 0.5, 1.5, 3),
			sweep.Floats("r3", 0.75, 2.5),
		},
		MaxFailRate: 0.12,
	}
	bad := 0
	grid.Run(func(p sweep.Point) error {
		refused, err := setbackCase(t, p.String("corner"), p.Float("r1"), p.Float("r2"), p.Float("r3"))
		if err != nil && !refused {
			bad++
			t.Errorf("made but not right at %v: %v", p, err)
		}
		return err
	}).Require(t)
	if bad > 0 {
		t.Errorf("%d corners made unsound or not tangent", bad)
	}
}
