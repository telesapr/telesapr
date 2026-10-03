package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/sweep"
)

// sphereLoopSet is the half disc of radius r on the z axis, in the
// y=0 plane, wound as profileLoopSet winds its rectangle: two rational
// quarter arcs and the diameter along the axis.
func sphereLoopSet(r float64) *SBezierLoopSet {
	w := math.Sqrt(0.5)
	lo, mid, hi := geom.Vector{Z: -r}, geom.Vector{X: r}, geom.Vector{Z: r}
	a := SBezierFrom3(lo, geom.Vector{X: r, Z: -r}, mid)
	a.Weight[1] = w
	b := SBezierFrom3(mid, geom.Vector{X: r, Z: r}, hi)
	b.Weight[1] = w
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: 1}, Point: lo}
	sbls.Outer.Curves = []SBezier{a, b, SBezierFrom2(hi, lo)}
	return sbls
}

// patchUpSphere is a lathed sphere of radius r about c, turned so that
// the middle of its first curved patch faces +Z: a cap about the top
// then lies inside that one patch.
func patchUpSphere(r float64, c geom.Vector) *SShell {
	var sh SShell
	sh.MakeFromRevolutionOf(sphereLoopSet(r), geom.Vector{}, geom.Vector{Z: 1}, core.RgbaColor{}, noFaces{})
	var d geom.Vector
	for i := range sh.Surface {
		if s := &sh.Surface[i]; s.DegM > 1 || s.DegN > 1 {
			d = s.PointAt(0.5, 0.5).WithMagnitude(1)
			break
		}
	}
	// Z goes to u x v = d under FromBasis(u, v); its inverse takes d to Z.
	u := geom.Vector{Z: 1}.Cross(d).WithMagnitude(1)
	if u.Magnitude() < 0.5 {
		u = geom.Vector{X: 1}
	}
	q := geom.QuaternionFromBasis(u, d.Cross(u)).Inverse()
	var out SShell
	out.MakeFromTransformationOf(&sh, c, q, 1)
	return &out
}

// capVolume is the volume of a spherical cap of height k on a sphere of
// radius r.
func capVolume(r, k float64) float64 { return math.Pi * k * k * (3*r - k) / 3 }

// meshIsSound reports whether the shell's mesh is watertight and does
// not cross itself.
func meshIsSound(sh *SShell) bool {
	var m SMesh
	sh.TriangulateInto(&m)
	if m.IsEmpty() {
		return false
	}
	var naked SEdgeList
	var inters, leaks bool
	SKdNodeFrom(&m).MakeCertainEdgesInto(&naked, EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
	return !inters && !leaks && len(naked.Edges) == 0
}

// TestSphereThroughFace unites a box with a sphere that bulges through
// its top face in a small cap. The circle where they meet crosses no
// trim edge of either surface, so only the search for closed loops
// inside the patches finds it.
func TestSphereThroughFace(t *testing.T) {
	old := ChordTolMm()
	SetChordTolMm(0.005)
	t.Cleanup(func() { SetChordTolMm(old) })
	const r, top = 10.0, 9.3
	box := extrudedBox(geom.Vector{X: -15, Y: -15, Z: -15}, geom.Vector{X: 15, Y: 15, Z: top})
	sphere := patchUpSphere(r, geom.Vector{})
	boxV := 30 * 30 * (top + 15)
	cap := capVolume(r, r-top)
	for _, c := range []struct {
		name string
		op   SurfaceCombineAs
		want float64
	}{
		{"union", SurfaceUnion, boxV + cap},
		{"difference", SurfaceDiff, boxV - (4*math.Pi*r*r*r/3 - cap)},
		{"intersection", SurfaceInter, 4*math.Pi*r*r*r/3 - cap},
	} {
		var out SShell
		out.MakeFromBoolean(box, sphere, c.op)
		if out.BooleanFailed {
			t.Errorf("%s: boolean failed", c.name)
			continue
		}
		if !meshIsSound(&out) {
			t.Errorf("%s: mesh not watertight", c.name)
		}
		// The mesh's chords lose a little of the sphere.
		if got := shellVolume(&out); math.Abs(got-c.want) > 1e-3*c.want {
			t.Errorf("%s: volume %.4f, want %.4f", c.name, got, c.want)
		}
	}
}

// sphereFaceCase unites, subtracts and intersects the box below z = top
// with a sphere of radius r at c, and checks each result: the Boolean
// succeeds, the mesh is watertight, and the volumes add up as sets do,
// union + intersection = box + sphere and difference + intersection =
// box, to a small fraction of the sphere's volume.
func sphereFaceCase(r, top float64, c geom.Vector) error {
	box := func() *SShell {
		return extrudedBox(geom.Vector{X: -20, Y: -20, Z: -20}, geom.Vector{X: 20, Y: 20, Z: top})
	}
	vols := map[SurfaceCombineAs]float64{}
	for _, op := range []SurfaceCombineAs{SurfaceUnion, SurfaceDiff, SurfaceInter} {
		var out SShell
		out.MakeFromBoolean(box(), patchUpSphere(r, c), op)
		if out.BooleanFailed {
			return fmt.Errorf("%v: boolean failed", op)
		}
		if !meshIsSound(&out) {
			return fmt.Errorf("%v: mesh not watertight", op)
		}
		vols[op] = shellVolume(&out)
	}
	boxV, sphereV := shellVolume(box()), shellVolume(patchUpSphere(r, c))
	tol := 1e-2 * sphereV
	if d := vols[SurfaceUnion] + vols[SurfaceInter] - boxV - sphereV; math.Abs(d) > tol {
		return fmt.Errorf("union + intersection off by %.4g", d)
	}
	if d := vols[SurfaceDiff] + vols[SurfaceInter] - boxV; math.Abs(d) > tol {
		return fmt.Errorf("difference + intersection off by %.4g", d)
	}
	return nil
}

// TestSphereThroughFaceSweep runs sphereFaceCase over the sphere's
// radius, how far it bulges through the face, and where it sits under
// the face: small caps lie inside one patch of the sphere, larger ones
// cross its seams.
func TestSphereThroughFaceSweep(t *testing.T) {
	sweepChordTol(t)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("r", 3, 7, 10),
			sweep.Floats("bulge", 0.02, 0.05, 0.1, 0.2, 0.4),
			sweep.Floats("dx", 0, 0.37, 1.9),
			sweep.Floats("dy", 0, -0.61, 2.3),
		},
		MaxFailRate:       sphereFaceMaxFailRate,
		Sample:            12,
		SampleMaxFailRate: 1.0 / 12,
	}
	grid.Run(func(p sweep.Point) error {
		r := p.Float("r")
		c := geom.Vector{X: p.Float("dx"), Y: p.Float("dy")}
		return sphereFaceCase(r, r*(1-p.Float("bulge")), c)
	}).Require(t)
}

