package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// leaningWedge is a plate whose top is z = 0, and on it a wedge 14
// along its foot and h high at its back, 10 wide, its width leaning by
// lean along the foot, sunk depth into the plate. Its sloping top's
// foot lies in the plate's top face, and so does the edge where the
// slope meets the front wall below; sunk by nothing, the wedge's base
// lies on the plate's top, facing it. Both are turned by q about the
// origin, so that no plane is one of the axes'.
func leaningWedge(h, lean, depth float64, q geom.Quaternion) (plate, wedge *SShell) {
	v := func(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }
	var p, w SShell
	p.MakePrism([]geom.Vector{v(-10, -8, -3), v(28, -8, -3), v(28, 18, -3), v(-10, 18, -3)}, v(0, 0, 0), v(0, 0, 3), core.RGBA{})
	profile := []geom.Vector{v(0, 0, -depth), v(14, 0, -depth), v(14, 0, 0), v(0, 0, h)}
	if depth == 0 {
		profile = []geom.Vector{v(0, 0, 0), v(14, 0, 0), v(0, 0, h)}
	}
	w.MakePrism(profile, v(0, 0, 0), v(lean, 10, 0), core.RGBA{})
	plate, wedge = &SShell{}, &SShell{}
	plate.MakeFromTransformationOf(&p, geom.Vector{}, q, 1)
	wedge.MakeFromTransformationOf(&w, geom.Vector{}, q, 1)
	return plate, wedge
}

// TestLeaningWedgeSweep unites, subtracts and intersects the leaning
// wedge and its plate (roadmap K3, the first case of coincident
// contact): an edge of one solid lying in a face of the other, and
// with the wedge not sunk, a face lying on a face. Every lean failed
// the union that leaned back along the foot: the slope, a sheared
// patch, clipped the line where it meets the plate as though its
// tangents were at right angles, and cut it short of the front wall's
// line along the same edge. The volumes are the prisms': the plate's,
// and the slope's triangle 7h, or the part sunk 14 depth, times 10.
func TestLeaningWedgeSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("h", 5, 10, 40),
			sweep.Floats("lean", -6, -4, -2, -0.5, 0, 2, 4),
			sweep.Floats("depth", 0, 1),
			sweep.Floats("turn", 0, 35),
			sweep.Strings("op", "union", "difference", "intersection"),
		},
		MaxFailRate: 0,
		Sample:      40,
	}
	grid.Run(func(p sweep.Point) error {
		h, depth := p.Float("h"), p.Float("depth")
		q := geom.QuaternionFromAxisAngle(geom.Vector{X: 0.3, Y: 1, Z: 0.2}.WithMagnitude(1), p.Float("turn")*math.Pi/180)
		plate, wedge := leaningWedge(h, p.Float("lean"), depth, q)
		plateV, aboveV, sunkV := 38.0*26*3, 70*h, 140*depth
		var op SurfaceCombineAs
		var want float64
		switch p.String("op") {
		case "union":
			op, want = SurfaceUnion, plateV+aboveV
		case "difference":
			op, want = SurfaceDiff, plateV-sunkV
		default:
			op, want = SurfaceInter, sunkV
		}
		var out SShell
		out.MakeFromBoolean(plate, wedge, op)
		if out.BooleanFailed || (want > 0 && !meshIsSound(&out)) {
			return errUnsound(out.BooleanFailed)
		}
		var m SMesh
		out.TriangulateInto(&m)
		if got := m.CalculateVolume(); math.Abs(got-want) > 1e-6*plateV {
			return fmt.Errorf("volume %g, want %g", got, want)
		}
		return nil
	}).Require(t)
}

// TestAlongTangents checks the parameter rates along a direction for
// tangents at right angles and sheared ones: along tv, u stays put.
func TestAlongTangents(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ d, tu, tv geom.Vector }{
		{geom.Vector{X: 1}, geom.Vector{X: 2}, geom.Vector{Y: 3}},
		{geom.Vector{X: -4, Y: 10}, geom.Vector{X: 14, Z: -5}, geom.Vector{X: -4, Y: 10}},
		{geom.Vector{X: 3, Y: 1}, geom.Vector{X: 1}, geom.Vector{X: 1, Y: 1}},
	} {
		ud, vd := alongTangents(c.d, c.tu, c.tv)
		if got := c.tu.ScaledBy(ud).Plus(c.tv.ScaledBy(vd)); !got.EqualsTol(c.d, 1e-12) {
			t.Errorf("d %v over %v, %v: %g, %g give %v", c.d, c.tu, c.tv, ud, vd, got)
		}
	}
}

