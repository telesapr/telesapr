package topo_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/check"
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// pinchCone is the cone of radius 10 at z = 0 and 4 at z = 30, turned
// about z with its flat ends four patches each, or, merged, turned
// about -z with each end merged into one plane.
func pinchCone(merged bool, tol srf.Tolerance) *srf.SShell {
	p, n, axis := []geom.Vector{{}, {X: 10}, {X: 4, Z: 30}, {Z: 30}}, v(0, 1, 0), v(0, 0, 1)
	if merged {
		p, n, axis = []geom.Vector{{}, {Z: 30}, {X: 4, Z: 30}, {X: 10}}, v(0, -1, 0), v(0, 0, -1)
	}
	sbls := &srf.SBezierLoopSet{Normal: n, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, srf.SBezierFrom2(p[i], p[(i+1)%len(p)]))
	}
	sh := &srf.SShell{Tol: tol}
	sh.MakeFromRevolutionOf(sbls, geom.Vector{}, axis, core.RGBA{}, noGroup{})
	if merged {
		sh.MergeCoincidentSurfaces()
	}
	return sh
}

// pinchBand is a band standing on z = 0 from x = 6 to 20 and y = -3 to
// 3, whose top is the arc through (6, ±3, 21) and (6, 0, 20), extruded
// along x: at x = 6 its top touches the cone at the one point (6, 0,
// 20), where the arc is tangent to the curve the plane x = 6 cuts from
// the cone. Its end face outside the cone is two lobes meeting there.
// flip draws the arc the other way.
func pinchBand(flip bool, tol srf.Tolerance) *srf.SShell {
	const r = 5.0 // the arc's radius, about (6, 0, 25)
	h := math.Asin(3 / r)
	p0, p2 := v(6, -3, 21), v(6, 3, 21)
	if flip {
		p0, p2 = p2, p0
	}
	arc := srf.SBezierFrom3(p0, v(6, 0, 25-r/math.Cos(h)), p2)
	arc.Weight[1] = math.Cos(h)
	sbl := srf.SBezierList{Curves: []srf.SBezier{
		srf.SBezierFrom2(v(6, -3, 21), v(6, -3, 0)),
		srf.SBezierFrom2(v(6, -3, 0), v(6, 3, 0)),
		srf.SBezierFrom2(v(6, 3, 0), v(6, 3, 21)),
		arc,
	}}
	var sets srf.SBezierLoopSetSet
	var poly srf.SPolygon
	var allClosed, allCoplanar bool
	var notClosed srf.SEdge
	var notCoplanar geom.Vector
	sets.FindOuterFacesFrom(&sbl, &poly, nil, srf.Tol(0.1, 20), &allClosed, &notClosed, &allCoplanar, &notCoplanar, nil)
	sh := &srf.SShell{Tol: tol}
	for i := range sets.Sets {
		sh.MakeFromExtrusionOf(&sets.Sets[i], v(0, 0, 0), v(14, 0, 0), core.RGBA{})
	}
	return sh
}

// TestPinchedLoopSplit unites, intersects and subtracts the cone and
// the band whose top touches it at one point at the band's end face
// (roadmap K3). The end face outside the cone is two lobes meeting at
// the point, which the trims run round as one loop through it twice:
// counted one face, the shell's Euler characteristic was 1 against its
// mesh's 2. The loop is split there into two faces, and every result is
// sound to the oracle, the Euler check included.
func TestPinchedLoopSplit(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("chord", 0.01, 0.002, 0.0008544921875),
			sweep.Bools("flip"),
			sweep.Bools("merged"),
		},
		MaxFailRate: 0,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		tol := srf.Tol(p.Float("chord"), 200)
		cone, band := pinchCone(p.Bool("merged"), tol), pinchBand(p.Bool("flip"), tol)
		for k, op := range []srf.SurfaceCombineAs{srf.SurfaceUnion, srf.SurfaceInter, srf.SurfaceDiff, srf.SurfaceDiff} {
			a, b := cone, band
			if k == 3 {
				a, b = band, cone
			}
			out := srf.SShell{Tol: tol}
			out.MakeFromBoolean(a, b, op)
			var m srf.SMesh
			out.TriangulateInto(&m)
			if r := check.Solid(&out, &m); !r.Sound() {
				return fmt.Errorf("%v (%d): naked %d, crossing %d, topology %v", op, k, r.NakedEdges, r.SelfIntersecting, r.Topology)
			}
		}
		// The union's end face at x = 6 is the two lobes, two faces.
		out := srf.SShell{Tol: tol}
		out.MakeFromBoolean(cone, band, srf.SurfaceUnion)
		tp, err := topo.Build(&out)
		if err != nil {
			return err
		}
		lobes := 0
		for f := range tp.Faces {
			if s := tp.Surface(f); s != nil && onPlaneX6(s) {
				lobes++
			}
		}
		if lobes != 2 {
			return fmt.Errorf("%d faces at x = 6, want the two lobes", lobes)
		}
		return nil
	}).Require(t)
}

// onPlaneX6 reports whether a surface is a plane patch in x = 6.
func onPlaneX6(s *srf.SSurface) bool {
	if s.DegM != 1 || s.DegN != 1 {
		return false
	}
	for i := range 2 {
		for j := range 2 {
			if math.Abs(s.Ctrl[i][j].X-6) > 1e-9 {
				return false
			}
		}
	}
	return true
}
