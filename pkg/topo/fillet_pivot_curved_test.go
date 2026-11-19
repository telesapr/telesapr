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

// leaningWall is a pocket's wall on a cylinder lying on its side, its
// axis along the pocket's other way, at height cz and across at c, of
// radius rad: the wall's place across at height z.
type leaningWall struct{ c, cz, rad float64 }

func (w leaningWall) at(z float64) float64 {
	return w.c + math.Sqrt(w.rad*w.rad-(z-w.cz)*(z-w.cz))
}

// arcTo is the wall's arc from height z0 to z1 in the plane the profile
// is drawn in, point(across, z) placing it.
func (w leaningWall) arcTo(z0, z1 float64, point func(a, z float64) geom.Vector) srf.SBezier {
	p0, p1 := point(w.at(z0), z0), point(w.at(z1), z1)
	c := point(w.c, w.cz)
	a0, a1 := p0.Minus(c), p1.Minus(c)
	half := math.Acos(a0.Dot(a1)/(w.rad*w.rad)) / 2
	arc := srf.SBezierFrom3(p0, c.Plus(a0.Plus(a1).WithMagnitude(w.rad/math.Cos(half))), p1)
	arc.Weight[1] = math.Cos(half)
	return arc
}

// The pockets' leaning walls: x+ widening upward (12 at z = 10, 17.83
// at z = 20), and y+ (2.9 at z = 11, 8 at z = 20).
var (
	wallX = leaningWall{-28, 40, 50}
	wallY = leaningWall{8 - math.Sqrt(2500-400), 40, 50}
)

// curvedPocket is a pocket whose x+ wall leans (wallX), and with
// curvedY its y+ wall too (wallY), its other walls x = -12 and y = -6,
// from floor up through the top of a 50 x 30 x 20 block (dish false) or
// of dishPocket's dish. The crease where the x+ wall meets a y wall is
// an arc in the y wall's plane, or with curvedY where it meets the y+
// wall, the two cylinders' curve of intersection.
func curvedPocket(dish, curvedY bool, tol srf.Tolerance) *srf.SShell {
	floor := 8.0
	if dish {
		floor = 4
	}
	return curvedPocketFloor(dish, curvedY, floor, tol)
}

// curvedPocketFloor is curvedPocket with its floor at height floor.
func curvedPocketFloor(dish, curvedY bool, floor float64, tol srf.Tolerance) *srf.SShell {
	var base *srf.SShell
	if dish {
		base = lathed([]geom.Vector{v(0, 0, 0), v(40, 0, 0), v(40, 0, 20), v(0, 0, 5)}, tol)
	} else {
		base = &srf.SShell{Tol: tol}
		base.MakePrism([]geom.Vector{v(-20, -15, 0), v(30, -15, 0), v(30, 15, 0), v(-20, 15, 0)}, v(0, 0, 0), v(0, 0, 20),
			core.NewRGBA(100, 100, 100, 255))
	}
	gray := core.NewRGBA(100, 100, 100, 255)
	const top = 25.0
	xz := func(a, z float64) geom.Vector { return v(a, 0, z) }
	var lx srf.SBezierLoopSet
	lx.Outer.Curves = append(lx.Outer.Curves,
		srf.SBezierFrom2(v(-12, 0, floor), xz(wallX.at(floor), floor)), wallX.arcTo(floor, top, xz),
		srf.SBezierFrom2(xz(wallX.at(top), top), v(-12, 0, top)), srf.SBezierFrom2(v(-12, 0, top), v(-12, 0, floor)))
	lx.Normal = v(0, 1, 0)
	py := 6.0
	if curvedY {
		py = 14
	}
	px := srf.SShell{Tol: tol}
	px.MakeFromExtrusionOf(&lx, v(0, -6, 0), v(0, py, 0), gray)
	pocket := &px
	if curvedY {
		yz := func(a, z float64) geom.Vector { return v(0, a, z) }
		var ly srf.SBezierLoopSet
		ly.Outer.Curves = append(ly.Outer.Curves,
			srf.SBezierFrom2(v(0, -6, floor), yz(wallY.at(floor), floor)), wallY.arcTo(floor, top, yz),
			srf.SBezierFrom2(yz(wallY.at(top), top), v(0, -6, top)), srf.SBezierFrom2(v(0, -6, top), v(0, -6, floor)))
		ly.Normal = v(-1, 0, 0)
		qy, both := srf.SShell{Tol: tol}, srf.SShell{Tol: tol}
		qy.MakeFromExtrusionOf(&ly, v(-14, 0, 0), v(24, 0, 0), gray)
		both.MakeFromIntersectionOf(&px, &qy)
		pocket = &both
	}
	out := srf.SShell{Tol: tol}
	out.MakeFromDifferenceOf(base, pocket)
	return &out
}

