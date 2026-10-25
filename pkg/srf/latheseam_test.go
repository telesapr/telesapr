package srf

import (
	"errors"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// lathedProfile is the solid lathed a full turn about the z axis from
// the closed polygon p in the xz plane, at tol.
func lathedProfile(p []geom.Vector, tol Tolerance) *SShell {
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: 1}, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, SBezierFrom2(p[i], p[(i+1)%len(p)]))
	}
	sh := SShell{Tol: tol}
	sh.MakeFromRevolutionOf(sbls, geom.Vector{}, geom.Vector{Z: 1}, core.RGBA{}, noFaces{})
	return &sh
}

// arcXZ is the circular arc in the xz plane about c of radius r, from
// angle a0 to a1 (radians, less than a quarter turn apart).
func arcXZ(c geom.Vector, r, a0, a1 float64) SBezier {
	at := func(a, s float64) geom.Vector {
		return c.Plus(geom.Vector{X: s * math.Cos(a), Z: s * math.Sin(a)})
	}
	h := (a1 - a0) / 2
	b := SBezierFrom3(at(a0, r), at(a0+h, r/math.Cos(h)), at(a1, r))
	b.Weight[1] = math.Cos(h)
	return b
}

// arcBand is the slab between the arcs of radius r0 and r1 about c in
// the xz plane, from angle a0 to a1, extruded w either side of y = 0:
// a teapot's handle where it meets the pot. It is built at tol.
func arcBand(c geom.Vector, r0, r1, a0, a1, w float64, tol Tolerance) *SShell {
	in, out := arcXZ(c, r0, a0, a1), arcXZ(c, r1, a0, a1)
	out.Reverse()
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: -1}, Point: in.Start()}
	sbls.Outer.Curves = []SBezier{in, SBezierFrom2(in.Finish(), out.Start()), out, SBezierFrom2(out.Finish(), in.Start())}
	sh := SShell{Tol: tol}
	sh.MakeFromExtrusionOf(sbls, geom.Vector{Y: -w}, geom.Vector{Y: w}, core.RGBA{})
	return &sh
}

// TestBandOverLatheSeamSweep unites a hollow lathed cone, a spout or a
// pot, with a band whose arcs meet its walls at shallow angles over
// the lathe's seam, the half plane y = 0, x > 0 (roadmap 24.3, from
// drawing a teapot through the MCP server).
func TestBandOverLatheSeamSweep(t *testing.T) {
	t.Parallel()
	tol := sweepTol()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("cz", 5, 12, 20),
			sweep.Floats("cx", -1, 1, 3, 5),
			sweep.Floats("r", 6, 7, 8),
			sweep.Floats("angle", -30, 0, 30),
		},
		MaxFailRate: latheSeamMaxFailRate,
		Sample:      20,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		pot := lathedProfile([]geom.Vector{{X: 5}, {X: 8}, {X: 4, Z: 30}, {X: 1, Z: 30}}, tol)
		a := p.Float("angle") * math.Pi / 180
		band := arcBand(geom.Vector{X: p.Float("cx"), Z: p.Float("cz")}, p.Float("r"), p.Float("r")+1.5, a-0.6, a+0.6, 1.5, tol)
		out := SShell{Tol: tol}
		out.MakeFromBoolean(pot, band, SurfaceUnion)
		if out.BooleanFailed || !meshIsSound(&out) {
			return errUnsound(out.BooleanFailed)
		}
		return nil
	}).Require(t)
}

// latheSeamMaxFailRate is the fraction of TestBandOverLatheSeamSweep's
// grid allowed to fail: none since 24.3's rest. The 8 of 108 left after
// 24.1 and 24.2 (14 before them), all with the band low on the pot, had
// a marching seed half a millimetre past the band's side: the lathed
// wall's bottom circle met the band's arc, a cylinder, beyond its
// patch, and the hit, projected onto the patch's side, passed its trim
// (offPatchTol). The march from it took the curve's own seed, and the
// wall's trims did not close.
const latheSeamMaxFailRate = 0

// TestBoreToLathedTipSweep bores a lathed spout along its axis, a
// little off it or not, with the bore's end cap in the plane of the
// spout's tip, a disc of four flat patches meeting at the axis
// (roadmap 24.3). A cap on the axis passes; one off it is coincident
// contact across the patches' seams (roadmap K3), where the bore's
// circle crosses a seam that no surface crosses: every one of those
// failed until the circle was split at the seams (fuseCoplanarSeam).
func TestBoreToLathedTipSweep(t *testing.T) {
	t.Parallel()
	tol := sweepTol()
	grid := boreTipGrid()
	grid.MaxFailRate = boreTipMaxFailRate
	grid.Parallel = true
	grid.Run(boreToLathedTipAt(tol)).Require(t)
}