// sphereFaceMaxFailRate is the fraction of TestSphereThroughFaceSweep's
// grid allowed to fail: what the whole grid gave on 2026-10-02 and
// again on 2026-10-03, 10 of 135, against 99 before the closed loops were searched for. Nine are
// the intersection at the largest bulge, whose cap crosses the sphere's
// seams and which failed before as well; one is a cap so small that it
// triangulates flat against the face. The sample of 12 had one.
const sphereFaceMaxFailRate = 10.0 / 135

// crossedCylinder is a solid cylinder of radius r, along x through the
// point (0, dy, 0) turned by angle degrees about y, long enough to
// pass through the solid cylinder of radius 5 and height 20 on the z
// axis.
func crossedCylinder(r, dy, angle float64) *SShell {
	cyl := lathedTube(0, r, -30, 30)
	// Lay it along x, then turn it about y.
	q := geom.QuaternionFromAxisAngle(geom.Vector{Y: 1}, (90+angle)*math.Pi/180)
	var out SShell
	out.MakeFromTransformationOf(cyl, geom.Vector{Y: dy, Z: 10}, q, 1)
	return &out
}

// cylinderCase combines the upright cylinder with a crossed one and
// checks the results as sphereFaceCase does.
func cylinderCase(r, dy, angle float64) error {
	upright := func() *SShell { return lathedTube(0, 5, 0, 20) }
	vols := map[SurfaceCombineAs]float64{}
	for _, op := range []SurfaceCombineAs{SurfaceUnion, SurfaceDiff, SurfaceInter} {
		var out SShell
		out.MakeFromBoolean(upright(), crossedCylinder(r, dy, angle), op)
		if out.BooleanFailed {
			return fmt.Errorf("%v: boolean failed", op)
		}
		if !meshIsSound(&out) {
			return fmt.Errorf("%v: mesh not watertight", op)
		}
		vols[op] = shellVolume(&out)
	}
	a, b := shellVolume(upright()), shellVolume(crossedCylinder(r, dy, angle))
	tol := 1e-2 * math.Min(a, b)
	if d := vols[SurfaceUnion] + vols[SurfaceInter] - a - b; math.Abs(d) > tol {
		return fmt.Errorf("union + intersection off by %.4g", d)
	}
	if d := vols[SurfaceDiff] + vols[SurfaceInter] - a; math.Abs(d) > tol {
		return fmt.Errorf("difference + intersection off by %.4g", d)
	}
	return nil
}

// TestCylinderThroughCylinderSweep crosses cylinders over the second's
// radius, its offset from the first's axis and its angle to the first.
func TestCylinderThroughCylinderSweep(t *testing.T) {
	sweepChordTol(t)
	grid := sweep.Grid{
		Axes: []sweep.Axis{
			sweep.Floats("r", 1, 2.5, 4),
			sweep.Floats("dy", 0, 0.7, 2.9),
			sweep.Floats("angle", 0, 15, 30, 45, 60, 75),
		},
		MaxFailRate:       cylinderMaxFailRate,
		Sample:            10,
		SampleMaxFailRate: 2.0 / 10,
	}
	grid.Run(func(p sweep.Point) error {
		return cylinderCase(p.Float("r"), p.Float("dy"), p.Float("angle"))
	}).Require(t)
}

// cylinderMaxFailRate is the fraction of TestCylinderThroughCylinderSweep's
// grid allowed to fail: what the whole grid gave on 2026-10-03, 11 of
// 54, ten unions at 45 degrees and steeper and one difference that
// leaks. On 2026-10-02 it was 19, the same points before and after the
// kernel fixes of roadmap step 5; which commit mended eight was not
// looked for. Their curves are the ones the boundary-seeded marching
// follows (roadmap 13.2). The sample of 10 has two.
const cylinderMaxFailRate = 11.0 / 54

// sweepChordTol sets the chord tolerance the sweeps run at: fine enough
// that the volumes of the meshes add up as the solids' do.
func sweepChordTol(t *testing.T) {
	old := ChordTolMm()
	SetChordTolMm(0.01)
	t.Cleanup(func() { SetChordTolMm(old) })
}
