package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// TestFlushCutterThenBoolean cuts a pocket into a block's bottom with a
// cutter extruded from the bottom, flush with it, and then cuts or adds
// a box away from the pocket, and cuts another (roadmap K3: a cutter
// flush with a face). The cutter's cap on the bottom is dropped, and the
// edge its side shared with the cap stays a curve of its own beside the
// bottom's intersection curve along the same outline; the next Boolean,
// finding no surface for the cap, thinned that curve's chords against
// the side alone, and at chord tolerances of 0.005 and below the side's
// mesh and the bottom's parted round the pocket's mouth.
func TestFlushCutterThenBoolean(t *testing.T) {
	t.Parallel()
	v := func(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("chord", 0.02, 0.005, 0.002, 0.001),
			sweep.Strings("cutter", "round", "square"),
			sweep.Strings("then", "difference", "union"),
		},
		MaxFailRate: 0,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		tol := Tol(p.Float("chord"), 1000)
		block := extrudedBox(v(-20, -20, 0), v(20, 20, 20), tol)
		curves, pocket := circleCurves(v(0, 0, 0), v(1, 0, 0), v(0, 1, 0), 5), 25*math.Pi*10
		if p.String("cutter") == "square" {
			curves, pocket = polylineCurves(v(-4, -4, 0), v(4, -4, 0), v(4, 4, 0), v(-4, 4, 0)), 640.0
		}
		cutter := extrudedCurves(curves, v(0, 0, 0), v(0, 0, 10), tol)
		cut := SShell{Tol: tol}
		cut.MakeFromBoolean(block, cutter, SurfaceDiff)
		box := extrudedBox(v(10, 10, 15), v(30, 30, 30), tol)
		op, want := SurfaceDiff, 32000-pocket-10*10*5
		if p.String("then") == "union" {
			op, want = SurfaceUnion, 32000-pocket+20*20*15-10*10*5
		}
		next := SShell{Tol: tol}
		next.MakeFromBoolean(&cut, box, op)
		if next.BooleanFailed || !meshIsSound(&next) {
			return errUnsound(next.BooleanFailed)
		}
		// And a third, at the opposite corner.
		out := SShell{Tol: tol}
		out.MakeFromBoolean(&next, extrudedBox(v(-30, -30, 15), v(-10, -10, 30), tol), SurfaceDiff)
		want -= 10 * 10 * 5
		if out.BooleanFailed || !meshIsSound(&out) {
			return fmt.Errorf("third: %w", errUnsound(out.BooleanFailed))
		}
		// The round pocket's mesh is short of the circle's area by its
		// chords' sag.
		if got := shellVolume(&out); math.Abs(got-want) > 0.01*pocket {
			return fmt.Errorf("volume %g, want %g", got, want)
		}
		return nil
	}).Require(t)
}

// lathedFrustum is the cone of radius 10 at z = 0 and 4 at z = 30, as a
// sketch's lathe makes it, its profile turned about -z and its flat ends
// merged into one plane each; or, quarters, turned about z and its flat
// ends left as four patches each, seams between them along x and y.
func lathedFrustum(quarters bool, tol Tolerance) *SShell {
	if quarters {
		return lathedProfile([]geom.Vector{{}, {X: 10}, {X: 4, Z: 30}, {Z: 30}}, tol)
	}
	p := []geom.Vector{{}, {Z: 30}, {X: 4, Z: 30}, {X: 10}}
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: -1}, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, SBezierFrom2(p[i], p[(i+1)%len(p)]))
	}
	sh := &SShell{Tol: tol}
	sh.MakeFromRevolutionOf(sbls, geom.Vector{}, geom.Vector{Z: -1}, core.RGBA{}, noFaces{})
	sh.MergeCoincidentSurfaces()
	return sh
}

// endBand is a band standing on z = 0 from x = 6 to 20, 2y wide, whose
// top is the arc through (6, ±y, top) and (6, 0, top+sag), extruded
// along x; drawn from -y to y, or, flip, the other way.
func endBand(y, top, sag float64, flip bool, tol Tolerance) *SShell {
	v := func(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }
	c := (y*y - sag*sag) / (2 * sag)
	ctr, r := v(6, 0, top-c), math.Abs(c+sag)
	h := math.Asin(y / r)
	mid := ctr.Plus(geom.Vector{Z: math.Copysign(r/math.Cos(h), sag)})
	p0, p2 := v(6, -y, top), v(6, y, top)
	if flip {
		p0, p2 = p2, p0
	}
	arc := SBezierFrom3(p0, mid, p2)
	arc.Weight[1] = math.Cos(h)
	sbl := polylineCurves(v(6, -y, top), v(6, -y, 0), v(6, y, 0), v(6, y, top))
	sbl.Curves = append(sbl.Curves[:3], arc)
	return extrudedCurves(sbl, geom.Vector{}, v(14, 0, 0), tol)
}

