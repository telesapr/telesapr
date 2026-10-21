package topo_test

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// loopPath is the closed curve at(a), a from 0 to 2 pi, as n cubics
// through points evenly apart in a, each from its point and derivative.
func loopPath(n int, at func(a float64) (geom.Vector, geom.Vector)) []srf.SBezier {
	da := 2 * math.Pi / float64(n)
	var path []srf.SBezier
	for k := range n {
		pa, ta := at(float64(k) * da)
		pb, tb := at(float64(k+1) * da)
		if k == n-1 {
			pb, tb = at(0)
		}
		path = append(path, hermite(pa, ta.ScaledBy(da), pb, tb.ScaledBy(da)))
	}
	// The pieces meet exactly, as the sketch makes them.
	for k := range path {
		path[k].Ctrl[path[k].Degree] = path[(k+1)%n].Ctrl[0]
	}
	return path
}

// circlePath is a circle of radius r in the XY plane.
func circlePath(r float64) []srf.SBezier {
	return loopPath(16, func(a float64) (geom.Vector, geom.Vector) {
		return v(r*math.Cos(a), r*math.Sin(a), 0), v(-r*math.Sin(a), r*math.Cos(a), 0)
	})
}

// saddlePath is an ellipse of half axes r and 0.6 r, bent up and down
// by h twice a turn and tilted: a closed path off any plane, round which
// rotation-minimising frames come back turned (a tennis ball's seam,
// symmetric, would bring them back as they left).
func saddlePath(r, h float64) []srf.SBezier {
	b, c := 0.6*r, 5.0
	return loopPath(24, func(a float64) (geom.Vector, geom.Vector) {
		return v(r*math.Cos(a), b*math.Sin(a), h*math.Sin(2*a)+c*math.Cos(a)),
			v(-r*math.Sin(a), b*math.Cos(a), 2*h*math.Cos(2*a)-c*math.Sin(a))
	})
}

// Profiles swept round closed paths, a circle and a saddle, with and
// without a whole turn of twist: no caps, the surfaces meeting where the
// path closes, sound, and of the volume the centroid theorem gives, the
// profile's area times the path's length.
func TestPathSweepClosed(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	for _, c := range []struct {
		name    string
		path    []srf.SBezier
		profile int
		twist   float64
	}{
		{"torus", circlePath(20), 0, 0},
		{"square ring", circlePath(20), 1, 0},
		{"twisted ring", circlePath(20), 1, 2 * math.Pi},
		{"saddle", saddlePath(25, 8), 1, 0},
		{"saddle with a hole", saddlePath(25, 8), 2, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			loops, area := profileAcross(c.path, c.profile)
			sp := newPath(t, c.path, topo.PathSweepOptions{Tol: 5e-4, Closed: true, Twist: c.twist})
			sh, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
			if err != nil {
				t.Fatal(err)
			}
			for i := range sh.Surface {
				if s := &sh.Surface[i]; s.DegM == 1 && s.DegN == 1 {
					t.Fatalf("a closed sweep has a flat face: a cap")
				}
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			perimeter := 0.0
			for _, l := range loops {
				perimeter += pathLength(l)
			}
			want := area * pathLength(c.path)
			allow := tol.ChordTolMm() * perimeter * pathLength(c.path)
			if !rep.Sound() || math.Abs(rep.Volume-want) > allow {
				t.Errorf("%s, want volume %.4f within %.3g", rep.String(), want, allow)
				for _, pr := range rep.Topology {
					t.Logf("  %s at %v", pr.Msg, pr.At)
				}
			}
		})
	}
}

// The frames round a closed path meet where it closes: the profile at
// the end is the profile at the start, whatever turn the path's shape
// gives the frames on the way.
func TestPathSweepClosedFramesMeet(t *testing.T) {
	t.Parallel()
	for _, path := range [][]srf.SBezier{circlePath(20), saddlePath(25, 8)} {
		sp := newPath(t, path, topo.PathSweepOptions{Closed: true})
		m := sp.Motion(sp.End())
		for _, p := range []geom.Vector{v(20, 3, 1), v(18, -2, 4), v(22, 0, -3)} {
			if d := m(p).Minus(p).Magnitude(); d > 1e-6 {
				t.Errorf("the frames don't meet: %v moves by %g round the path", p, d)
			}
		}
	}
	// Left alone, a saddle's frames come back turned; a circle's don't.
	open := newPath(t, saddlePath(25, 8), topo.PathSweepOptions{})
	if p := v(25, 0, 8); open.Motion(open.End())(p).Minus(p).Magnitude() < 1 {
		t.Error("the saddle's frames come back as they left; the test shows nothing")
	}
}

func TestPathSweepClosedRefusals(t *testing.T) {
	t.Parallel()
	circle := circlePath(20)
	if _, err := topo.NewSweepPath(circle, v(0, 0, 1), topo.PathSweepOptions{Closed: true, Twist: math.Pi / 2}); err == nil {
		t.Error("a closed path took a quarter turn of twist")
	}
	if _, err := topo.NewSweepPath(circle[:12], v(0, 0, 1), topo.PathSweepOptions{Closed: true}); err == nil {
		t.Error("an open path swept as closed")
	}
	// A square of cubics closes at a corner.
	sq := []srf.SBezier{
		srf.SBezierFrom2(v(0, 0, 0), v(10, 0, 0)), srf.SBezierFrom2(v(10, 0, 0), v(10, 10, 0)),
		srf.SBezierFrom2(v(10, 10, 0), v(0, 10, 0)), srf.SBezierFrom2(v(0, 10, 0), v(0, 0, 0)),
	}
	if _, err := topo.NewSweepPath(sq[1:], v(0, 0, 1), topo.PathSweepOptions{Closed: true}); err == nil {
		t.Error("an open chain of three sides swept as closed")
	}
	sq2 := []srf.SBezier{sq[0], sq[1], sq[2], sq[3]}
	if _, err := topo.NewSweepPath(sq2, v(0, 0, 1), topo.PathSweepOptions{Closed: true}); err == nil {
		t.Error("a path closing at a corner swept as closed")
	}
}
