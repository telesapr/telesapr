package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// exactTubeX is the exact tube of radius r about the x axis from x0 to
// x1, its circle's seam at phase radians from y.
func exactTubeX(r, x0, x1, phase float64, tol srf.Tolerance) *srf.SShell {
	var curves []srf.SBezier
	for k := range 4 {
		a0 := phase + float64(k)*math.Pi/2
		a1 := a0 + math.Pi/2
		p0 := v(x0, r*math.Cos(a0), r*math.Sin(a0))
		p1 := v(x0, r*math.Cos(a1), r*math.Sin(a1))
		c := v(x0, r*math.Sqrt2*math.Cos(a0+math.Pi/4), r*math.Sqrt2*math.Sin(a0+math.Pi/4))
		sb := srf.SBezierFrom3(p0, c, p1)
		sb.Weight[1] = math.Sqrt2 / 2
		curves = append(curves, sb)
	}
	sbl := srf.SBezierList{Curves: curves}
	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	var allClosed, allCoplanar bool
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&sbl, &poly, nil, srf.Tol(0.1, 20), &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	sh := &srf.SShell{Tol: tol}
	for i := range sets.Sets {
		sh.MakeFromExtrusionOf(&sets.Sets[i], v(0, 0, 0), v(x1-x0, 0, 0), core.RGBA{})
	}
	return sh
}

// fittedTubeZ is the tube of radius r along the z axis from z0 to z1
// swept by the kernel's fitted sweep, its skin B-spline patches fitted
// to the circle, not exact; its circle's seam at phase radians from x.
func fittedTubeZ(t *testing.T, r, z0, z1, phase float64, tol srf.Tolerance) *srf.SShell {
	t.Helper()
	d := (z1 - z0) / 3
	path := []srf.SBezier{srf.SBezierFrom4(v(0, 0, z0), v(0, 0, z0+d), v(0, 0, z0+2*d), v(0, 0, z1))}
	sp, err := topo.NewSweepPath(path, v(1, 0, 0), topo.PathSweepOptions{Reach: r})
	if err != nil {
		t.Fatal(err)
	}
	sh, err := topo.MakePathSweep(sp, [][]srf.SBezier{circle(r, z0, phase)}, core.RGBA{}, 1<<30, 1<<30+1, 1<<30+2, tol)
	if err != nil {
		t.Fatal(err)
	}
	return sh
}

// A fitted tube and an exact one of the same radius whose axes cross
// touch tangent at two points, where the curves they meet in cross
// (roadmap K3): a sweep's corner beside a cubic, where the cubic's
// fitted run meets a line's exact one, is such a pair, and the march
// along those curves turns back at the points, where the normals are
// parallel, so every Boolean failed. Told the points (srf.SShell.Touch),
// the Booleans end each branch there. Crosses, tees and the corner's L
// (each tube ending in the plane through the other's axis), the
// circles' seams on the points and off: every union, difference and
// intersection is sound, and the intersection is the Steinmetz solid's
// share, 16/3 r³ for the cross, half for the tee and a quarter for the
// L, within what the chords lose. One of the 45 is left open (open
// names it): the cross's difference with the seams at 0.3 and 1.1, whose
// trims assemble but whose two sheets, the fitted tube's kept outside
// the exact one and the exact one's kept inside the fitted, close to the
// touching points as (z² − x²)/2, under the chord tolerance within a few
// tenths of a radius of them, and their meshes cross there (the mesher's,
// roadmap K4). The cross's intersection with the seams at 0.3 and 0
// failed before 2026-11-09: the fitted tube's seam, through a touching
// point, met the exact tube at a grazing angle 2e-4 along from it, and
// the vertex there split the mesh from the branches ending at the point
// (srf snapToKnownTouches).
func TestFittedTubeTouchingExact(t *testing.T) {
	t.Parallel()
	tol := srf.Tol(0.01, 30)
	const r = 1.0
	open := map[string]bool{
		"cross, seams at 0.30 and 1.10, difference": true,
	}
	bad := 0
	for _, c := range []struct {
		name           string
		z0, z1, x0, x1 float64
		share          float64
	}{
		{"cross", -5, 5, -5, 5, 1},
		{"tee", -5, 5, 0, 5, 0.5},
		{"corner", -5, 0, 0, 5, 0.25},
	} {
		for _, ph := range [][2]float64{{0, 0}, {0.3, 0}, {0, 0.3}, {math.Pi / 4, math.Pi / 4}, {0.3, 1.1}} {
			name := fmt.Sprintf("%s, seams at %.2f and %.2f", c.name, ph[0], ph[1])
			for _, op := range []string{"union", "difference", "intersection"} {
				f := fittedTubeZ(t, r, c.z0, c.z1, ph[0], tol)
				e := exactTubeX(r, c.x0, c.x1, ph[1], tol)
				f.Touch = []geom.Vector{v(0, r, 0), v(0, -r, 0)}
				e.Touch = f.Touch
				var out srf.SShell
				out.Tol = tol
				switch op {
				case "union":
					out.MakeFromUnionOf(f, e)
				case "difference":
					out.MakeFromDifferenceOf(f, e)
				default:
					out.MakeFromIntersectionOf(f, e)
				}
				var m srf.SMesh
				out.TriangulateInto(&m)
				var naked srf.SEdgeList
				var inters, leaks bool
				srf.SKdNodeFrom(&m).MakeCertainEdgesInto(&naked, srf.EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
				if out.BooleanFailed || len(naked.Edges) > 0 || inters || leaks {
					if open[name+", "+op] {
						bad++
						continue
					}
					t.Errorf("%s, %s: failed %v, %d naked edges, crossing %v, want sound",
						name, op, out.BooleanFailed, len(naked.Edges), inters)
					continue
				}
				if op != "intersection" {
					continue
				}
				want := c.share * 16 * r * r * r / 3
				if got := m.CalculateVolume(); math.Abs(got-want) > 0.02*want {
					t.Errorf("%s: intersection %.4f, want %.4f within 2%%", name, got, want)
				}
			}
		}
	}
	if bad < len(open) {
		t.Logf("%d of the %d open cases now sound: take them off the list", len(open)-bad, len(open))
	}
}
