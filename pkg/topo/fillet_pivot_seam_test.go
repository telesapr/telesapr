package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// seamPocket is dishPocket's dish, its patches one sheet as a lathed
// sketch's are, with a square pocket 8 on a side from z = 2 up through
// its top whose corner at (16, off) points along +x, turned by rot
// degrees about that corner, built at tol: the seam between the lathe's
// quarters at y = 0 runs through the corner, or off from it, and
// crosses one of the corner's rim edges near it. The picks are the two
// rim edges at the corner, a at 135 + rot degrees from it and b at
// 225 + rot, of sizes ra and rb. spin turns the pocket about the dish's
// axis by that many degrees, the same solid with the seam away from the
// corner.
func seamPocket(off, rot, spin float64, tol srf.Tolerance) (*srf.SShell, func(ra, rb float64) []topo.FilletPick) {
	dish := lathed([]geom.Vector{v(0, 0, 0), v(40, 0, 0), v(40, 0, 20), v(0, 0, 5)}, tol)
	for i := range dish.Surface {
		dish.Surface[i].Sheet = 1
	}
	dir := func(deg float64) geom.Vector {
		r := deg * math.Pi / 180
		return v(math.Cos(r), math.Sin(r), 0)
	}
	turn := func(p geom.Vector) geom.Vector {
		s, k := math.Sincos(spin * math.Pi / 180)
		return v(p.X*k-p.Y*s, p.X*s+p.Y*k, p.Z)
	}
	c := turn(v(16, off, 2))
	a, b := turn(dir(135+rot).ScaledBy(8)), turn(dir(225+rot).ScaledBy(8))
	pocket, out := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
	pocket.MakePrism([]geom.Vector{c, c.Plus(b), c.Plus(a).Plus(b), c.Plus(a)}, v(0, 0, 0), v(0, 0, 30),
		core.NewRGBA(100, 100, 100, 255))
	out.MakeFromDifferenceOf(dish, &pocket)
	on := func(p geom.Vector) geom.Vector { return v(p.X, p.Y, 5+15.0/40*math.Hypot(p.X, p.Y)) }
	return &out, func(ra, rb float64) []topo.FilletPick {
		return []topo.FilletPick{{Arc: true, At: on(c.Plus(a.ScaledBy(0.5))), R: ra},
			{Arc: true, At: on(c.Plus(b.ScaledBy(0.5))), R: rb}}
	}
}

// onSeamNearCorner reports whether the solid has a vertex on the dish's
// top, on the seam at y = 0, within 3 sizes of the corner at (16, off):
// where the corner's side or a fillet's foot was cut at the seam.
func onSeamNearCorner(tp *topo.Topology, off, size float64) bool {
	for _, vx := range tp.Vertices {
		p := vx.P
		top := 5 + 15.0/40*math.Hypot(p.X, p.Y)
		if math.Abs(p.Y) < 1e-6 && p.X > 0 && math.Abs(p.Z-top) < 1e-4 && p.Minus(v(16, off, top)).Magnitude() < 3*size {
			return true
		}
	}
	return false
}

