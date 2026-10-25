package srf

import (
	"errors"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/cdt"
)

// cdtTol meshes with the constrained Delaunay mesher at a chord
// tolerance of chord.
func cdtTol(chord float64) Tolerance { return Tol(chord, 20).WithMesher(MesherCDT) }

// TestCDTShapes meshes lathed and revolved solids, a cone and a sphere
// with patches folded to poles among them: each surface is meshed by
// the CDT mesher and not given back to the ear clipper, and the mesh is
// closed, doesn't cross itself and holds the closed form's volume
// within what the chord tolerance loses.
func TestCDTShapes(t *testing.T) {
	t.Parallel()
	tol := cdtTol(0.01)
	frustum := func(r0, r1, h float64) float64 { return math.Pi * h / 3 * (r0*r0 + r0*r1 + r1*r1) }
	for _, c := range []struct {
		name string
		sh   *SShell
		want float64
	}{
		{"pot", lathedProfile([]geom.Vector{{}, {X: 30}, {X: 20, Z: 40}, {Z: 40}}, tol), frustum(30, 20, 40)},
		{"spout", lathedProfile([]geom.Vector{{}, {X: 6}, {X: 3, Z: 20}, {Z: 20}}, tol), frustum(6, 3, 20)},
		{"cone", lathedCone(5, 10, tol), frustum(5, 0, 10)},
		{"rod", lathedTube(0, 1, -5, 20, tol), math.Pi * 25},
		{"torus", torusY(geom.Vector{}, 8, 2, tol), 2 * math.Pi * math.Pi * 8 * 4},
		{"sphere", patchUpSphere(10, geom.Vector{}, tol), 4.0 / 3 * math.Pi * 1000},
	} {
		for i := range c.sh.Surface {
			s := &c.sh.Surface[i]
			var el SEdgeList
			s.MakeEdgesInto(c.sh, &el, true, nil)
			var poly SPolygon
			el.AssemblePolygon(&poly, nil, true)
			var m SMesh
			if err := poly.cdtTriangulateInto(&m, s); err != nil || len(m.Triangles) == 0 {
				t.Errorf("%s: surface %d: %v, %d triangles", c.name, i, err, len(m.Triangles))
			}
		}
		if !meshIsSound(c.sh) {
			t.Errorf("%s: the mesh is not watertight or crosses itself", c.name)
		}
		// The chords lose up to about the area times the tolerance: on
		// the rod, of radius 1, 0.6%, mostly its ends' polygons, which
		// the trims make and the ear clipper meshes as well (0.65%).
		if got := shellVolume(c.sh); math.Abs(got-c.want) > 0.01*c.want {
			t.Errorf("%s: volume %g, want %g", c.name, got, c.want)
		}
	}
}

// TestCDTBoreToLathedTipSweep is TestBoreToLathedTipSweep meshed by the
// CDT mesher: none fails. The ear clipper fans long chords across the
// thin lens of the bore's wall where the bore breaks out through the
// spout's side, and 20 of 80 failed.
func TestCDTBoreToLathedTipSweep(t *testing.T) {
	t.Parallel()
	tol := cdtTol(0.01)
	grid := boreTipGrid()
	grid.Parallel = true
	grid.Run(boreToLathedTipAt(tol)).Require(t)
}

// TestCDTHandleOnPotSweep is TestHandleOnPotSweep meshed by the CDT
// mesher: none fails. On the spike 6 of 144 failed, against the ear
// clipper's 9, all tilted, with a seam of the ring inside the pot by
// less than the chord tolerance (roadmap K3, mended since).
func TestCDTHandleOnPotSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("a sweep")
	}
	t.Parallel()
	tol := cdtTol(0.01)
	grid := handleGrid()
	grid.MaxFailRate = 0
	grid.SampleMaxFailRate = 2.0 / 24
	grid.Parallel = true
	grid.Run(handleOnPotAt(tol)).Require(t)
}

