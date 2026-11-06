package srf

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// nearCase is a pair of solids that touch, stand off or are sunk in by
// eps (roadmap K3, doc/dev-notes/k3-fuse-spike.md): a and b, the volumes
// of their union, of a less b and of their intersection, how close a
// volume must come, and the contact's area. A result that takes an
// offset within the chord tolerance for none, as snapToPlanes makes it
// within a tenth of it, is off by the area times the offset. meshed is
// what b's mesh loses of b's exact volume, as a share of it: a curved b's
// volumes are reckoned from that mesh, and a piece of b meshed apart from
// it, as an intersection is, can lose that share of itself again with a
// mesher that does not mesh it the same way (the CDT puts points inside
// the side).
type nearCase struct {
	a, b               *SShell
	union, diff, inter float64
	relTol             float64
	area               float64
	meshed             float64
}

// nearTurn turns a shell off every axis by deg degrees.
func nearTurn(sh *SShell, deg float64) *SShell {
	if deg == 0 {
		return sh
	}
	q := geom.QuaternionFromAxisAngle(geom.Vector{X: 0.3, Y: 1, Z: 0.2}.WithMagnitude(1), deg*math.Pi/180)
	out := &SShell{Tol: sh.Tol}
	out.MakeFromTransformationOf(sh, geom.Vector{}, q, 1)
	return out
}

// nearFixtures are the pairs by name, at offset eps (negative: sunk in by
// -eps), turned by turn degrees.
var nearFixtures = map[string]func(eps, turn float64, tol Tolerance) nearCase{
	// A block on a block, its bottom eps above the other's top, part of
	// it over the edge.
	"stack": func(eps, turn float64, tol Tolerance) nearCase {
		a := extrudedBox(geom.Vector{}, geom.Vector{X: 10, Y: 10, Z: 5}, tol)
		b := extrudedBox(geom.Vector{X: 3, Y: 2, Z: 5 + eps}, geom.Vector{X: 8, Y: 12, Z: 9}, tol)
		ov := 40 * math.Max(0, -eps)
		return nearCase{nearTurn(a, turn), nearTurn(b, turn), 500 + 50*(4-eps) - ov, 500 - ov, ov, 1e-9, 50, 0}
	},
	// A block beside a block, its side eps off the other's side.
	"side": func(eps, turn float64, tol Tolerance) nearCase {
		a := extrudedBox(geom.Vector{}, geom.Vector{X: 10, Y: 10, Z: 5}, tol)
		b := extrudedBox(geom.Vector{X: 10 + eps, Y: 2, Z: 1}, geom.Vector{X: 15, Y: 8, Z: 4}, tol)
		ov := 18 * math.Max(0, -eps)
		return nearCase{nearTurn(a, turn), nearTurn(b, turn), 500 + 18*(5-eps) - ov, 500 - ov, ov, 1e-9, 18, 0}
	},
	// A block inside a block, its top eps above the other's top.
	"flushtop": func(eps, turn float64, tol Tolerance) nearCase {
		a := extrudedBox(geom.Vector{}, geom.Vector{X: 10, Y: 10, Z: 5}, tol)
		b := extrudedBox(geom.Vector{X: 3, Y: 3, Z: 2}, geom.Vector{X: 7, Y: 7, Z: 5 + eps}, tol)
		bv := 16 * (3 + eps)
		in := 16 * (3 + math.Min(0, eps))
		return nearCase{nearTurn(a, turn), nearTurn(b, turn), 500 + bv - in, 500 - in, in, 1e-9, 16, 0}
	},
	// A lathed cylinder standing on a block, its base eps above the top.
	"cylinder": func(eps, turn float64, tol Tolerance) nearCase {
		a := extrudedBox(geom.Vector{X: -5, Y: -5, Z: -2}, geom.Vector{X: 5, Y: 5, Z: 0}, tol)
		v := func(x, z float64) geom.Vector { return geom.Vector{X: x, Z: z} }
		b := lathedProfile([]geom.Vector{v(0, eps), v(3, eps), v(3, 6), v(0, 6)}, tol)
		var m SMesh
		b.TriangulateInto(&m)
		bv := m.CalculateVolume()
		ov := bv * math.Max(0, -eps) / (6 - eps)
		exact := 9 * math.Pi * (6 - eps)
		return nearCase{nearTurn(a, turn), nearTurn(b, turn), 200 + bv - ov, 200 - ov, ov, 1e-6, 9 * math.Pi,
			math.Abs(exact-bv) / exact}
	},
}

