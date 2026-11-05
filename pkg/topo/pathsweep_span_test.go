package topo_test

import (
	"errors"
	"math"
	"math/rand"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A fitted sweep made in spans: each span, capped where it is cut, is
// sound, and their volumes add up to the whole sweep's, twisted and
// round a mitred corner alike, the cuts kept out of the straight pieces
// beside the corner. A path with a run swept rigidly beside a corner at
// an arc can't be cut.
func TestPathSweepSpans(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.005, 100)
	col := core.NewRGBA(100, 100, 100, 255)
	r := rand.New(rand.NewSource(3))
	for _, c := range []struct {
		name string
		path []srf.SBezier
		opts topo.PathSweepOptions
	}{
		{"twisted", wanderPath(r, 3, 10, true), topo.PathSweepOptions{Tol: 5e-4, Twist: math.Pi / 2}},
		{"corner", cornerPath(r, 90, 0), topo.PathSweepOptions{Tol: 5e-4}},
	} {
		loops, _ := profileAcross(c.path, 1)
		sp := newPath(t, c.path, c.opts)
		if !sp.Spannable() {
			t.Fatalf("%s: not spannable", c.name)
		}
		whole, err := topo.MakePathSweep(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2), tol)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		want := volumeOf(whole)
		cuts := []float64{0}
		for _, f := range []float64{0.3, 0.6, 0.9} {
			m, ok := sp.CutAt(0, sp.End(), f)
			if !ok {
				t.Fatalf("%s: no cut at %g", c.name, f)
			}
			cuts = append(cuts, m)
		}
		cuts = append(cuts, sp.End())
		got := 0.0
		for k := range len(cuts) - 1 {
			sh, err := topo.MakePathSweepSpan(sp, loops, col, srf.OriginFor(9, 0), srf.OriginFor(9, 1), srf.OriginFor(9, 2),
				tol, cuts[k], cuts[k+1])
			if err != nil {
				t.Fatalf("%s: span %g to %g: %v", c.name, cuts[k], cuts[k+1], err)
			}
			var m srf.SMesh
			sh.TriangulateInto(&m)
			if rep := check.Solid(sh, &m); !rep.Sound() {
				t.Errorf("%s: span %g to %g: %s", c.name, cuts[k], cuts[k+1], rep.String())
			}
			got += volumeOf(sh)
		}
		if math.Abs(got-want) > 1e-3*want {
			t.Errorf("%s: the spans' volumes add to %.4f, the whole's %.4f", c.name, got, want)
		}
	}

	// Round the corner the straight pieces beside it, the path's last
	// but one and two, are taken whole: a cut that falls in one moves to
	// its end away from the corner, and a span of just the two has none.
	path := cornerPath(r, 90, 0)
	sp := newPath(t, path, topo.PathSweepOptions{Tol: 5e-4})
	lo, hi := float64(len(path)-3), float64(len(path)-1)
	for k := range 20 {
		f := (float64(k) + 0.5) / 20
		m, ok := sp.CutAt(0, sp.End(), f)
		if ok && m > lo && m < hi {
			t.Errorf("cut at %g lands at %g, inside a straight piece beside the corner", f, m)
		}
	}
	if m, ok := sp.CutAt(lo, hi, 0.5); ok {
		t.Errorf("the two straight pieces about the corner were cut, at %g", m)
	}
}

// A path with a sharp corner beside an arc sweeps the corner's run
// rigidly, and is not cut in spans.
func TestPathSweepSpanRigid(t *testing.T) {
	t.Parallel()
	// Up the z axis, then at a right angle a quarter circle about
	// (0, 0, 20) toward x.
	arc := srf.SBezierFrom3(v(0, 0, 10), v(10, 0, 10), v(10, 0, 20))
	arc.Weight[1] = math.Sqrt2 / 2
	path := []srf.SBezier{srf.SBezierFrom2(v(0, 0, 0), v(0, 0, 10)), arc}
	sp, err := topo.NewSweepPath(path, v(0, 1, 0), topo.PathSweepOptions{Tol: 5e-4, Reach: 1})
	if err != nil {
		t.Fatal(err)
	}
	if sp.Spannable() {
		t.Fatal("a corner beside an arc swept with no rigid run")
	}
	loops, _ := profileAcross(path, 1)
	if _, err := topo.MakePathSweepSpan(sp, loops, core.NewRGBA(1, 1, 1, 255), 1, 2, 3, srf.Tol(0.005, 100), 0, 1); !errors.Is(err, topo.ErrSpanRigid) {
		t.Errorf("got %v, want ErrSpanRigid", err)
	}
}

// volumeOf is the volume of the shell's mesh.
func volumeOf(sh *srf.SShell) float64 {
	var m srf.SMesh
	sh.TriangulateInto(&m)
	return m.CalculateVolume()
}
