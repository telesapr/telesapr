package topo_test

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// The trough: a block 30 x 20 whose top is a cylinder of radius 20
// about the line x = 0, z = 40 along y, concave, 20 high at x = 0 and
// 21.67 at the pocket's ends; a pocket 16 x 6 through it, its floor at
// z = 18, so that its walls stand 2 high where the top is lowest.
const (
	troughR     = 20.0
	troughAxis  = 40.0
	troughFloor = 18.0
	troughHalfX = 8.0
)

// troughPocket is the trough with its pocket, at tol.
func troughPocket(tol srf.Tolerance) *srf.SShell {
	gray := core.NewRGBA(100, 100, 100, 255)
	top := func(x float64) geom.Vector { return v(x, 0, troughAxis-math.Sqrt(troughR*troughR-x*x)) }
	p0, p1, c := top(15), top(-15), v(0, 0, troughAxis)
	a0, a1 := p0.Minus(c), p1.Minus(c)
	half := math.Acos(a0.Dot(a1)/(troughR*troughR)) / 2
	arc := srf.SBezierFrom3(p0, c.Plus(a0.Plus(a1).WithMagnitude(troughR/math.Cos(half))), p1)
	arc.Weight[1] = math.Cos(half)
	var ls srf.SBezierLoopSet
	ls.Outer.Curves = append(ls.Outer.Curves,
		srf.SBezierFrom2(v(-15, 0, 0), v(15, 0, 0)), srf.SBezierFrom2(v(15, 0, 0), p0), arc,
		srf.SBezierFrom2(p1, v(-15, 0, 0)))
	ls.Normal = v(0, 1, 0)
	block, pocket, out := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
	block.MakeFromExtrusionOf(&ls, v(0, -10, 0), v(0, 10, 0), gray)
	pocket.MakePrism([]geom.Vector{v(-troughHalfX, -3, troughFloor), v(troughHalfX, -3, troughFloor),
		v(troughHalfX, 3, troughFloor), v(-troughHalfX, 3, troughFloor)}, v(0, 0, 0), v(0, 0, 30-troughFloor), gray)
	out.MakeFromDifferenceOf(&block, &pocket)
	return &out
}

// troughTaken is what a fillet (or with chamfer, a chamfer) of r of the
// trough pocket's y+ rim edge takes, cut by the floor's plane and the
// end walls' planes x = ±8: in each plane through the top's axis the
// section is the same spandrel (or triangle), s out from the top's
// cylinder and w out from the wall y = 3, swept round the axis over
// the angles its points' x lies within ±8 at and above the floor. By
// the midpoint rule over the section, the angle exact.
func troughTaken(r float64, chamfer bool) float64 {
	const n = 1200
	h := r / n
	vol := 0.0
	for i := range n {
		s := (float64(i) + 0.5) * h
		rho := troughR + s
		th := math.Asin(math.Min(1, troughHalfX/rho))
		if lift := troughAxis - troughFloor; rho > lift {
			th -= math.Acos(lift / rho)
		}
		if th <= 0 {
			continue
		}
		for j := range n {
			w := (float64(j) + 0.5) * h
			in := s+w <= r
			if !chamfer {
				in = (s-r)*(s-r)+(w-r)*(w-r) >= r*r
			}
			if in {
				vol += rho * 2 * th * h * h
			}
		}
	}
	return vol
}