// errNearVolume is a near-contact Boolean whose volume is off.
var errNearVolume = errors.New("volume off")

// nearOutcome makes the Boolean op of c and checks it: made, closed where
// it has a volume, and of the volume want, within the offset's slack.
func nearOutcome(c nearCase, op SurfaceCombineAs, want, eps float64) error {
	slack := 1e-6
	if math.Abs(eps) <= c.a.Tol.ChordTolMm() {
		slack += c.area * math.Abs(eps)
	}
	var out SShell
	out.MakeFromBoolean(c.a, c.b, op)
	if out.BooleanFailed {
		return errUnsound(true)
	}
	var m SMesh
	out.TriangulateInto(&m)
	got := m.CalculateVolume()
	if want <= slack {
		// Touching, or sunk by a hair taken for none: nothing, or a
		// shell of no volume.
		if m.IsEmpty() || math.Abs(got) < slack {
			return nil
		}
		return fmt.Errorf("%w: %g, want none", errNearVolume, got)
	}
	if !meshSound(&m) {
		return errUnsound(false)
	}
	if math.Abs(got-want) > c.relTol*math.Max(want, 1)+2*c.meshed*want+slack {
		return fmt.Errorf("%w: %g, want %g", errNearVolume, got, want)
	}
	return nil
}

// TestNearContactSweep unites, subtracts and intersects solids that
// touch, stand a hair off or are sunk a hair in (roadmap K3, the spike of
// 2026-11-04), from 3e-7 to 3e-2 mm, turned off the axes, at two chord
// tolerances. Before snapToPlanes, 150 of the 816 failed (114 refused, 33
// not closed, 3 off in volume with a closed mesh), all at offsets under
// the chord tolerance; a turned cylinder 4 mm from a block failed too,
// as an intersection curve off both patches was kept.
func TestNearContactSweep(t *testing.T) {
	t.Parallel()
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("fixture", "stack", "side", "flushtop", "cylinder"),
			sweep.Floats("eps", -3e-2, -1e-2, -3e-3, -1e-3, -1e-5, -1e-6, -3e-7, 0, 3e-7, 1e-6, 3e-6, 1e-5, 1e-4, 1e-3, 3e-3, 1e-2, 3e-2),
			sweep.Floats("chord", 0.1, 0.01),
			sweep.Floats("turn", 0, 35),
			sweep.Strings("op", "union", "difference", "intersection"),
		},
		MaxFailRate: 0,
		Sample:      120,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		eps := p.Float("eps")
		c := nearFixtures[p.String("fixture")](eps, p.Float("turn"), Tol(p.Float("chord"), 20))
		switch p.String("op") {
		case "union":
			return nearOutcome(c, SurfaceUnion, c.union, eps)
		case "difference":
			return nearOutcome(c, SurfaceDiff, c.diff, eps)
		}
		return nearOutcome(c, SurfaceInter, c.inter, eps)
	}).Require(t)
}

// TestDisjointTurnedCylinder unites a lathed cylinder with a block 4 mm
// below it, both turned: an intersection curve of the block's top with
// the cylinder's side, on their surfaces' extensions past both patches,
// was kept, its closest points on the patches within their parameters,
// and the side's trim did not close (addExactIntersectionCurve).
func TestDisjointTurnedCylinder(t *testing.T) {
	t.Parallel()
	tol := Tol(0.1, 20)
	v := func(x, z float64) geom.Vector { return geom.Vector{X: x, Z: z} }
	for _, turn := range []float64{0, 10, 35, 90} {
		b := nearTurn(lathedProfile([]geom.Vector{v(0, 1), v(3, 1), v(3, 6), v(0, 6)}, tol), turn)
		a := nearTurn(extrudedBox(geom.Vector{X: -5, Y: -5, Z: -5}, geom.Vector{X: 5, Y: 5, Z: -3}, tol), turn)
		var out SShell
		out.MakeFromBoolean(a, b, SurfaceUnion)
		if out.BooleanFailed || !meshIsSound(&out) {
			t.Errorf("turned %g°: failed %v, sound %v", turn, out.BooleanFailed, meshIsSound(&out))
			continue
		}
		// The cylinder is meshed again in the union, not point for point
		// as on its own (the constrained Delaunay mesher's points inside a
		// face fall by rounding): each within the chord tolerance of its
		// curved side and ends, of area 2π·3·5 + 2·π·3².
		if got, want := shellVolume(&out), 200+shellVolume(b); math.Abs(got-want) > tol.ChordTolMm()*(2*math.Pi*3*5+2*math.Pi*9) {
			t.Errorf("turned %g°: volume %g, want %g", turn, got, want)
		}
	}
}

