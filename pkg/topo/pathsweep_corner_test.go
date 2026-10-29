package topo_test

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// cornerPath is a lead line, cubics wandering from it, a line of 12
// off their end, a sharp turn by angle (degrees) about an axis leaning
// out of the cubics' plane by lean, a line of 12 and cubics again.
func cornerPath(r *rand.Rand, angle, lean float64) []srf.SBezier {
	path := wanderPath(r, 2, 10, true)
	last := path[len(path)-1]
	p := last.Finish()
	d1 := last.TangentAt(1).WithMagnitude(1)
	a := p.Plus(d1.ScaledBy(12))
	path = append(path, srf.SBezierFrom2(p, a))
	axis := d1.Cross(v(0, 0, 1)).WithMagnitude(-1).RotatedAroundAxis(d1, lean*math.Pi/180)
	axis = d1.Cross(axis).Cross(d1).WithMagnitude(1)
	d2 := d1.RotatedAroundAxis(axis, angle*math.Pi/180)
	b := a.Plus(d2.ScaledBy(12))
	path = append(path, srf.SBezierFrom2(a, b))
	nd := d2.RotatedAroundAxis(axis, -0.5).WithMagnitude(30)
	q := b.Plus(d2.ScaledBy(30).Plus(nd).ScaledBy(0.5))
	return append(path, hermite(b, d2.ScaledBy(30), q, nd))
}

