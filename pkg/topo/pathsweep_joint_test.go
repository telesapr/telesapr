package topo_test

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/fixtures"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// arcPiece is the arc from p leaving along the unit d, turning by angle
// (radians, at most a quarter turn) about z through radius r, its centre
// on the left for a positive angle.
func arcPiece(p, d geom.Vector, r, angle float64) srf.SBezier {
	axis := v(0, 0, 1)
	if angle < 0 {
		axis, angle = axis.ScaledBy(-1), -angle
	}
	c := p.Plus(axis.Cross(d).ScaledBy(r))
	q := p.RotatedAbout(c, axis, angle)
	m := p.Plus(d.ScaledBy(r * math.Tan(angle/2)))
	sb := srf.SBezierFrom3(p, m, q)
	sb.Weight[1] = math.Cos(angle / 2)
	return sb
}

// arcCornerPath is a lead line along x to the origin, a sharp turn by
// corner (degrees) about z, an arc of radius 15 turning by turn degrees,
// and, with tail, a cubic on from it out of the plane.
func arcCornerPath(corner, turn float64, tail bool) []srf.SBezier {
	path := []srf.SBezier{srf.SBezierFrom2(v(-10, 0, 0), v(0, 0, 0))}
	d := v(1, 0, 0).RotatedAroundAxis(v(0, 0, 1), corner*math.Pi/180)
	a := arcPiece(v(0, 0, 0), d, 15, turn*math.Pi/180)
	path = append(path, a)
	if tail {
		p, t := a.Finish(), a.TangentAt(1).WithMagnitude(30)
		nd := t.RotatedAroundAxis(v(0, 1, 0), 0.4)
		path = append(path, hermite(p, t, p.Plus(t.Plus(nd).ScaledBy(0.5)), nd))
	}
	return path
}

