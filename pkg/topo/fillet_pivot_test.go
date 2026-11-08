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

// dishPocket is a dish lathed about the z axis, 40 in radius and 20
// high at its rim, its top a cone down to 5 at the axis, with a 24 x 12
// pocket from z = 2 up through its top, built at tol. The pocket's rim
// edges are curves on the cone, which slopes three times as steeply
// along x at its corners as along y.
func dishPocket(tol srf.Tolerance) *srf.SShell {
	dish := lathed([]geom.Vector{v(0, 0, 0), v(40, 0, 0), v(40, 0, 20), v(0, 0, 5)}, tol)
	pocket, out := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
	pocket.MakePrism([]geom.Vector{v(-12, -6, 2), v(12, -6, 2), v(12, 6, 2), v(-12, 6, 2)}, v(0, 0, 0), v(0, 0, 30),
		core.NewRGBA(100, 100, 100, 255))
	out.MakeFromDifferenceOf(dish, &pocket)
	return &out
}

// dishRim are picks of the dish's pocket's rim edges, by a point on
// each: "x+" the edge on the wall at x = 12, and so on.
func dishRim(names ...string) []topo.FilletPick {
	on := func(x, y float64) topo.FilletPick {
		return topo.FilletPick{Arc: true, At: v(x, y, 5+15.0/40*math.Hypot(x, y))}
	}
	at := map[string]topo.FilletPick{"x+": on(12, 1), "x-": on(-12, 1), "y+": on(1, 6), "y-": on(1, -6)}
	var out []topo.FilletPick
	for _, n := range names {
		out = append(out, at[n])
	}
	return out
}

// Rim edges of a pocket in a curved face that meet at its inside
// corners, picked together: the ball swings round the crease there, and
// a corner joins the two fillets, tangent to them and to the face they
// share. Each fillet ends where its ball, touching its wall on the
// crease, touches the cone: at the corner (12, 6) the x+ fillet's ball
// stands at (12+r, 6, zc), a radius under the cone, and its foot is
// (12, 6, zc); the y+ fillet's at (12, 6+r). OpenCASCADE's makeFillet of
// the same edges (2026-11-07) has the y+ foot there to 1e-6 and takes
// the volumes below from the dish; at these corners it runs each fillet
// on into the other and the wall a hundredth of a millimetre past the
// crease, where ours fills between their ends (the ball's own surface
// would fold on the crease, rollingpivot_corner.go), and took 3.3% less
// at a radius of 1, 2.4% at 2, measured by meshes at 0.002% (73 s a
// case). Here the meshes are coarser, their volumes some 4% off either
// way at a radius of 1: the volume only rules out a corner gone wrong,
// the feet are the check.
func TestFilletPivotCorner(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball fillets of a pocket's rim in a cone, with corners, slow")
	}
	t.Parallel()
	cases := sweep.Pick([]struct {
		picks []string
		r     float64
		occ   float64
	}{
		{[]string{"x+", "y+"}, 1, 3.920346},
		{[]string{"x+", "y+"}, 2, 15.570806},
		{[]string{"x+", "x-", "y+"}, 1, 5.038845},
		{[]string{"x+", "x-", "y+"}, 2, 20.190235},
		{[]string{"x+", "x-", "y+", "y-"}, 1, 7.903276},
		{[]string{"x+", "x-", "y+", "y-"}, 2, 31.706112},
	}, 3)
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s r%g", strings.Join(c.picks, ","), c.r), func(t *testing.T) {
			t.Parallel()
			sh := dishPocket(fine(0.01))
			before := meshVolume(sh)
			if err := topo.FilletPicks(sh, dishRim(c.picks...), c.r, 1<<30, false); err != nil {
				t.Fatal(err)
			}
			took := before - sound(t, sh)
			if took < c.occ*(1-0.08) || took > c.occ*(1+0.05) {
				t.Errorf("took %g, OpenCASCADE %g", took, c.occ)
			}
			tp := build(t, sh)
			checkCreaseEnds(t, tp)
			k := 15.0 / 40
			foot := func(x, y float64) geom.Vector {
				return v(12, 6, 5+k*math.Hypot(x, y)-c.r*math.Sqrt(1+k*k))
			}
			for _, p := range []geom.Vector{foot(12+c.r, 6), foot(12, 6+c.r)} {
				if !hasVertex(tp.Vertices, p) {
					t.Errorf("no fillet's foot at %v", p)
				}
			}
		})
	}
}