// curvedRim are picks of curvedPocket's rim edges by a point on each,
// "x+" the edge on the wall at x+ and so on.
func curvedRim(dish, curvedY bool, names ...string) []topo.FilletPick {
	topAt := func(x, y float64) float64 {
		if dish {
			return 5 + 15.0/40*math.Hypot(x, y)
		}
		return 20
	}
	// The rim's point on a leaning wall, a fixed point of the wall's
	// place across at the top's height there.
	onWall := func(w leaningWall, other float64, swap bool) geom.Vector {
		a := w.at(15)
		for range 100 {
			x, y := a, other
			if swap {
				x, y = other, a
			}
			a = w.at(topAt(x, y))
		}
		if swap {
			return v(other, a, topAt(other, a))
		}
		return v(a, other, topAt(a, other))
	}
	yPlus := v(1, 6, topAt(1, 6))
	if curvedY {
		yPlus = onWall(wallY, 1, true)
	}
	at := map[string]geom.Vector{
		"x+": onWall(wallX, 1, false), "x-": v(-12, 1, topAt(-12, 1)),
		"y+": yPlus, "y-": v(1, -6, topAt(1, -6)),
	}
	var out []topo.FilletPick
	for _, n := range names {
		out = append(out, topo.FilletPick{Arc: true, At: at[n]})
	}
	return out
}

// Rim edges of curvedPocket meeting at its inside corners, where the
// crease between the two walls is curved: the corner patch's swing is
// re-centred along the crease (rollingpivot_crease.go), or where the two
// feet on the crease are one point, as where both walls lean alike in a
// flat top, the two fillets meet in a mitre. Refused by name before
// 2026-11-19 but for that mitre. OpenCASCADE's makeFillet and makeChamfer
// of the same edges (scripts/fillet-pivot-curved-fixtures.py, FreeCADCmd,
// 2026-11-19) took the volumes below; ours, the mesh carried to its
// surfaces (surfaceVolume), take lo to hi of them. The mitres agree to
// 1e-5. A corner patch takes less than OpenCASCADE's, which runs each
// fillet on into the other past the crease where ours fills between
// their ends, as at a straight crease (TestFilletPivotCorner): in the
// block about 0.026 r³ a fillet's corner and 0.064 d³ a chamfer's,
// measured against each edge alone, whose volumes agree to 3e-5; in the
// dish more, where OpenCASCADE's Volume of its own solid is off by a few
// tenths of a mm³ (a y+ fillet alone with both walls leaning measures a
// fifth more than ours, which is within 0.1% of the ball's envelope,
// TestFilletEndLeaningWall).
func TestFilletPivotCurvedCrease(t *testing.T) {
	if testing.Short() {
		t.Skip("rolling-ball fillets and chamfers of pockets' rims with curved creases at their corners, slow")
	}
	t.Parallel()
	for _, c := range sweep.Pick([]struct {
		dish, curvedY bool
		picks         string
		r             float64
		flat          bool
		occ, lo, hi   float64
	}{
		{false, false, "x+,y+", 1, false, 7.299546, -0.005, -0.002},
		{false, false, "x+,y+", 2, false, 29.189114, -0.009, -0.005},
		{false, false, "x+,y-", 1, false, 7.299546, -0.005, -0.002},
		{false, false, "x+,y+,x-,y-", 1, false, 16.487816, -0.005, -0.002},
		{false, false, "x+,y+", 1, true, 20.610428, -0.005, -0.002},
		{false, false, "x+,y+,x-,y-", 1.5, true, 96.709966, -0.006, -0.003},
		{false, true, "x+,y+", 1, false, 3.219632, -1e-4, 1e-4},
		{false, true, "x+,y+", 2, false, 12.618824, -1e-4, 1e-4},
		{false, true, "x+,y+,x-,y-", 1, false, 12.761880, -0.006, -0.003},
		{false, true, "x+,y+", 1, true, 20.102175, -1e-4, 1e-4},
		{false, true, "x+,y+,x-,y-", 1.5, true, 97.302289, -0.006, -0.003},
		{true, false, "x+,y+", 1, false, 2.879500, -0.02, -0.012},
		{true, false, "x+,y+", 2, false, 11.189964, -0.035, -0.025},
		{true, false, "x+,y-", 1.5, false, 6.382695, -0.026, -0.017},
		{true, false, "x+,y+,x-,y-", 1, false, 6.810288, -0.022, -0.014},
		{true, false, "x+,y+", 1, true, 15.201804, -0.016, -0.008},
		{true, false, "x+,y+,x-,y-", 1.5, true, 75.954362, -0.026, -0.018},
		{true, true, "x+,y+", 1, false, 1.072038, -0.027, -0.018},
		{true, true, "x+,y+", 1.5, false, 2.280799, -0.017, -0.009},
		{true, true, "x+,y-", 1, false, 2.875632, -0.035, -0.027},
		{true, true, "x+,y+", 1, true, 12.220162, -0.032, -0.024},
		{true, true, "x+,y+,x-,y-", 1, false, 4.588486, -0.026, -0.016},
		{true, true, "x+,y+,x-,y-", 1, true, 28.408016, -0.029, -0.021},
	}, 0) {
		t.Run(fmt.Sprintf("dish %v curvedY %v %s r%g flat %v", c.dish, c.curvedY, c.picks, c.r, c.flat), func(t *testing.T) {
			t.Parallel()
			sh := curvedPocket(c.dish, c.curvedY, fine(0.01))
			before := surfaceVolume(t, sh)
			if err := topo.FilletPicks(sh, curvedRim(c.dish, c.curvedY, strings.Split(c.picks, ",")...), c.r, 1<<30, c.flat); err != nil {
				if fe, ok := errors.AsType[*topo.FilletError](err); ok {
					t.Fatalf("%v at %v", err, fe.At)
				}
				t.Fatal(err)
			}
			if off, where := stepEdgesOffFaces(t, sh); off > 1e-6 {
				t.Errorf("written as STEP, an edge lies %.3g off its face: %s", off, where)
			}
			sound(t, sh)
			took := before - surfaceVolume(t, sh)
			t.Logf("took %.6f, OpenCASCADE %.6f, %+.5f", took, c.occ, took/c.occ-1)
			if took < c.occ*(1+c.lo) || took > c.occ*(1+c.hi) {
				t.Errorf("took %.6f, OpenCASCADE %.6f", took, c.occ)
			}
		})
	}
}