// Sharp corners between two lines of a sweep along cubics are mitred
// as a rigid sweep's are: each side a prism cut by the plane halving the
// corner, the two sharing the cut. The solid is sound, and keeps the
// profile's area times the path's length, a profile about the path
// losing at the mitre's inside what it gains at its outside.
func TestPathSweepCorner(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	r := rand.New(rand.NewSource(3))
	for _, c := range []struct{ angle, lean float64 }{{30, 0}, {90, 0}, {120, 0}, {60, 40}, {90, 90}} {
		path := cornerPath(r, c.angle, c.lean)
		for kind := range 3 {
			loops, area := profileAcross(path, kind)
			sp := newPath(t, path, topo.PathSweepOptions{Tol: 5e-4})
			sh, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
			if err != nil {
				t.Errorf("corner %g leaning %g, profile %d: %v", c.angle, c.lean, kind, err)
				continue
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			perimeter := 0.0
			for _, l := range loops {
				perimeter += pathLength(l)
			}
			want := area * pathLength(path)
			allow := tol.ChordTolMm() * perimeter * pathLength(path)
			t.Logf("corner %g leaning %g, profile %d: volume %.4f, want %.4f within %.3g", c.angle, c.lean, kind, rep.Volume, want, allow)
			if !rep.Sound() || math.Abs(rep.Volume-want) > allow {
				t.Errorf("corner %g leaning %g, profile %d: %s, want volume %.4f within %.3g",
					c.angle, c.lean, kind, rep.String(), want, allow)
			}
		}
	}
}

// What a corner can't be mitred at is refused by name: a curve beside
// it, a twist with no stretch left to turn on, frames held to a direction the corner turns them off,
// and lines too short for their mitres.
func TestPathSweepCornerRefusals(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(3))
	path := cornerPath(r, 90, 0)
	n := len(path)
	// A cubic straight into the corner.
	bent := append([]srf.SBezier(nil), path[:n-3]...)
	p := bent[len(bent)-1].Finish()
	d := bent[len(bent)-1].TangentAt(1)
	q := p.Plus(d.WithMagnitude(12)).Plus(d.Cross(v(0, 0, 1)).WithMagnitude(2))
	bent = append(bent, hermite(p, d, q, d), srf.SBezierFrom2(q, q.Plus(d.Cross(v(0, 0, 1)).WithMagnitude(12))))
	cases := []struct {
		name string
		path []srf.SBezier
		opts topo.PathSweepOptions
		want error
	}{
		{"curve beside", bent, topo.PathSweepOptions{}, topo.ErrPathCornerCurved},
		{"twisted with no reach", path, topo.PathSweepOptions{Twist: 1}, topo.ErrPathCornerTwisted},
		{"twisted with nowhere to twist", path[n-3 : n-1], topo.PathSweepOptions{Twist: 1, Reach: 10}, topo.ErrPathCornerTwisted},
		{"held off", path, topo.PathSweepOptions{Up: v(1, 1, 1)}, topo.ErrPathCornerHeld},
	}
	for _, c := range cases {
		_, err := topo.NewSweepPath(c.path, c.path[0].TangentAt(0).NormalDir(0), c.opts)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	// Held to the corner's axis, the frames turn with it.
	axis := path[n-3].TangentAt(0).Cross(path[n-2].TangentAt(0))
	if _, err := topo.NewSweepPath(path, v(1, 0, 0), topo.PathSweepOptions{Up: axis}); err != nil {
		t.Errorf("held to the corner's axis: %v", err)
	}
	// A profile wider than the lines are long.
	sp := newPath(t, path, topo.PathSweepOptions{})
	o, t0 := path[0].Start(), path[0].TangentAt(0).WithMagnitude(1)
	e1 := t0.NormalDir(0).WithMagnitude(1)
	e2 := t0.Cross(e1)
	var sq []srf.SBezier
	pts := []geom.Vector{}
	for _, xy := range [][2]float64{{-15, -15}, {15, -15}, {15, 15}, {-15, 15}} {
		pts = append(pts, o.Plus(e1.ScaledBy(xy[0])).Plus(e2.ScaledBy(xy[1])))
	}
	for i := range pts {
		sq = append(sq, srf.SBezierFrom2(pts[i], pts[(i+1)%4]))
	}
	if _, _, err := sp.SweptSurfaces([][]srf.SBezier{sq}); !errors.Is(err, topo.ErrPathCornerShort) {
		t.Errorf("a profile 30 across mitred on lines of 12: %v", err)
	}
}

// profileReach is how far the loops reach from the path's start.
func profileReach(path []srf.SBezier, loops [][]srf.SBezier) float64 {
	o, r := path[0].Start(), 0.0
	for _, l := range loops {
		for _, sb := range l {
			for _, p := range sb.Ctrl[:sb.Degree+1] {
				r = math.Max(r, p.Minus(o).Magnitude())
			}
		}
	}
	return r
}

// A twisted sweep holds its twist still along the lines beside a corner,
// over the stretch its mitre spans, and twists along the rest: the
// corner is mitred as an untwisted one is, the solid sound, and the
// profile, about the path, keeps its area times the path's length.
func TestPathSweepTwistedCorner(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	r := rand.New(rand.NewSource(3))
	for _, c := range []struct{ angle, lean, twist float64 }{{30, 0, 1}, {90, 0, -2}, {120, 0, 3}, {60, 40, 1.5}} {
		path := cornerPath(r, c.angle, c.lean)
		for kind := range 3 {
			loops, area := profileAcross(path, kind)
			sp := newPath(t, path, topo.PathSweepOptions{Tol: 5e-4, Twist: c.twist, Reach: profileReach(path, loops)})
			if len(sp.Pieces()) <= len(path) {
				t.Errorf("corner %g, profile %d: no line split for the held stretch", c.angle, kind)
			}
			// The frames don't turn along a piece beside the corner.
			ps := sp.Pieces()
			sharp := func(k int) bool {
				return k > 0 && k < len(ps) && ps[k-1].TangentAt(1).WithMagnitude(1).Dot(ps[k].TangentAt(0).WithMagnitude(1)) < 0.99
			}
			q := path[0].Start().Plus(v(0, 0, 1))
			for i, pc := range ps {
				if !sharp(i) && !sharp(i+1) {
					continue
				}
				// Short of the piece's end: at it the frame is the next piece's.
				const e = 1e-3
				d := pc.Finish().Minus(pc.Start())
				turned := sp.Motion(float64(i) + 1 - e)(q).Minus(sp.Motion(float64(i))(q)).Minus(d.ScaledBy(1 - e))
				if turned.Magnitude() > 1e-9 {
					t.Errorf("corner %g: piece %d beside the corner turns the frames by %g", c.angle, i, turned.Magnitude())
				}
			}
			if s := sp.TwistShare(sp.Length()); math.Abs(s-1) > 1e-12 {
				t.Errorf("corner %g: twist share %g at the end", c.angle, s)
			}
			sh, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
			if err != nil {
				t.Errorf("corner %g leaning %g twist %g, profile %d: %v", c.angle, c.lean, c.twist, kind, err)
				continue
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			rep := check.Solid(sh, &m)
			perimeter := 0.0
			for _, l := range loops {
				perimeter += pathLength(l)
			}
			want := area * pathLength(path)
			allow := tol.ChordTolMm() * perimeter * pathLength(path)
			t.Logf("corner %g leaning %g twist %g, profile %d: volume %.4f, want %.4f within %.3g", c.angle, c.lean, c.twist, kind, rep.Volume, want, allow)
			if !rep.Sound() || math.Abs(rep.Volume-want) > allow {
				t.Errorf("corner %g leaning %g twist %g, profile %d: %s, want volume %.4f within %.3g",
					c.angle, c.lean, c.twist, kind, rep.String(), want, allow)
			}
		}
	}
}