// TestPinInBoreSweep puts a pin of radius 3 + eps in a bore of 3 drilled
// through a block, and a lathed rod in a lathed tube, coaxial (roadmap
// K3, the spike of 2026-11-04), and unites, subtracts and intersects
// them. The bore's side and the pin's are one surface where eps is 0:
// the edge-on-edge test took the curvature their seams' chords dip by
// for a side, and every Boolean failed or left its mesh open. Within a
// tenth of the chord tolerance the pin is made the bore's size
// (snapToCylinders). Offsets past that are left out: their sides' chords
// cross within the chord tolerance, which only a shared mesh would mend
// (K4).
func TestPinInBoreSweep(t *testing.T) {
	t.Parallel()
	x, y := geom.Vector{X: 1}, geom.Vector{Y: 1}
	v := func(x, z float64) geom.Vector { return geom.Vector{X: x, Z: z} }
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Strings("fixture", "drilled", "lathed"),
			sweep.Floats("eps", -1e-3, -1e-5, -1e-6, -1e-7, 0, 1e-7, 1e-6, 1e-5, 1e-3),
			sweep.Floats("chord", 0.1, 0.01),
			sweep.Floats("turn", 0, 35),
			sweep.Strings("op", "union", "difference", "intersection"),
		},
		MaxFailRate: 0,
		Sample:      40,
		Parallel:    true,
	}
	grid.Run(func(p sweep.Point) error {
		tol, eps := Tol(p.Float("chord"), 20), p.Float("eps")
		if math.Abs(eps) >= tol.ChordTolMm()/snapPlaneDiv {
			return nil // past the fuse: see above
		}
		var a, b *SShell
		if p.String("fixture") == "drilled" {
			blk := extrudedBox(geom.Vector{X: -6, Y: -6, Z: 0}, geom.Vector{X: 6, Y: 6, Z: 6}, tol)
			drill := extrudedCurves(circleCurves(geom.Vector{}, x, y, 3), geom.Vector{Z: -1}, geom.Vector{Z: 7}, tol)
			a = &SShell{Tol: tol}
			a.MakeFromBoolean(blk, drill, SurfaceDiff)
			b = extrudedCurves(circleCurves(geom.Vector{}, x, y, 3+eps), geom.Vector{Z: -1}, geom.Vector{Z: 7}, tol)
		} else {
			a = lathedProfile([]geom.Vector{v(3, 0), v(5, 0), v(5, 6), v(3, 6)}, tol)
			b = lathedProfile([]geom.Vector{v(0, -1), v(3+eps, -1), v(3+eps, 7), v(0, 7)}, tol)
		}
		a, b = nearTurn(a, p.Float("turn")), nearTurn(b, p.Float("turn"))
		// The pin is as large as the bore, so the two only touch.
		pin := shellVolume(b)
		aV := shellVolume(a)
		var op SurfaceCombineAs
		var want float64
		switch p.String("op") {
		case "union":
			op, want = SurfaceUnion, aV+pin
		case "difference":
			op, want = SurfaceDiff, aV
		default:
			op, want = SurfaceInter, 0
		}
		var out SShell
		out.MakeFromBoolean(a, b, op)
		if out.BooleanFailed {
			return errUnsound(true)
		}
		var m SMesh
		out.TriangulateInto(&m)
		got := m.CalculateVolume()
		if want == 0 {
			if math.Abs(got) > 1e-3 {
				return fmt.Errorf("%w: %g, want none", errNearVolume, got)
			}
			return nil
		}
		if !meshSound(&m) {
			return errUnsound(false)
		}
		if math.Abs(got-want) > 2e-3*want {
			return fmt.Errorf("%w: %g, want %g", errNearVolume, got, want)
		}
		return nil
	}).Require(t)
}