// A pocket's corner on or near the seam between a lathe's quarters, so
// that one fillet's foot on the dish lies on one quarter and the other's
// on the next. Two feet at two depths on the crease, one radius: the
// corner patch's side on the dish crosses the seam, which is cut there
// and dropped back to the rim (rollingpivot_seam.go); they were refused
// "crosses a seam of the face they share" before 2026-11-19. Two radii,
// or two distances: the kinked mitre's point lies on one quarter while
// one chain's edge runs wholly along the other, and that chain's foot is
// carried across the seam a little before its end (rollingmitre_seam.go);
// they failed "a loop doesn't close" before. OpenCASCADE's fillets and
// chamfers of the same edges (scripts/fillet-pivot-seam-fixtures.py,
// FreeCADCmd, its dish's own seam turned away from the corner: across it
// this near the corner its fillets came out inside out) took the volumes
// below; ours, the mesh carried to its surfaces (surfaceVolume) before
// less after, within lo and hi of them: a fillets' corner patch 1% to
// 1.8% under, as the dish's pocket's are (TestFilletPivotCorner), a
// chamfers' 0.05% over and 0.8% under, a mitre within 0.05%. The same
// pocket turned 45 degrees about the dish's axis, its corner well inside
// a quarter, takes within 0.12% of ours, the fillets' fit to their
// tolerance sampled elsewhere: the seam changes nothing of the solid.
// (Turned 30 degrees, the chamfers of 1 and 1.5 at (0.1, 0) were refused
// as not closed, away from any seam.)
func TestFilletPivotSeam(t *testing.T) {
	if testing.Short() {
		t.Skip("fillets and chamfers of a pocket's corner on a lathe's seam, slow")
	}
	t.Parallel()
	cases := sweep.Pick([]struct {
		off, rot float64
		ra, rb   float64
		flat     bool
		occ      float64
		lo, hi   float64
	}{
		{0.1, 0, 1, 1, false, 1.694120, -0.02, 0},
		{0.1, 0, 1, 1, true, 8.089690, -0.001, 0.001},
		{0.1, 20, 1, 1, false, 1.844810, -0.02, 0},
		{-0.2, 20, 1, 1, false, 1.860550, -0.02, 0},
		{-0.2, 20, 1, 1, true, 8.178287, -0.01, 0.001},
		{0.3, -20, 1.5, 1.5, false, 4.208835, -0.02, 0},
		{0.1, 0, 0.5, 0.5, false, 0.426412, -0.02, 0},
		{0.1, 0, 1, 1.5, false, 2.753363, -0.002, 0.002},
		{0.1, 0, 1, 1.5, true, 13.182498, -0.001, 0.001},
		{-0.2, 20, 1.5, 1, false, 3.238393, -0.002, 0.002},
		{0.2, -20, 1, 1.5, true, 13.493981, -0.001, 0.001},
		{0.3, 0, 1.5, 1, false, 2.739362, -0.002, 0.002},
	}, 6)
	for _, c := range cases {
		t.Run(fmt.Sprintf("off %g rot %g %g,%g flat %v", c.off, c.rot, c.ra, c.rb, c.flat), func(t *testing.T) {
			t.Parallel()
			var took [2]float64
			for i, spin := range []float64{0, 45} {
				sh, picks := seamPocket(c.off, c.rot, spin, fine(0.01))
				before := surfaceVolume(t, sh)
				if err := topo.FilletPicks(sh, picks(c.ra, c.rb), 1, 1<<30, c.flat); err != nil {
					t.Fatalf("turned %g: %v", spin, err)
				}
				sound(t, sh)
				if spin == 0 && !onSeamNearCorner(build(t, sh), c.off, max(c.ra, c.rb)) {
					t.Errorf("no vertex on the seam near the corner")
				}
				took[i] = before - surfaceVolume(t, sh)
			}
			t.Logf("took %.6f, turned off the seam %.6f, OpenCASCADE %.6f, %+.5f", took[0], took[1], c.occ, took[0]/c.occ-1)
			if math.Abs(took[0]-took[1]) > 0.002*took[1] {
				t.Errorf("took %.6f, turned off the seam %.6f", took[0], took[1])
			}
			if took[0] < c.occ*(1+c.lo) || took[0] > c.occ*(1+c.hi) {
				t.Errorf("took %.6f, OpenCASCADE %.6f", took[0], c.occ)
			}
		})
	}
}

// The same corner quickly: fillets of one radius at a coarse tolerance,
// the corner patch across the seam, sound.
func TestFilletPivotSeamQuick(t *testing.T) {
	t.Parallel()
	sh, picks := seamPocket(0.1, 0, 0, fine(0.05))
	if err := topo.FilletPicks(sh, picks(1, 1), 1, 1<<30, false); err != nil {
		t.Fatal(err)
	}
	sound(t, sh)
	if !onSeamNearCorner(build(t, sh), 0.1, 1) {
		t.Errorf("no vertex on the seam near the corner")
	}
}

