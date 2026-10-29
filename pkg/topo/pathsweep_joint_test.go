package topo_test

import (
	"errors"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
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