// checkCreaseEnds checks that no vertex of the pocket's creases is left
// between where a corner meets it and the rim: every vertex on a crease
// line lies below the cone's surface by more than the tolerance.
func checkCreaseEnds(t *testing.T, tp *topo.Topology) {
	t.Helper()
	for _, vx := range tp.Vertices {
		p := vx.P
		if math.Abs(math.Abs(p.X)-12) > 1e-6 || math.Abs(math.Abs(p.Y)-6) > 1e-6 {
			continue
		}
		top := 5 + 15.0/40*math.Hypot(12, 6)
		if p.Z > top-1e-3 && p.Z < top+1e-3 {
			// The rim's own corner, where no fillet came.
			continue
		}
		if p.Z > top {
			t.Errorf("a vertex of the crease above the rim, at %v", p)
		}
	}
}

// Chamfers of the same rim turn its inside corners the same way, their
// corner ruled between the crease and the cone (OpenCASCADE's
// makeChamfer refuses these). The solid must be sound, and every corner
// split from its crease.
func TestChamferPivotCorner(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball chamfers of a pocket's rim in a cone, with corners, slow")
	}
	t.Parallel()
	for _, picks := range sweep.Pick([][]string{{"x+", "y+"}, {"x+", "x-", "y+", "y-"}}, 0) {
		t.Run(strings.Join(picks, ","), func(t *testing.T) {
			t.Parallel()
			sh := dishPocket(fine(0.01))
			before := meshVolume(sh)
			if err := topo.FilletPicks(sh, dishRim(picks...), 1, 1<<30, true); err != nil {
				t.Fatal(err)
			}
			if took := before - sound(t, sh); took <= 0 {
				t.Errorf("took %g", took)
			}
			checkCreaseEnds(t, build(t, sh))
		})
	}
}

// The corner at one inside corner, quickly: the rim's two edges there
// at a coarse tolerance, sound.
func TestFilletPivotCornerQuick(t *testing.T) {
	t.Parallel()
	sh := dishPocket(fine(0.05))
	if err := topo.FilletPicks(sh, dishRim("x+", "y+"), 1, 1<<30, false); err != nil {
		t.Fatal(err)
	}
	sound(t, sh)
	checkCreaseEnds(t, build(t, sh))
}

// teardropPocket is the dish of dishPocket with a teardrop-shaped
// pocket from z = 2 up through its top instead: a circle of radius 5
// about (14, 10) and the two lines tangent to it from its point at
// (26, 10), built at tol, all within one quarter of the lathe's
// patches. Its rim is one chain, smooth but at the point, where it turns
// at an inside corner.
func teardropPocket(tol srf.Tolerance) *srf.SShell {
	dish := lathed([]geom.Vector{v(0, 0, 0), v(40, 0, 0), v(40, 0, 20), v(0, 0, 5)}, tol)
	c, rad, tip := v(14, 10, 0), 5.0, v(26, 10, 0)
	d := tip.Minus(c).WithMagnitude(1)
	n := v(-d.Y, d.X, 0)
	at := func(a float64) geom.Vector {
		return c.Plus(d.ScaledBy(rad * math.Cos(a))).Plus(n.ScaledBy(rad * math.Sin(a)))
	}
	beta := math.Acos(rad / tip.Minus(c).Magnitude())
	var ls srf.SBezierLoopSet
	ls.Outer.Curves = append(ls.Outer.Curves, srf.SBezierFrom2(tip, at(beta)))
	turn := 2*math.Pi - 2*beta
	for k := range 3 {
		a0 := beta + turn*float64(k)/3
		half := turn / 6
		mid := c.Plus(at(a0 + half).Minus(c).ScaledBy(1 / math.Cos(half)))
		sb := srf.SBezierFrom3(at(a0), mid, at(a0+2*half))
		sb.Weight[1] = math.Cos(half)
		ls.Outer.Curves = append(ls.Outer.Curves, sb)
	}
	ls.Outer.Curves = append(ls.Outer.Curves, srf.SBezierFrom2(at(2*math.Pi-beta), tip))
	ls.Normal = v(0, 0, -1)
	pocket, out := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
	pocket.MakeFromExtrusionOf(&ls, v(0, 0, 2), v(0, 0, 32), core.NewRGBA(100, 100, 100, 255))
	out.MakeFromDifferenceOf(dish, &pocket)
	return &out
}

