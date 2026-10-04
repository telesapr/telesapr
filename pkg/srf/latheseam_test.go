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
// the closed polygon p in the xz plane.
func lathedProfile(p []geom.Vector) *SShell {
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: 1}, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, SBezierFrom2(p[i], p[(i+1)%len(p)]))
	}
	var sh SShell
	sh.MakeFromRevolutionOf(sbls, geom.Vector{}, geom.Vector{Z: 1}, core.RgbaColor{}, noFaces{})
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
// a teapot's handle where it meets the pot.
func arcBand(c geom.Vector, r0, r1, a0, a1, w float64) *SShell {
	in, out := arcXZ(c, r0, a0, a1), arcXZ(c, r1, a0, a1)
	out.Reverse()
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: -1}, Point: in.Start()}
	sbls.Outer.Curves = []SBezier{in, SBezierFrom2(in.Finish(), out.Start()), out, SBezierFrom2(out.Finish(), in.Start())}
	var sh SShell
	sh.MakeFromExtrusionOf(sbls, geom.Vector{Y: -w}, geom.Vector{Y: w}, core.RgbaColor{})
	return &sh
}

// TestBandOverLatheSeamSweep unites a hollow lathed cone, a spout or a
// pot, with a band whose arcs meet its walls at shallow angles over
// the lathe's seam, the half plane y = 0, x > 0 (roadmap 24.3, from
// drawing a teapot through the MCP server).
func TestBandOverLatheSeamSweep(t *testing.T) {
	sweepChordTol(t)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("cz", 5, 12, 20),
			sweep.Floats("cx", -1, 1, 3, 5),
			sweep.Floats("r", 6, 7, 8),
			sweep.Floats("angle", -30, 0, 30),
		},
		MaxFailRate:       latheSeamMaxFailRate,
		Sample:            20,
		SampleMaxFailRate: 3.0 / 20,
	}
	grid.Run(func(p sweep.Point) error {
		pot := lathedProfile([]geom.Vector{{X: 5}, {X: 8}, {X: 4, Z: 30}, {X: 1, Z: 30}})
		a := p.Float("angle") * math.Pi / 180
		band := arcBand(geom.Vector{X: p.Float("cx"), Z: p.Float("cz")}, p.Float("r"), p.Float("r")+1.5, a-0.6, a+0.6, 1.5)
		var out SShell
		out.MakeFromBoolean(pot, band, SurfaceUnion)
		if out.BooleanFailed || !meshIsSound(&out) {
			return errUnsound(out.BooleanFailed)
		}
		return nil
	}).Require(t)
}

// latheSeamMaxFailRate is the fraction of TestBandOverLatheSeamSweep's
// grid allowed to fail: 8 of 108 at the first measurement (roadmap
// 24.3, after 24.1 and 24.2; 14 before them), all with the band low on
// the pot, where it also crosses the pot's flat bottom and meets the
// inner wall at the seam. Not mended: the trims of the inner wall's
// patch beside the seam come out mixed with its neighbour's.
const latheSeamMaxFailRate = 8.0 / 108

// TestBoreToLathedTipSweep bores a lathed spout along its axis, a
// little off it or not, with the bore's end cap in the plane of the
// spout's tip, a disc of four flat patches meeting at the axis
// (roadmap 24.3). A cap on the axis passes; one off it is coincident
// contact across the patches' seams, which the kernel doesn't take
// (F5, under "Left out"): every one of those fails.
func TestBoreToLathedTipSweep(t *testing.T) {
	sweepChordTol(t)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("tilt", 0, 30, 48, 75),
			sweep.Floats("bore", 0.5, 1, 1.5, 2.5, 2.9),
			sweep.Floats("tip", 3, 1.2),
			sweep.Floats("off", 0, 0.3),
		},
		MaxFailRate:       boreTipMaxFailRate,
		Sample:            16,
		SampleMaxFailRate: 10.0 / 16,
	}
	grid.Run(func(p sweep.Point) error {
		q := geom.QuaternionFromAxisAngle(geom.Vector{X: 0.3, Y: 1, Z: 0.1}.WithMagnitude(1), p.Float("tilt")*math.Pi/180)
		at := geom.Vector{X: 3, Y: 1, Z: 2}
		tip := p.Float("tip")
		var spout, bore SShell
		spout.MakeFromTransformationOf(lathedProfile([]geom.Vector{{}, {X: 6}, {X: tip, Z: 20}, {Z: 20}}), at, q, 1)
		bore.MakeFromTransformationOf(lathedTube(0, math.Min(p.Float("bore"), tip*0.95), -5, 20),
			at.Plus(q.Rotate(geom.Vector{X: p.Float("off")})), q, 1)
		var out SShell
		out.MakeFromBoolean(&spout, &bore, SurfaceDiff)
		if out.BooleanFailed || !meshIsSound(&out) {
			return errUnsound(out.BooleanFailed)
		}
		return nil
	}).Require(t)
}

// boreTipMaxFailRate is the fraction of TestBoreToLathedTipSweep's
// grid allowed to fail: the 40 of 80 with the bore off the axis.
const boreTipMaxFailRate = 40.0 / 80

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