// torusY is the ring of minor radius r about a circle of radius big
// about the axis through c along y, turned from a circle of four arcs:
// its seams run round the ring in the plane y = 0 (inside and outside)
// and along its top and bottom, and across it every quarter turn.
func torusY(c geom.Vector, big, r float64) *SShell {
	ctr := c.Plus(geom.Vector{X: big})
	at := func(a, s float64) geom.Vector {
		return ctr.Plus(geom.Vector{X: s * math.Cos(a), Y: s * math.Sin(a)})
	}
	sbls := &SBezierLoopSet{Normal: geom.Vector{Z: -1}, Point: ctr}
	const h = math.Pi / 4
	for k := range 4 {
		a0 := float64(k) * 2 * h
		b := SBezierFrom3(at(a0, r), at(a0+h, r/math.Cos(h)), at(a0+2*h, r))
		b.Weight[1] = math.Cos(h)
		sbls.Outer.Curves = append(sbls.Outer.Curves, b)
	}
	var sh SShell
	sh.MakeFromRevolutionOf(sbls, c, geom.Vector{Y: 1}, core.RGBA{}, noFaces{})
	return &sh
}

// TestHandleOnPotSweep unites a pot, lathed, with a ring for its handle
// (roadmap K3: the teapot's handle, a torus joined to a cone, was
// unsound), over where the ring is, its sizes, its height and its tilt.
// Its plane, y = 0, is the plane of the pot's seam, so seams of the two
// cross there, edge on edge; and the grid's round numbers put a ring's
// seam tangent to the pot (at z = 20 the pot's radius is 25, and a ring
// about x = -31 of 8 less 2 reaches it) or its corner on the pot's face.
// 15 of 144 failed: where two patches met only where their seams cross,
// the march from there followed one patch's surface past its edge
// (onBothPatches), and where the curve ran nearly straight along the ring's
// top, one step took the seeds of three curves and made one. 9 failed
// then: where a ring's patch corner lies on the pot's seam, the curve
// runs from it along the ring's seam within the chord tolerance, and the
// march on the patch beside the curve's own followed it too and used up
// the seed where the rest of that patch's curve starts
// (marchOnBothPatches); and the curve's chord lay on the seam, so that
// on its own patch it trimmed a sliver of nothing (hasReversed). The 5
// left: two where a curve lies a little inside a seam of the pot, deeper
// than the classification's probe, or ends 1e-5 from the seam's vertex,
// and three whose trims mesh crossing themselves (roadmap K4).
func TestHandleOnPotSweep(t *testing.T) {
	sweepChordTol(t)
	grid := handleGrid()
	grid.MaxFailRate = handleMaxFailRate
	grid.Run(handleOnPot).Require(t)
}

// handleGrid is TestHandleOnPotSweep's grid, its allowances unset.
func handleGrid() sweep.Grid {
	return sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("cx", -29, -31, -33, -35),
			sweep.Floats("big", 8, 10),
			sweep.Floats("r", 2, 2.5, 3),
			sweep.Floats("z", 15, 20, 26),
			sweep.Floats("tilt", 0, 20),
		},
		Sample: 24,
	}
}

// handleOnPot unites the pot and the ring of TestHandleOnPotSweep at p.
func handleOnPot(p sweep.Point) error {
	pot := lathedProfile([]geom.Vector{{}, {X: 30}, {X: 20, Z: 40}, {Z: 40}})
	ring := torusY(geom.Vector{}, p.Float("big"), p.Float("r"))
	var handle SShell
	q := geom.QuaternionFromAxisAngle(geom.Vector{X: 1}, p.Float("tilt")*math.Pi/180)
	handle.MakeFromTransformationOf(ring, geom.Vector{X: p.Float("cx"), Z: p.Float("z")}, q, 1)
	var out SShell
	out.MakeFromBoolean(pot, &handle, SurfaceUnion)
	if out.BooleanFailed || !meshIsSound(&out) {
		return errUnsound(out.BooleanFailed)
	}
	return nil
}