// A sharp corner beside an arc on a fitted sweep is joined as a rigid
// sweep's: the run of the lead line, the corner and the arc is swept
// rigidly, the cubic on from it fitted, and the two joined on the section
// between them. The solid is sound, and its volume is the rigid run's
// alone and the profile's area times the cubic's length. Held to the
// arc's axis, the frames are the same; twisted, the twist is held still
// along the run and turns along the cubic.
func TestPathSweepArcCorner(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	for _, c := range []struct {
		corner, turn float64
		opts         topo.PathSweepOptions
	}{
		{60, 60, topo.PathSweepOptions{}},
		{-45, 80, topo.PathSweepOptions{}},
		{90, -30, topo.PathSweepOptions{}},
		// Held to the arc's axis, and twisted along the cubic only.
		{60, 60, topo.PathSweepOptions{Up: v(0, 0, 1)}},
		{-45, 80, topo.PathSweepOptions{Twist: 2, Reach: 6}},
	} {
		for kind := range 3 {
			path := arcCornerPath(c.corner, c.turn, true)
			loops, area := profileAcross(path, kind)
			opts := c.opts
			opts.Tol = 5e-4
			sp := newPath(t, path, opts)
			sh, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
			if err != nil {
				t.Errorf("corner %g turn %g, profile %d: %v", c.corner, c.turn, kind, err)
				continue
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)

			// The rigid run alone.
			run := newPath(t, arcCornerPath(c.corner, c.turn, false), topo.PathSweepOptions{Tol: 5e-4})
			rs, err := topo.MakePathSweep(run, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
			if err != nil {
				t.Fatalf("corner %g turn %g, the run alone: %v", c.corner, c.turn, err)
			}
			var rm srf.SMesh
			rs.TriangulateInto(&rm)
			rrep := check.Solid(rs, &rm)
			perimeter := 0.0
			for _, l := range loops {
				perimeter += pathLength(l)
			}
			tail := pathLength(path[2:])
			want := rrep.Volume + area*tail
			allow := tol.ChordTolMm() * perimeter * pathLength(path)
			t.Logf("corner %g turn %g, profile %d: volume %.4f, want %.4f within %.3g", c.corner, c.turn, kind, rep.Volume, want, allow)
			if !rep.Sound() || !rrep.Sound() || math.Abs(rep.Volume-want) > allow {
				t.Errorf("corner %g turn %g, profile %d: %s (run %s), want volume %.4f within %.3g",
					c.corner, c.turn, kind, rep.String(), rrep.String(), want, allow)
			}
		}
	}
}

// A run whose frames turn otherwise than its rigid steps would turn the
// profile, held to a direction off the arc's axis, is refused by name.
func TestPathSweepArcCornerHeldOff(t *testing.T) {
	t.Parallel()
	path := arcCornerPath(60, 60, true)
	_, err := topo.NewSweepPath(path, v(0, 0, 1), topo.PathSweepOptions{Up: v(0, 1, 1)})
	if !errors.Is(err, topo.ErrPathCornerHeld) {
		t.Errorf("held off the arc's axis: %v", err)
	}
}

// twistedRunPath is a line of length lead along x to the origin, a sharp
// turn by corner (degrees) about z and an arc of radius 15 turning by
// turn degrees; with after, the arc first and the line on from it, the
// corner between them.
func twistedRunPath(lead, corner, turn float64, after bool) []srf.SBezier {
	if !after {
		line := srf.SBezierFrom2(v(-lead, 0, 0), v(0, 0, 0))
		d := v(1, 0, 0).RotatedAroundAxis(v(0, 0, 1), corner*math.Pi/180)
		return []srf.SBezier{line, arcPiece(v(0, 0, 0), d, 15, turn*math.Pi/180)}
	}
	a := arcPiece(v(0, 0, 0), v(1, 0, 0), 15, turn*math.Pi/180)
	p := a.Finish()
	d := a.TangentAt(1).WithMagnitude(1).RotatedAroundAxis(v(0, 0, 1), corner*math.Pi/180)
	return []srf.SBezier{a, srf.SBezierFrom2(p, p.Plus(d.ScaledBy(lead)))}
}

// arrivingLoops are the profile's loops as a twisted sweep along
// twistedRunPath brings them to its rigid run, turned about the path's
// start by the twist when the line before the run has taken all of it,
// as they are when the arc comes first: the untwisted sweep of those is
// the twisted one's volume, the twist along a line keeping it.
func arrivingLoops(path []srf.SBezier, loops [][]srf.SBezier, twist float64, after bool) [][]srf.SBezier {
	if after {
		return loops
	}
	o, axis := path[0].Start(), path[0].TangentAt(0).WithMagnitude(1)
	out := make([][]srf.SBezier, len(loops))
	for i, l := range loops {
		for _, sb := range l {
			for k := 0; k <= sb.Degree; k++ {
				sb.Ctrl[k] = sb.Ctrl[k].RotatedAbout(o, axis, twist)
			}
			out[i] = append(out[i], sb)
		}
	}
	return out
}

// A twisted path whose every piece is in a rigid run, a line and an arc
// about a corner, holds the twist still along the run but for the line's
// stretch past its joint's reach, which leaves the run and takes the
// twist: the solid is sound, and of the volume of the untwisted sweep
// of the profile turned as it reaches the run (arrivingLoops). A line too
// short to leave anything past its joint's reach leaves the arc to twist
// past its own, made sound or refused by name; with the arc too short as
// well, there is nowhere to twist.
func TestPathSweepTwistedRun(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	for _, c := range []struct {
		lead, corner, turn, twist float64
		after                     bool
	}{
		{40, 60, 60, 1, false},
		{40, -45, 80, -2, false},
		{30, 90, -30, 3, false},
		{40, 60, 60, 1.5, true},
		{30, -50, 45, -1, true},
	} {
		for kind := range 3 {
			if fixtures.RaceDetector && kind != 1 {
				// Under the race detector one profile of the three: they
				// sweep on the same goroutines.
				continue
			}
			path := twistedRunPath(c.lead, c.corner, c.turn, c.after)
			loops, _ := profileAcross(path, kind)
			reach := profileReach(path, loops)
			sp := newPath(t, path, topo.PathSweepOptions{Tol: 5e-4, Twist: c.twist, Reach: reach})
			if len(sp.Pieces()) <= len(path) {
				t.Errorf("%+v, profile %d: the line isn't split where its joint's reach ends", c, kind)
			}
			if s := sp.TwistShare(sp.Length()); math.Abs(s-1) > 1e-12 {
				t.Errorf("%+v: twist share %g at the end", c, s)
			}
			sh, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
			if err != nil {
				t.Errorf("%+v, profile %d: %v", c, kind, err)
				continue
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)

			flat := newPath(t, path, topo.PathSweepOptions{Tol: 5e-4})
			fs, err := topo.MakePathSweep(flat, arrivingLoops(path, loops, c.twist, c.after), col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
			if err != nil {
				t.Fatalf("%+v, untwisted: %v", c, err)
			}
			var fm srf.SMesh
			fs.TriangulateInto(&fm)
			frep := check.Solid(fs, &fm)
			perimeter := 0.0
			for _, l := range loops {
				perimeter += pathLength(l)
			}
			allow := tol.ChordTolMm() * perimeter * pathLength(path)
			t.Logf("%+v, profile %d: volume %.4f, untwisted %.4f within %.3g", c, kind, rep.Volume, frep.Volume, allow)
			if !rep.Sound() || !frep.Sound() || math.Abs(rep.Volume-frep.Volume) > allow {
				t.Errorf("%+v, profile %d: %s (untwisted %s), want volume %.4f within %.3g",
					c, kind, rep.String(), frep.String(), frep.Volume, allow)
			}
		}
	}
	// A lead line within its joint's reach: the arc twists past its own
	// joint's reach instead. The lead is then too short for the joint, a
	// corner named; never a solid that isn't sound.
	path := twistedRunPath(1, 60, 60, false)
	loops, _ := profileAcross(path, 1)
	rep, flat, err := twistedArcsSolid(path, loops, 1)
	if _, named := errors.AsType[*srf.CornerError](err); !named {
		if allow := arcsAllowance(path, loops); err != nil || !rep.Sound() || math.Abs(rep.Volume-flat.Volume) > allow {
			t.Errorf("a lead of 1 within its joint's reach: %v, %s, want volume %.4f within %.3g", err, rep.String(), flat.Volume, allow)
		}
	}
	// With the arc too short to leave anything past its joint's reach
	// either: nowhere to twist.
	path = twistedRunPath(1, 60, 8, false)
	_, err = topo.NewSweepPath(path, path[0].TangentAt(0).NormalDir(0),
		topo.PathSweepOptions{Twist: 1, Reach: profileReach(path, loops)})
	if !errors.Is(err, topo.ErrPathCornerTwisted) {
		t.Errorf("a lead of 1 and an arc of 8°, both within the joint's reach: %v, want %v", err, topo.ErrPathCornerTwisted)
	}
}

// Twisted runs of a line and an arc about a sharp corner, over corners
// and arcs either way, short and long lines, the three profiles and two
// twists: each is made sound with the volume of the untwisted sweep of
// the profile as it reaches the run, or refused by name. A path whose
// untwisted solid already crosses itself
// (the arc turning back over the line) is left out. The rate allowed, 0,
// was fixed before the sweep first ran.
func TestPathSweepTwistedRunSweep(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	grid := sweep.Grid{
		Parallel: true,
		Axes: []sweep.Axis{
			sweep.Bools("after"),
			sweep.Floats("corner", -150, -120, -90, -60, -30, 30, 60, 90, 120, 150),
			sweep.Floats("turn", -90, -45, -20, 20, 45, 90),
			sweep.Floats("lead", 12, 25, 40),
			sweep.Floats("profile", 0, 1, 2),
			sweep.Floats("twist", 1, -3),
		},
		MaxFailRate: 0,
		Sample:      24,
	}
	solid := func(path []srf.SBezier, loops [][]srf.SBezier, twist float64) (*check.Report, error) {
		sp, err := topo.NewSweepPath(path, path[0].TangentAt(0).NormalDir(0),
			topo.PathSweepOptions{Tol: 5e-4, Twist: twist, Reach: profileReach(path, loops)})
		if err != nil {
			return nil, err
		}
		sh, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
		if err != nil {
			return nil, err
		}
		var m srf.SMesh
		sh.TriangulateInto(&m)
		rep := check.Solid(sh, &m)
		return &rep, nil
	}
	grid.Run(func(p sweep.Point) error {
		path := twistedRunPath(p.Float("lead"), p.Float("corner"), p.Float("turn"), p.Bool("after"))
		loops, _ := profileAcross(path, int(p.Float("profile")))
		// A path the untwisted sweep refuses, or makes crossing itself, is
		// not this sweep's case.
		flat, flatErr := solid(path, arrivingLoops(path, loops, p.Float("twist"), p.Bool("after")), 0)
		if flatErr == nil && !flat.Sound() {
			return nil
		}
		rep, err := solid(path, loops, p.Float("twist"))
		if _, refused := errors.AsType[*srf.CornerError](err); refused || flatErr != nil {
			return nil //nolint:nilerr // a refusal by name, or a path refused untwisted, is no failure here
		}
		if err != nil {
			return err
		}
		perimeter := 0.0
		for _, l := range loops {
			perimeter += pathLength(l)
		}
		allow := tol.ChordTolMm() * perimeter * pathLength(path)
		if !rep.Sound() || math.Abs(rep.Volume-flat.Volume) > allow {
			return fmt.Errorf("%s, want volume %.4f within %.3g", rep.String(), flat.Volume, allow)
		}
		return nil
	}).Require(t)
}
