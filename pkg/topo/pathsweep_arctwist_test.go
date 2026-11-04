package topo_test

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// twistedArcsPath is an arc of radius r1 turning by turn1 degrees from
// the origin along x, a sharp corner by corner degrees about z, and an
// arc of radius r2 turning by turn2: a path of arcs and a corner only.
func twistedArcsPath(r1, turn1, corner, r2, turn2 float64) []srf.SBezier {
	a := arcPiece(v(0, 0, 0), v(1, 0, 0), r1, turn1*math.Pi/180)
	p := a.Finish()
	d := a.TangentAt(1).WithMagnitude(1).RotatedAroundAxis(v(0, 0, 1), corner*math.Pi/180)
	return []srf.SBezier{a, arcPiece(p, d, r2, turn2*math.Pi/180)}
}

// turnedLoops are loops turned about the path's start tangent by angle.
func turnedLoops(path []srf.SBezier, loops [][]srf.SBezier, angle float64) [][]srf.SBezier {
	o, axis := path[0].Start(), path[0].TangentAt(0).WithMagnitude(1)
	out := make([][]srf.SBezier, len(loops))
	for i, l := range loops {
		for _, sb := range l {
			for k := 0; k <= sb.Degree; k++ {
				sb.Ctrl[k] = sb.Ctrl[k].RotatedAbout(o, axis, angle)
			}
			out[i] = append(out[i], sb)
		}
	}
	return out
}

// errUntwisted marks a failure of the untwisted sweep twistedArcsSolid
// compares with.
var errUntwisted = errors.New("the untwisted sweep")

// twistedArcsSolid is the twisted sweep of loops along path, and the
// untwisted sweep of the loops turned as the twisted one brings them to
// the corner, the share of the twist turned by then: the twist about a
// profile centred on the path keeps its volume along an arc as along a
// line, and the joint is the same, so the two have one volume.
func twistedArcsSolid(path []srf.SBezier, loops [][]srf.SBezier, twist float64) (rep, flat check.Report, err error) {
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	solid := func(sp *topo.SweepPath, loops [][]srf.SBezier) (check.Report, error) {
		sh, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
		if err != nil {
			return check.Report{}, err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		return check.Solid(sh, &m), nil
	}
	r0 := path[0].TangentAt(0).NormalDir(0)
	sp, err := topo.NewSweepPath(path, r0, topo.PathSweepOptions{Tol: 5e-4, Twist: twist, Reach: profileReach(path, loops)})
	if err != nil {
		return rep, flat, err
	}
	if rep, err = solid(sp, loops); err != nil {
		return rep, flat, err
	}
	share := sp.TwistShare(pathLength(path[:1]))
	fp, err := topo.NewSweepPath(path, r0, topo.PathSweepOptions{Tol: 5e-4})
	if err != nil {
		return rep, flat, fmt.Errorf("%w: %w", errUntwisted, err)
	}
	flat, err = solid(fp, turnedLoops(path, loops, share*twist))
	if err != nil {
		return rep, flat, fmt.Errorf("%w: %w", errUntwisted, err)
	}
	return rep, flat, nil
}

// arcsAllowance is the volume two sweeps along path of loops may differ
// by: the chord tolerance over the profile's sides along the path.
func arcsAllowance(path []srf.SBezier, loops [][]srf.SBezier) float64 {
	perimeter := 0.0
	for _, l := range loops {
		perimeter += pathLength(l)
	}
	return srf.Tol(0.005, 100).ChordTolMm() * perimeter * pathLength(path)
}

// A twisted path of two arcs about a sharp corner, with no line to twist
// on, holds the twist still along each arc only as far as the corner's
// joint reaches, and twists along the rest of the arcs: the solid is
// sound, and of the volume of the untwisted sweep of the profile turned
// as it reaches the corner. Where the joints leave too little of the
// arcs, it is still refused by name.
func TestPathSweepTwistedArcs(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		r1, turn1, corner, r2, turn2, twist float64
		kind                                int
	}{
		{15, 60, 90, 15, 60, 1, 0},
		{15, 60, 90, 15, 60, 1, 1},
		{20, 45, -120, 12, -60, 1, 0},
		{20, 45, -120, 12, -60, -2, 1},
	} {
		path := twistedArcsPath(c.r1, c.turn1, c.corner, c.r2, c.turn2)
		loops, _ := profileAcross(path, c.kind)
		rep, flat, err := twistedArcsSolid(path, loops, c.twist)
		if err != nil {
			t.Errorf("%+v: %v", c, err)
			continue
		}
		allow := arcsAllowance(path, loops)
		t.Logf("%+v: volume %.4f, untwisted %.4f within %.3g", c, rep.Volume, flat.Volume, allow)
		if !rep.Sound() || !flat.Sound() || math.Abs(rep.Volume-flat.Volume) > allow {
			t.Errorf("%+v: %s (untwisted %s), want volume %.4f within %.3g", c, rep.String(), flat.String(), flat.Volume, allow)
		}
	}
	// A corner of 150° between two arcs: their joints take the arcs whole.
	path := twistedArcsPath(15, 80, 150, 15, 80)
	loops, _ := profileAcross(path, 1)
	_, err := topo.NewSweepPath(path, path[0].TangentAt(0).NormalDir(0),
		topo.PathSweepOptions{Twist: 1, Reach: profileReach(path, loops)})
	if !errors.Is(err, topo.ErrPathCornerTwisted) {
		t.Errorf("arcs at a corner of 150°: %v, want %v", err, topo.ErrPathCornerTwisted)
	}
}

// Twisted paths of two arcs about a sharp corner, over corners and turns
// either way, two radii, the three profiles and two twists: each is made
// sound with the volume of the untwisted sweep of the profile turned as
// it reaches the corner, or refused by name. A path the untwisted sweep
// refuses or makes crossing itself is left out. The rate allowed, 0, was
// fixed before the sweep first ran.
func TestPathSweepTwistedArcsSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Floats("corner", -150, -120, -90, -60, 60, 90, 120, 150),
			sweep.Floats("turn1", -80, -40, 40, 80),
			sweep.Floats("turn2", -80, -40, 40, 80),
			sweep.Floats("radius", 12, 25),
			sweep.Floats("profile", 0, 1, 2),
			sweep.Floats("twist", 1, -3),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	grid.Run(func(p sweep.Point) error {
		r := p.Float("radius")
		path := twistedArcsPath(r, p.Float("turn1"), p.Float("corner"), r, p.Float("turn2"))
		loops, _ := profileAcross(path, int(p.Float("profile")))
		rep, flat, err := twistedArcsSolid(path, loops, p.Float("twist"))
		if _, refused := errors.AsType[*srf.CornerError](err); refused {
			return nil //nolint:nilerr // a refusal by name is no failure here
		}
		if errors.Is(err, errUntwisted) {
			return nil //nolint:nilerr // the untwisted sweep refused: not this sweep's case
		}
		if err != nil {
			return err
		}
		if !flat.Sound() {
			return nil
		}
		allow := arcsAllowance(path, loops)
		if !rep.Sound() || math.Abs(rep.Volume-flat.Volume) > allow {
			return fmt.Errorf("%s, want volume %.4f within %.3g", rep.String(), flat.Volume, allow)
		}
		return nil
	}).Require(t)
}