// TestBandEndOnCone unites a cone with a band whose end face cuts into
// the cone's side and whose top, an arc across the band, dips to the
// cone just past the end face or meets it at the end face's arc
// (roadmap K3, from the fillet run-offs onto a curved face). With the
// top dipping in by 0.1 to 0.5 the Boolean failed: the curve the end face
// cuts from the cone was marched from the cone's seam, above the top, and
// stopped where the top's arc crosses it, a seed inside both patches
// whose curve goes on; the seed left at the band's side then had none to
// end at, so both faces' trims stayed open (passesThrough). And with the
// cone's flat end four patches, the band's bottom edge across it, lying
// on it, was split at the seam between the quarters on the end's patches
// and not on the band's (coplanarTrimCrossings under either face). Each
// Boolean is sound, and the volumes add up: the cone less the band and
// their intersection are the cone, and so on.
//
// With the top meeting the cone at one point (top 21, sag -1), where the
// end face's arc is tangent to the curve the end face cuts from the
// cone, the end face outside the cone is two lobes touching there, which
// its trims run round as one loop through the point twice; the mesh
// closes, and pkg/topo splits the loop into the two faces
// (TestPinchedLoopSplit).
func TestBandEndOnCone(t *testing.T) {
	t.Parallel()
	tol := Tol(0.0008544921875, 200)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("top", 20.5, 20.9, 21, 21.05, 21.5),
			sweep.Floats("sag", -1, 1),
			sweep.Bools("flip"),
			sweep.Bools("quarters"),
		},
		MaxFailRate: 0,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		cone := lathedFrustum(p.Bool("quarters"), tol)
		band := endBand(3, p.Float("top"), p.Float("sag"), p.Bool("flip"), tol)
		var vol [4]float64
		for k, op := range []SurfaceCombineAs{SurfaceUnion, SurfaceInter, SurfaceDiff, SurfaceDiff} {
			a, b := cone, band
			if k == 3 {
				a, b = band, cone
			}
			out := SShell{Tol: tol}
			out.MakeFromBoolean(a, b, op)
			if out.BooleanFailed || !meshIsSound(&out) {
				return fmt.Errorf("%v: %w", op, errUnsound(out.BooleanFailed))
			}
			vol[k] = shellVolume(&out)
		}
		// The cone's side is meshed by other chords where the band cuts
		// it, which moves its volume by up to a few cubic millimetres; a
		// lost piece of either solid is tens.
		cv, bv := shellVolume(cone), shellVolume(band)
		for _, c := range []struct {
			name      string
			got, want float64
		}{
			{"union", vol[0], vol[2] + vol[1] + vol[3]},
			{"cone", vol[2] + vol[1], cv},
			{"band", vol[3] + vol[1], bv},
		} {
			if math.Abs(c.got-c.want) > 1e-3*(cv+bv) {
				return fmt.Errorf("%s %g, want %g", c.name, c.got, c.want)
			}
		}
		return nil
	}).Require(t)
}

// TestBandInCone intersects the cone and the band of TestBandEndOnCone at
// a finer chord, where the cone's side within the band strays from it as
// the ear clipper meshes it: it fans a sliver from the cone's base to the
// point where the end face's curve crosses its seam, through the solid.
// The sliver's own edges each end on a face it meets and cross nothing,
// but the flat end face's fan to that point passes through it: 30 of the
// end face's edges crossed the mesh, with the band's top above the cone
// or not. The mender tests a straying surface against the other faces'
// edges too, and meshes it again (mendCrossings).
func TestBandInCone(t *testing.T) {
	t.Parallel()
	tol := Tol(0.0005, 200)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("top", 21, 21.001),
			sweep.Floats("sag", -1, 1),
			sweep.Bools("quarters"),
		},
		MaxFailRate: 0,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		cone := lathedFrustum(p.Bool("quarters"), tol)
		band := endBand(3, p.Float("top"), p.Float("sag"), false, tol)
		out := SShell{Tol: tol}
		out.MakeFromBoolean(cone, band, SurfaceInter)
		if out.BooleanFailed || !meshIsSound(&out) {
			return errUnsound(out.BooleanFailed)
		}
		return nil
	}).Require(t)
}

// TestCutAcrossCurvedSeam cuts a box out of a block with a round post as
// high as it standing half out past its side, the box's side crossing
// the arc where the post's top meets the block's in one plane (roadmap
// "a union opened at a top face two parts share"). The arc, a trim of
// both tops, is split where the box's side meets it on its exact curve;
// the line where that side crosses the tops was split where it crossed
// the arc's chord, short of that by the chord's sag, and neither top's
// trim closed.
func TestCutAcrossCurvedSeam(t *testing.T) {
	t.Parallel()
	v := func(x, y, z float64) geom.Vector { return geom.Vector{X: x, Y: y, Z: z} }
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("chord", 0.02, 0.005, 0.001),
			sweep.Floats("side", 18, 19, 21, 22),
		},
		MaxFailRate: 0,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		tol := Tol(p.Float("chord"), 20)
		block := extrudedBox(v(-20, -20, 0), v(20, 20, 10), tol)
		post := extrudedCurves(circleCurves(v(20, 0, 2), v(1, 0, 0), v(0, 1, 0), 3), v(0, 0, 0), v(0, 0, 8), tol)
		joined := SShell{Tol: tol}
		joined.MakeFromBoolean(block, post, SurfaceUnion)
		x := p.Float("side")
		out := SShell{Tol: tol}
		out.MakeFromBoolean(&joined, extrudedBox(v(-19, -10, 1), v(x, 10, 11), tol), SurfaceDiff)
		if out.BooleanFailed || !meshIsSound(&out) {
			return errUnsound(out.BooleanFailed)
		}
		return nil
	}).Require(t)
}
