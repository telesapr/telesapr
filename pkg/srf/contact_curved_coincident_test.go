package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// lensArea is the area of the intersection of a disc of radius a and
// one of radius b whose centres lie d apart.
func lensArea(a, b, d float64) float64 {
	switch {
	case d >= a+b:
		return 0
	case d <= math.Abs(a-b):
		r := math.Min(a, b)
		return math.Pi * r * r
	}
	x := (d*d + a*a - b*b) / (2 * d * a)
	y := (d*d + b*b - a*a) / (2 * d * b)
	return a*a*math.Acos(x) + b*b*math.Acos(y) - 0.5*math.Sqrt((-d+a+b)*(d+a-b)*(d-a+b)*(d+a+b))
}

// boreCutter is a ring 4 high about the z axis, 20 across its outside,
// whose bore of radius 8 at its foot widens by k per unit up (a cylinder
// where k is 0), lathed in quarters, with a rod of radius 3 at (12, 0)
// from z = -6 to 10 united with it where rod is set; and the bore's cone
// (or cylinder) filled, from z = -1 to 8, its curved face on the bore's
// over the ring's height, swept through breaks (in quarters where nil):
// the cutter a fillet past a bore cuts the widened solid back by
// (pkg/topo, curvedCutter). It gives both and the volume of their
// intersection, the rod's part inside the cutter.
func boreCutter(k float64, rod bool, breaks []float64, tol Tolerance) (a, b *SShell, inter float64) {
	v := func(x, z float64) geom.Vector { return geom.Vector{X: x, Z: z} }
	a = lathedProfile([]geom.Vector{v(8, 0), v(20, 0), v(20, 4), v(8+4*k, 4)}, tol)
	if rod {
		r := extrudedCurves(circleCurves(geom.Vector{X: 12}, geom.Vector{X: 1}, geom.Vector{Y: 1}, 3), geom.Vector{Z: -6}, geom.Vector{Z: 10}, tol)
		u := &SShell{Tol: tol}
		u.MakeFromBoolean(a, r, SurfaceUnion)
		a = u
		const n = 4000
		for i := range n {
			z := -1 + 9*(float64(i)+0.5)/n
			inter += lensArea(3, 8+k*z, 12) * 9 / n
		}
	}
	p := []geom.Vector{v(0, -1), v(8-k, -1), v(8+8*k, 8), v(0, 8)}
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: 1}, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, SBezierFrom2(p[i], p[(i+1)%len(p)]))
	}
	b = &SShell{Tol: tol}
	if breaks == nil {
		b.MakeFromRevolutionOf(sbls, geom.Vector{}, geom.Vector{Z: 1}, core.RGBA{}, noFaces{})
	} else {
		b.MakeFromRevolutionBetween(sbls, geom.Vector{}, geom.Vector{Z: 1}, core.RGBA{}, noFaces{}, breaks)
	}
	return a, b, inter
}

// cutterBreaks are the angles a cutter's curved face is swept through:
// in quarters as the bore is (nil), in tenths, or in pieces of 0.3 to
// 0.9 radians from 0.4, none on the bore's seams.
func cutterBreaks(name string) []float64 {
	var out []float64
	switch name {
	case "tenths":
		for i := range 11 {
			out = append(out, 2*math.Pi*float64(i)/10)
		}
	case "uneven":
		a := 0.4
		for _, d := range []float64{0.9, 0.3, 0.7, 0.9, 0.5, 0.8, 0.6, 0.9} {
			out = append(out, a)
			a += d
		}
		out = append(out, 0.4+2*math.Pi)
	}
	return out
}

// TestCoincidentRoundSweep unites, subtracts both ways and intersects a
// ring with a bore and the bore's cylinder or cone filled and carried on
// past both its ends, their curved faces on each other but cut into
// patches at other angles round the axis, with and without a rod
// beside the bore running into the cutter (roadmap K3, the fillet past
// a curved side carried on above its face, 2026-11-18). Cut at the same
// angles, every Boolean was right; cut at others, every one of the 64
// failed or lost the faces' loops: an edge of one face running across
// the other was classified by chords that dip off the surface by its
// curvature (sameRoundSurface), the edge where the ring's flat ends
// meet the bore was not split where the cutter's seams cross it
// (roundSeamsUnder), the curve where a flat end meets a patch of the
// cutter did not follow that edge past the end's patch (roundOf), and
// on a cone the end's outline was taken whole across the cutter's
// patch (piecesOnRoundPatch). The rod's part in the cutter is a thin
// lens, whose volume the chords get within 1e-2.
func TestCoincidentRoundSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("side", "cylinder", "cone"),
			sweep.Strings("breaks", "quarters", "tenths", "uneven"),
			sweep.Strings("rod", "none", "rod"),
			sweep.Floats("turn", 0, 35),
			sweep.Strings("op", "cutter-ring", "ring-cutter", "union", "intersection"),
		},
		MaxFailRate: 0,
		Sample:      24,
		Parallel:    true,
	}
	tol := Tol(0.003, 20)
	grid.Run(func(p sweep.Point) error {
		k := 0.0
		if p.String("side") == "cone" {
			k = math.Tan(10 * math.Pi / 180)
		}
		a, b, in := boreCutter(k, p.String("rod") == "rod", cutterBreaks(p.String("breaks")), tol)
		a, b = nearTurn(a, p.Float("turn")), nearTurn(b, p.Float("turn"))
		aV, bV := shellVolume(a), shellVolume(b)
		var out SShell
		var want float64
		switch p.String("op") {
		case "cutter-ring":
			out.MakeFromBoolean(b, a, SurfaceDiff)
			want = bV - in
		case "ring-cutter":
			out.MakeFromBoolean(a, b, SurfaceDiff)
			want = aV - in
		case "union":
			out.MakeFromBoolean(a, b, SurfaceUnion)
			want = aV + bV - in
		default:
			out.MakeFromBoolean(a, b, SurfaceInter)
			want = in
		}
		if out.BooleanFailed {
			return errUnsound(true)
		}
		got, sound := soundVolume(&out)
		if want > 0 && !sound {
			return errUnsound(false)
		}
		if math.Abs(got-want) > 2e-3*want+1e-2 {
			return fmt.Errorf("%w: %g, want %g", errNearVolume, got, want)
		}
		return nil
	}).Require(t)
}