// A chamfer of two distances whose smaller chain's foot crosses the seam
// within a hundredth of its length of the mitre, 0.008 mm, and its
// mirror image: the section rampCrossings puts on the seam stands so
// near the mitre's that their weights interpolate below zero between
// them, refused "just short of its end" before 2026-11-19. The blend is
// tried again with no section there, the foot crossing the seam within
// the end patch and its piece on to the mitre carried onto the next
// quarter (rampSkipNear, mitreFootFace). OpenCASCADE's chamfers of the
// same edges (scripts/fillet-pivot-seam-fixtures.py) took the volumes:
// ours within 1e-5 of the first and 4.3e-4 of its mirror, as the
// mirror's neighbours at 1.48 and 1.52, made with a section on the seam,
// take 3e-4 more than the same pocket turned off the seam.
func TestFilletPivotSeamNearEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("chamfers of a pocket's corner beside a lathe's seam, slow")
	}
	t.Parallel()
	for _, c := range []struct {
		off, rot, ra, rb, occ float64
	}{
		{-0.3, 20, 1, 1.5, 13.135870},
		{0.3, -20, 1.5, 1, 13.135707},
	} {
		t.Run(fmt.Sprintf("off %g rot %g %g,%g", c.off, c.rot, c.ra, c.rb), func(t *testing.T) {
			t.Parallel()
			sh, picks := seamPocket(c.off, c.rot, 0, fine(0.01))
			before := surfaceVolume(t, sh)
			if err := topo.FilletPicks(sh, picks(c.ra, c.rb), 1, 1<<30, true); err != nil {
				t.Fatal(err)
			}
			sound(t, sh)
			if !onSeamNearCorner(build(t, sh), c.off, max(c.ra, c.rb)) {
				t.Errorf("no vertex on the seam near the corner")
			}
			if took := before - surfaceVolume(t, sh); math.Abs(took/c.occ-1) > 5e-4 {
				t.Errorf("took %.6f, OpenCASCADE %.6f", took, c.occ)
			}
		})
	}
}

// The corner of TestFilletPivotSeamNearEnd with its offset from the seam
// and the smaller chain's size moved finely about it, fillets and
// chamfers, so that the smaller foot's crossing of the seam runs up to
// the mitre and past it: each made sound. Before 2026-11-19 the grid's
// full run made 112 of 126 (14 chamfers refused "just short of its
// end").
func TestFilletPivotSeamNearEndSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a grid of a pocket's corners beside a lathe's seam, slow")
	}
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("off", -0.34, -0.33, -0.32, -0.31, -0.3, -0.29, -0.28, -0.27, -0.26),
			sweep.Floats("size", 0.96, 0.98, 0.99, 1, 1.01, 1.02, 1.04),
			sweep.Bools("flat"),
		},
		Sample: 12,
	}
	grid.Run(func(p sweep.Point) error {
		sh, picks := seamPocket(p.Float("off"), 20, 0, fine(0.01))
		if err := topo.FilletPicks(sh, picks(p.Float("size"), 1.5), 1, 1<<30, p.Bool("flat")); err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if rep := check.Solid(sh, &m); !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		return nil
	}).Require(t)
}

// The corner on the seam and a few tenths off it either way, turned by
// up to 20 degrees, fillets and chamfers of one size or two (0.5 to 1.5)
// at a chord tolerance of 0.01 mm: each made sound. The rate allowed,
// 1%, was fixed after the grid's first full run (2026-11-19), two
// chamfers of 1 and 1.5 a mirror pair refused as
// TestFilletPivotSeamNearEnd's were; made since 2026-11-19, none is.
func TestFilletPivotSeamSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a grid of a pocket's corners on a lathe's seam, slow")
	}
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("off", -0.3, -0.2, -0.1, 0, 0.1, 0.2, 0.3),
			sweep.Floats("rot", -20, 0, 20),
			sweep.Strings("sizes", "0.5,0.5", "1,1", "1.5,1.5", "1,1.5", "1.5,1"),
			sweep.Bools("flat"),
		},
		Sample: 24,
	}
	grid.Run(func(p sweep.Point) error {
		var ra, rb float64
		if _, err := fmt.Sscanf(p.String("sizes"), "%g,%g", &ra, &rb); err != nil {
			return err
		}
		sh, picks := seamPocket(p.Float("off"), p.Float("rot"), 0, fine(0.01))
		if err := topo.FilletPicks(sh, picks(ra, rb), 1, 1<<30, p.Bool("flat")); err != nil {
			return err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		if rep := check.Solid(sh, &m); !rep.Sound() {
			return fmt.Errorf("unsound: %s", rep.String())
		}
		return nil
	}).Require(t)
}
