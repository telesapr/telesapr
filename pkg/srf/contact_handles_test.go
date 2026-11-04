package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// curveHandlesErr says which of sh's curves names a surface sh lacks,
// or one its points don't lie on: each curve is an edge between the
// two surfaces it names, and a later Boolean, a mesh's edges and the
// derived topology read it so.
func curveHandlesErr(sh *SShell) error {
	for i := range sh.Curve {
		c := &sh.Curve[i]
		for _, h := range [2]struct {
			name string
			v    uint32
		}{{"A", c.SurfA.V}, {"B", c.SurfB.V}} {
			if h.v == 0 {
				continue
			}
			sf := sh.FindSurface(c.SurfA)
			if h.name == "B" {
				sf = sh.FindSurface(c.SurfB)
			}
			if sf == nil {
				return fmt.Errorf("curve %d names surface %s %d, which the shell lacks", c.H.V, h.name, h.v)
			}
			for _, pt := range c.Points {
				var uv geom.Point2d
				sf.ClosestPointTo(pt.P, &uv, false)
				if d := sf.PointAtUv(uv).Minus(pt.P).Magnitude(); d > 1e-3 {
					return fmt.Errorf("curve %d's point %v lies %g off its surface %s %d", c.H.V, pt.P, d, h.name, h.v)
				}
			}
		}
	}
	return nil
}

// TestFlushCutterCurveHandles cuts a pocket with a cutter flush with a
// block's bottom, then cuts and adds boxes, and requires every curve of
// each result to name surfaces the result holds and the curve lies on
// (roadmap K3, a flush cutter's orphaned edge). The cutter's cap on the
// bottom is dropped; the edge its side shared with the cap kept the
// cap's handle, which named no surface of the result, and the next
// Boolean, finding no surface for it in its operand, left it as it was,
// so that in its own result it named whichever surface came to have
// that handle there.
func TestFlushCutterCurveHandles(t *testing.T) {
	t.Parallel()
	v := func(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("chord", 0.02, 0.002),
			sweep.Strings("cutter", "round", "square"),
			sweep.Strings("then", "difference", "union"),
		},
		MaxFailRate: 0,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		tol := Tol(p.Float("chord"), 1000)
		block := extrudedBox(v(-20, -20, 0), v(20, 20, 20), tol)
		curves := circleCurves(v(0, 0, 0), v(1, 0, 0), v(0, 1, 0), 5)
		if p.String("cutter") == "square" {
			curves = polylineCurves(v(-4, -4, 0), v(4, -4, 0), v(4, 4, 0), v(-4, 4, 0))
		}
		cut := SShell{Tol: tol}
		cut.MakeFromBoolean(block, extrudedCurves(curves, v(0, 0, 0), v(0, 0, 10), tol), SurfaceDiff)
		if err := curveHandlesErr(&cut); err != nil {
			return fmt.Errorf("the pocket: %w", err)
		}
		op := SurfaceDiff
		if p.String("then") == "union" {
			op = SurfaceUnion
		}
		next := SShell{Tol: tol}
		next.MakeFromBoolean(&cut, extrudedBox(v(10, 10, 15), v(30, 30, 30), tol), op)
		if next.BooleanFailed || !meshIsSound(&next) {
			return errUnsound(next.BooleanFailed)
		}
		if err := curveHandlesErr(&next); err != nil {
			return fmt.Errorf("the box: %w", err)
		}
		// A copy moved and joined to it, as a step and repeat makes it,
		// renumbers every surface.
		var moved SShell
		moved.MakeFromTransformationOf(&next, v(100, 0, 0), geom.QuaternionIdentity, 1)
		var both SShell
		both.MakeFromAssemblyOf(&next, &moved)
		if err := curveHandlesErr(&both); err != nil {
			return fmt.Errorf("the assembly: %w", err)
		}
		if math.IsNaN(shellVolume(&next)) {
			return fmt.Errorf("volume NaN")
		}
		return nil
	}).Require(t)
}