// boreTipGrid is TestBoreToLathedTipSweep's grid, its allowances unset.
func boreTipGrid() sweep.Grid {
	return sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("tilt", 0, 30, 48, 75),
			sweep.Floats("bore", 0.5, 1, 1.5, 2.5, 2.9),
			sweep.Floats("tip", 3, 1.2),
			sweep.Floats("off", 0, 0.3),
		},
		Sample: 16,
	}
}

// boreToLathedTipAt bores the spout of TestBoreToLathedTipSweep at a
// point, the solids built at tol.
func boreToLathedTipAt(tol Tolerance) func(p sweep.Point) error {
	return func(p sweep.Point) error {
		q := geom.QuaternionFromAxisAngle(geom.Vector{X: 0.3, Y: 1, Z: 0.1}.WithMagnitude(1), p.Float("tilt")*math.Pi/180)
		at := geom.Vector{X: 3, Y: 1, Z: 2}
		tip := p.Float("tip")
		spout, bore := SShell{Tol: tol}, SShell{Tol: tol}
		spout.MakeFromTransformationOf(lathedProfile([]geom.Vector{{}, {X: 6}, {X: tip, Z: 20}, {Z: 20}}, tol), at, q, 1)
		bore.MakeFromTransformationOf(lathedTube(0, math.Min(p.Float("bore"), tip*0.95), -5, 20, tol),
			at.Plus(q.Rotate(geom.Vector{X: p.Float("off")})), q, 1)
		out := SShell{Tol: tol}
		out.MakeFromBoolean(&spout, &bore, SurfaceDiff)
		if out.BooleanFailed || !meshIsSound(&out) {
			return errUnsound(out.BooleanFailed)
		}
		return nil
	}
}

// TestMendCrossings bores the spout of TestBoreToLathedTipSweep where
// the bore breaks out through its side: the ear clipper's meshes of the
// faces, each on its own, cross where the bore's wall is a thin lens,
// and the shell's mesh, mended, is sound (roadmap K4).
func TestMendCrossings(t *testing.T) {
	t.Parallel()
	tol := sweepTol()
	q := geom.QuaternionIdentity
	at := geom.Vector{X: 3, Y: 1, Z: 2}
	spout, bore := SShell{Tol: tol}, SShell{Tol: tol}
	spout.MakeFromTransformationOf(lathedProfile([]geom.Vector{{}, {X: 6}, {X: 1.2, Z: 20}, {Z: 20}}, tol), at, q, 1)
	bore.MakeFromTransformationOf(lathedTube(0, 1, -5, 20, tol), at.Plus(geom.Vector{X: 0.3}), q, 1)
	out := SShell{Tol: tol}
	out.MakeFromBoolean(&spout, &bore, SurfaceDiff)
	if out.BooleanFailed {
		t.Fatal("the Boolean failed")
	}
	var ear SMesh
	mended := 0
	for i := range out.Surface {
		var m SMesh
		if out.Surface[i].triangulate(&out, &m, false) {
			mended++
		}
		ear.Triangles = append(ear.Triangles, m.Triangles...)
	}
	var naked SEdgeList
	var inters, leaks bool
	SKdNodeFrom(&ear).MakeCertainEdgesInto(&naked, EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
	if !inters && !leaks || mended == 0 {
		t.Errorf("the ear clipper's mesh is sound (crossing %t, leaking %t, %d faces straying)", inters, leaks, mended)
	}
	if !meshIsSound(&out) {
		t.Error("the mended mesh is not sound")
	}
}

// boreTipMaxFailRate is the fraction of TestBoreToLathedTipSweep's
// grid allowed to fail: none since roadmap K4's mending. The 20 of 80
// that failed before, whose bore breaks out through the spout's side
// below the tip, had Booleans that closed; the ear clipper's mesh of the
// bore's wall, a thin lens between the tip and where the bore leaves the
// cone, fanned long chords from the crossing that crossed the cone. The
// lens is now meshed again by the constrained Delaunay mesher
// (mendCrossings).
const boreTipMaxFailRate = 0

// errUnsound says how a Boolean's result failed.
func errUnsound(failed bool) error {
	if failed {
		return errBooleanFailed
	}
	return errNotSound
}

var (
	errBooleanFailed = errors.New("the Boolean failed")
	errNotSound      = errors.New("the mesh is not watertight or crosses itself")
)