// A chain that comes back to its own inside corner, the rim of a
// teardrop pocket in the cone, pivots there as two chains do: its two
// ends meet in a corner, and the crease under the point stops at the
// deeper foot. The solid must be sound, for a fillet and a chamfer.
func TestFilletPivotOneChain(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball fillet of a teardrop pocket's rim in a cone, slow")
	}
	t.Parallel()
	for _, flat := range []bool{false, true} {
		t.Run(fmt.Sprint("flat ", flat), func(t *testing.T) {
			t.Parallel()
			sh := teardropPocket(fine(0.01))
			pick := topo.FilletPick{Arc: true, At: v(9, 10, 5+15.0/40*math.Hypot(9, 10))}
			if err := topo.FilletPicks(sh, []topo.FilletPick{pick}, 1, 1<<30, flat); err != nil {
				if fe, ok := errors.AsType[*topo.FilletError](err); ok {
					t.Fatalf("%v at %v", err, fe.At)
				}
				t.Fatal(err)
			}
			sound(t, sh)
			top := 5 + 15.0/40*math.Hypot(26, 10)
			for _, vx := range build(t, sh).Vertices {
				p := vx.P
				if math.Abs(p.X-26) < 1e-6 && math.Abs(p.Y-10) < 1e-6 && p.Z > 2+1e-6 && math.Abs(p.Z-top) > 1e-3 && p.Z > top-0.5 {
					t.Errorf("a vertex of the crease left near the rim, at %v", p)
				}
			}
		})
	}
}

// Rim edges of a pocket in a flat face (pocketed: 20 x 10, 12 deep in a
// block's top), any of them: step 10 rounds the whole rim, and the
// rolling ball the rest, each fillet ending at the pocket's wall where
// it meets no other and mitred at an inside corner where it does, the
// two swept surfaces carried on past the crease. OpenCASCADE's
// makeFillet (2026-11-07) takes 0.2146 r² a unit of the rim's length,
// the quarter circle's spandrel, and 0.09587 r³ more at each inside
// corner two picks share: 4.292037 for one long edge at a radius of 1,
// 6.533925 for it and a short one beside it, 17.168147 and 26.519183 at
// 2. Its makeChamfer takes d²/2 and d³/3 at each such corner, exactly,
// as ours does: 15.333333 and 62.666667 for the two at 1 and 2.
func TestFilletPocketRimFlat(t *testing.T) {
	t.Parallel()
	rim := map[string]topo.FilletPick{
		"y-": {A: v(10, 10, 20), B: v(30, 10, 20)},
		"x+": {A: v(30, 10, 20), B: v(30, 20, 20)},
		"y+": {A: v(30, 20, 20), B: v(10, 20, 20)},
		"x-": {A: v(10, 20, 20), B: v(10, 10, 20)},
	}
	for _, c := range sweep.Pick([]struct {
		picks   []string
		length  float64
		corners int
		r       float64
		flat    bool
	}{
		{[]string{"y-"}, 20, 0, 1, false},
		{[]string{"y-", "y+"}, 40, 0, 1, false},
		{[]string{"y-", "x+"}, 30, 1, 1, false},
		{[]string{"y-", "x+"}, 30, 1, 2, false},
		{[]string{"y-", "x+", "y+"}, 50, 2, 1, false},
		{[]string{"x-", "y-", "x+", "y+"}, 60, 4, 1, false},
		{[]string{"y-", "x+"}, 30, 1, 1, true},
		{[]string{"y-", "x+", "y+"}, 50, 2, 2, true},
	}, 0) {
		t.Run(fmt.Sprintf("%s r%g flat %v", strings.Join(c.picks, ","), c.r, c.flat), func(t *testing.T) {
			t.Parallel()
			sh := pocketed(srf.Tol(0.001, 20))
			var picks []topo.FilletPick
			for _, k := range c.picks {
				picks = append(picks, rim[k])
			}
			if err := topo.FilletPicks(sh, picks, c.r, 1<<30, c.flat); err != nil {
				t.Fatal(err)
			}
			took := 40*30*20 - 20*10*12 - sound(t, sh)
			r2, r3 := c.r*c.r, c.r*c.r*c.r
			want := (1-math.Pi/4)*r2*c.length + float64(c.corners)*0.09587*r3
			if c.flat {
				want = r2/2*c.length + float64(c.corners)*r3/3
			}
			if math.Abs(took-want) > 0.004*want {
				t.Errorf("took %g, want %g", took, want)
			}
		})
	}
}