// The trough pocket's y+ rim edge, rounded or chamfered: the top is
// lowest at its middle, where from 2 on the foot on the wall runs below
// the wall's foot on the floor and comes back (rollingrunoff_dip.go),
// the floor's plane cutting the fillet there. The fillets within 1e-4
// of the integral (troughTaken), the floor's cut and the end walls'
// in it. The chamfers take 0.08% to 0.12% less than it before any of
// them reaches the floor (0.5 to 1.9, at chord tolerances of 0.01% and
// 0.002% alike), and on past it as smoothly, 0.13% to 0.16% less from
// 2.01 to 3: the integral cuts the chamfer's triangles square to the
// edge by the end walls' planes, where the chamfer's end sections are
// its own (chamferFootNormals).
func TestFilletTroughPastFloor(t *testing.T) {
	t.Parallel()
	for _, c := range sweep.Pick([]struct {
		r      float64
		flat   bool
		lo, hi float64
	}{
		{1.5, false, -1e-4, 1e-4},
		{2.001, false, -1e-4, 1e-4},
		{2.5, false, -1e-4, 1e-4},
		{3, false, -1e-4, 1e-4},
		{1.5, true, -0.0012, -0.0008},
		{2.001, true, -0.0015, -0.0011},
		{2.5, true, -0.0017, -0.0012},
		{3, true, -0.0018, -0.0013},
	}, 0) {
		t.Run(fmt.Sprintf("r%g flat %v", c.r, c.flat), func(t *testing.T) {
			t.Parallel()
			sh := troughPocket(fine(0.01))
			before := surfaceVolume(t, sh)
			pick := []topo.FilletPick{{Arc: true, At: v(0, 3, troughAxis-troughR)}}
			if err := topo.FilletPicks(sh, pick, c.r, 1<<30, c.flat); err != nil {
				if fe, ok := errors.AsType[*topo.FilletError](err); ok {
					t.Fatalf("%v at %v", err, fe.At)
				}
				t.Fatal(err)
			}
			sound(t, sh)
			took, want := before-surfaceVolume(t, sh), troughTaken(c.r, c.flat)
			t.Logf("took %.6f, the integral %.6f, %+.5f", took, want, took/want-1)
			if d := took/want - 1; d < c.lo || d > c.hi {
				t.Errorf("took %.6f, the integral %.6f", took, want)
			}
		})
	}
}

// The trough's y+ rim edge at 2, its walls' height where the top is
// lowest: the foot touches the floor there, the wall pinched to a
// point, which neither cut nor left whole comes out closed. Refused by
// name before 2026-11-23; now made at the nearest size within the chord
// tolerance at which the foot crosses the floor's edge or stays clear
// of it (footAtEdgeSize), sound and within the integral at that size as
// TestFilletTroughPastFloor's are. At 3.5 the foot runs below the floor
// all along, which an end's run-off refuses.
func TestFilletTroughTouchesFloor(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		r      float64
		flat   bool
		lo, hi float64
	}{
		{2, false, -1e-4, 1e-4},
		{2, true, -0.0015, -0.0011},
	} {
		t.Run(fmt.Sprintf("r%g flat %v", c.r, c.flat), func(t *testing.T) {
			t.Parallel()
			sh := troughPocket(fine(0.01))
			before := surfaceVolume(t, sh)
			pick := []topo.FilletPick{{Arc: true, At: v(0, 3, troughAxis-troughR)}}
			made, err := topo.FilletPicksMade(sh, pick, c.r, 1<<30, c.flat)
			if err != nil {
				t.Fatal(err)
			}
			if made == c.r || math.Abs(made-c.r) > sh.Tol.ChordTolMm()/(math.Sqrt2-1) {
				t.Errorf("made at %g, want a size near %g but not it", made, c.r)
			}
			sound(t, sh)
			took, want := before-surfaceVolume(t, sh), troughTaken(made, c.flat)
			t.Logf("made at %.6f, took %.6f, the integral %.6f, %+.5f", made, took, want, took/want-1)
			if d := took/want - 1; d < c.lo || d > c.hi {
				t.Errorf("took %.6f, the integral %.6f", took, want)
			}
		})
	}
	sh := troughPocket(fine(0.01))
	pick := []topo.FilletPick{{Arc: true, At: v(0, 3, troughAxis-troughR)}}
	if err := topo.FilletPicks(sh, pick, 3.5, 1<<30, false); err == nil || !strings.Contains(err.Error(), "runs off the face on side 2 all along") {
		t.Errorf("r 3.5: got %v, want a refusal of a foot below the floor all along", err)
	}
}