// TestCDTFlatWithHole meshes a square with a square hole on a plane:
// no point is added inside, and the triangles cover the square less
// the hole.
func TestCDTFlatWithHole(t *testing.T) {
	t.Parallel()
	s := FromPlane(geom.Vector{}, geom.Vector{X: 10}, geom.Vector{Y: 10})
	var p SPolygon
	for _, loop := range [][]geom.Vector{
		{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}},
		{{X: 0.4, Y: 0.4}, {X: 0.4, Y: 0.6}, {X: 0.6, Y: 0.6}, {X: 0.6, Y: 0.4}},
	} {
		var c SContour
		for _, q := range append(loop, loop[0]) {
			c.AddPoint(q)
		}
		p.Contours = append(p.Contours, c)
	}
	var m SMesh
	if err := p.cdtTriangulateInto(&m, &s); err != nil {
		t.Fatal(err)
	}
	area := 0.0
	for _, tr := range m.Triangles {
		for _, v := range []geom.Vector{tr.A, tr.B, tr.C} {
			if v.X != 0 && v.X != 1 && v.X != 0.4 && v.X != 0.6 {
				t.Fatalf("a point inside a plane: %v", v)
			}
		}
		// Clockwise in uv, as the ear clipper's are.
		area -= tr.Normal().Z / 2
	}
	if math.Abs(area-0.96) > 1e-12 {
		t.Errorf("area %g, want 0.96", area)
	}
}

// TestCDTOverlapFallsBack gives the ear clipper a boundary that runs
// back over itself, a spike of no width: the even-odd rule would drop
// the spike's edges, which the face beyond keeps.
func TestCDTOverlapFallsBack(t *testing.T) {
	t.Parallel()
	s := FromPlane(geom.Vector{}, geom.Vector{X: 10}, geom.Vector{Y: 10})
	var c SContour
	for _, q := range []geom.Vector{{}, {X: 1}, {X: 1, Y: 0.5}, {X: 1, Y: 0.2}, {X: 1, Y: 1}, {Y: 1}, {}} {
		c.AddPoint(q)
	}
	p := SPolygon{Contours: []SContour{c}}
	var m SMesh
	if err := p.cdtTriangulateInto(&m, &s); !errors.Is(err, errCDTMesh) || len(m.Triangles) != 0 {
		t.Errorf("err %v, %d triangles; want errCDTMesh and none", err, len(m.Triangles))
	}
}

// TestCDTPolarChart checks the chart about a cone's apex: the side
// folded to the apex is one point, and points elsewhere come back
// where they were.
func TestCDTPolarChart(t *testing.T) {
	t.Parallel()
	cone := lathedCone(5, 10, Tolerance{})
	for i := range cone.Surface {
		s := &cone.Surface[i]
		if s.flat() {
			continue
		}
		ch := newUVChart(s, []geom.Vector{{}, {X: 1, Y: 1}})
		if ch == nil || ch.pole < 0 {
			t.Fatalf("surface %d: no polar chart", i)
		}
		for _, q := range []geom.Vector{{X: 0.2, Y: 0.3}, {X: 0.9, Y: 0.1}, {X: 0.5, Y: 0.95}} {
			if back := ch.from(ch.to(q)); back.Minus(q).Magnitude() > 1e-12 {
				t.Errorf("surface %d: %v comes back as %v", i, q, back)
			}
		}
		pole := func(t float64) geom.Vector { return ch.unpolar(0, t) }
		if ch.to(pole(0)) != (cdt.Point{}) || ch.to(pole(0.7)) != (cdt.Point{}) {
			t.Errorf("surface %d: the folded side is not one point", i)
		}
		// The chart's lengths are the surface's, about.
		a, b := geom.Vector{X: 0.3, Y: 0.5}, geom.Vector{X: 0.31, Y: 0.5}
		pa, pb := ch.to(a), ch.to(b)
		if d, want := math.Hypot(pb.X-pa.X, pb.Y-pa.Y), s.PointAt(b.X, b.Y).Minus(s.PointAt(a.X, a.Y)).Magnitude(); math.Abs(d/want-1) > 0.05 {
			t.Errorf("surface %d: a length %g in the chart, %g on the surface", i, d, want)
		}
	}
}

func TestSegBoxCross(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		a, b cdt.Point
		want bool
	}{
		{cdt.Point{X: -1, Y: 0.5}, cdt.Point{X: 2, Y: 0.5}, true},
		{cdt.Point{X: 0.5, Y: 0.5}, cdt.Point{X: 0.6, Y: 0.6}, true},
		{cdt.Point{X: -1, Y: 2}, cdt.Point{X: 2, Y: 2}, false},
		{cdt.Point{X: -1, Y: 0}, cdt.Point{X: 0, Y: -1}, false},
		{cdt.Point{X: 1.5, Y: -1}, cdt.Point{X: 1.5, Y: 2}, false},
	} {
		if got := segBoxCross(c.a, c.b, 0, 0, 1, 1); got != c.want {
			t.Errorf("%v-%v: %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