// TestHandleOnPotSeam unites the pot with two rings of
// TestHandleOnPotSweep whose patch corner lies on the pot's seam: in one
// the ring's seam passes over the pot's within 4e-5 of where the curve
// crosses both (fuseVertices), in the other the ring dips into the pot's
// patch beside the seam by less than the chord tolerance (hasSame).
func TestHandleOnPotSeam(t *testing.T) {
	sweepChordTol(t)
	for _, c := range []struct{ cx, big, r, z float64 }{{-35, 10, 2.5, 15}, {-31, 8, 2.5, 15}} {
		pot := lathedProfile([]geom.Vector{{}, {X: 30}, {X: 20, Z: 40}, {Z: 40}})
		var handle SShell
		q := geom.QuaternionFromAxisAngle(geom.Vector{X: 1}, 20*math.Pi/180)
		handle.MakeFromTransformationOf(torusY(geom.Vector{}, c.big, c.r), geom.Vector{X: c.cx, Z: c.z}, q, 1)
		var out SShell
		out.MakeFromBoolean(pot, &handle, SurfaceUnion)
		if out.BooleanFailed || !meshIsSound(&out) {
			t.Errorf("%+v: %v", c, errUnsound(out.BooleanFailed))
		}
	}
}

// handleMaxFailRate is the fraction of TestHandleOnPotSweep's grid
// allowed to fail: none since roadmap K4's mending, which meshes again
// by the constrained Delaunay mesher the faces whose ear-clipped trims
// crossed themselves (the last 3 of 144).
const handleMaxFailRate = 0

// TestTorusVolume checks the ring the handle sweep uses: closed, facing
// out, of volume 2 pi^2 big r^2.
func TestTorusVolume(t *testing.T) {
	t.Parallel()
	ring := torusY(geom.Vector{X: -31, Z: 20}, 10, 2)
	var m SMesh
	ring.TriangulateInto(&m)
	want := 2 * math.Pi * math.Pi * 10 * 4
	if got := m.CalculateVolume(); !meshIsSound(ring) || math.Abs(got-want) > 0.05*want {
		t.Errorf("sound %t, volume %g, want %g", meshIsSound(ring), got, want)
	}
}

// mitredRuns are two round bars of radius 1 meeting at a mitre: the
// first comes along x to the plane x = 10 + k y, the second leaves it
// along the first's direction mirrored in the plane. Both are extruded
// from the same ellipse in that plane, so they share their outlines there.
func mitredRuns(k float64) (a, b *SShell) {
	at := func(t, s float64) geom.Vector {
		y, z := s*math.Cos(t), s*math.Sin(t)
		return geom.Vector{X: 10 + k*y, Y: y, Z: z}
	}
	n := geom.Vector{X: 1, Y: -k}.WithMagnitude(1)
	mk := func() *SBezierLoopSet {
		sbls := &SBezierLoopSet{Normal: n.ScaledBy(-1), Point: at(0, 1)}
		const h = math.Pi / 4
		for i := range 4 {
			t0 := float64(i) * 2 * h
			c := SBezierFrom3(at(t0, 1), at(t0+h, 1/math.Cos(h)), at(t0+2*h, 1))
			c.Weight[1] = math.Cos(h)
			sbls.Outer.Curves = append(sbls.Outer.Curves, c)
		}
		return sbls
	}
	in := geom.Vector{X: 1}
	back := in.ScaledBy(-1)
	out := back.Minus(n.ScaledBy(2 * back.Dot(n)))
	a, b = &SShell{}, &SShell{}
	a.MakeFromExtrusionOf(mk(), geom.Vector{}, in.ScaledBy(-8), core.RGBA{})
	b.MakeFromExtrusionOf(mk(), geom.Vector{}, out.ScaledBy(8), core.RGBA{})
	return a, b
}

// TestMitredBarsUnion unites two round bars meeting at a mitre, whose
// end caps lie on each other, facing, with the same curved outline
// (roadmap K3's third case of contact, coplanar opposite faces of equal
// outlines). Faces like these failed where a sweep's runs were cut by a
// half space and unioned; the cut left the cap a partial face, and caps
// made whole from the same ellipse unite.
func TestMitredBarsUnion(t *testing.T) {
	sweepChordTol(t)
	for _, k := range []float64{0, 0.2, 0.5, 1} {
		a, b := mitredRuns(k)
		var ma, mb, m SMesh
		a.TriangulateInto(&ma)
		b.TriangulateInto(&mb)
		var out SShell
		out.MakeFromBoolean(a, b, SurfaceUnion)
		out.TriangulateInto(&m)
		want := ma.CalculateVolume() + mb.CalculateVolume()
		if out.BooleanFailed || !meshIsSound(&out) || math.Abs(m.CalculateVolume()-want) > 1e-6*want {
			t.Errorf("k %g: failed %t, sound %t, volume %g, want %g", k, out.BooleanFailed, meshIsSound(&out), m.CalculateVolume(), want)
		}
	}
}