// One corner of curvedPocket's rim in the block's flat top at a coarse
// tolerance, quickly: the fillet's patch about a crease that is an arc,
// sound.
func TestFilletPivotCurvedCreaseQuick(t *testing.T) {
	t.Parallel()
	sh := curvedPocket(false, false, fine(0.05))
	if err := topo.FilletPicks(sh, curvedRim(false, false, "x+", "y+"), 1, 1<<30, false); err != nil {
		t.Fatal(err)
	}
	sound(t, sh)
}

// The x- and y+ edges of curvedPocket's rim in the dish with both walls
// leaning, filleted: at their corner the crease, an arc in the x- wall's
// plane, leans back 26° under the y+ fillet, whose foot on it lies
// nearer the vertex. Refused as not closed before 2026-11-19, the
// corner's sections leaving the crease along the y+ wall's face and
// crossing that fillet's mesh near its end; its foot on the crease now
// stands at the y+ foot while the ball would touch the crease above it
// (pivotFrame.hold). Sound at a chord tolerance of 0.002 too (by hand,
// 2026-11-19: 40 s).
func TestFilletPivotCurvedCreaseLeansBack(t *testing.T) {
	if testing.Short() {
		t.Skip("a rolling-ball fillet of a pocket's rim in a cone with a curved crease, slow")
	}
	t.Parallel()
	sh := curvedPocket(true, true, fine(0.01))
	if err := topo.FilletPicks(sh, curvedRim(true, true, "x-", "y+"), 1, 1<<30, false); err != nil {
		t.Fatal(err)
	}
	sound(t, sh)
}

// The x- and y- edges of curvedPocket's rim in the dish, filleted, at a
// chord tolerance of 0.002: their corner's crease, where the two flat
// walls meet, is straight, and leans back 9° under the x- fillet, whose
// foot on it lies nearer the vertex, the dish's top falling away from
// the corner along that edge. Refused as not closed at this tolerance
// before 2026-11-19 (sound at 0.005 and coarser): the corner's foot
// moved down the crease from the start, and the corner folded over the
// x- wall and fillet by a few micrometres at that foot. Its foot now
// stands there as at a curved crease leaning back (creaseDepth). The
// whole rim, with either wall or both leaning, at radii 1 to 2, came out
// the same way; all are sound now at 0.002 and 0.005.
func TestFilletPivotStraightCreaseLeansBack(t *testing.T) {
	if testing.Short() {
		t.Skip("a rolling-ball fillet of a pocket's rim in a cone at a fine chord tolerance, slow")
	}
	t.Parallel()
	sh := curvedPocket(true, false, fine(0.002))
	if err := topo.FilletPicks(sh, curvedRim(true, false, "x-", "y-"), 1, 1<<30, false); err != nil {
		t.Fatal(err)
	}
	sound(t, sh)
}
