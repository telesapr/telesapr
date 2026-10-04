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
		MaxFailRate: sphereFaceMaxFailRate,
		Sample:      12,
	}
	grid.Run(func(p sweep.Point) error {
		r := p.Float("r")
		c := geom.Vector{X: p.Float("dx"), Y: p.Float("dy")}
		return sphereFaceCase(r, r*(1-p.Float("bulge")), c)
	}).Require(t)
}

// sphereFaceMaxFailRate is the fraction of TestSphereThroughFaceSweep's
// grid allowed to fail: none since roadmap 24.1. The last, a cap so
// small (bulge 0.02) that its patch's grid kept clear of it, joined
// two points of the cut on either side of a patch's seam by an edge
// lying in the box's face (splitTrimDiagonals). It was 1 of 135 after
// roadmap 13.2, which mended nine intersections whose band of sphere
// laid a sliver flat on the plane's face (folded in triangulate.go);
// 10 on 2026-10-02 and 2026-10-03, and 99 before the closed loops were
// searched for.
const sphereFaceMaxFailRate = 0

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

// TestSphereCapNoFold intersects a sphere with the box below a plane
// that cuts it well above its middle: the band of sphere next to the
// flat cap must not lay a triangle of three rim points flat on the
// cap's face, which overlaps it (roadmap 13.2).
func TestSphereCapNoFold(t *testing.T) {
	sweepChordTol(t)
	r := 7.0
	box := extrudedBox(geom.Vector{X: -20, Y: -20, Z: -20}, geom.Vector{X: 20, Y: 20, Z: r * 0.6})
	var out SShell
	out.MakeFromBoolean(box, patchUpSphere(r, geom.Vector{}), SurfaceInter)
	if out.BooleanFailed || !meshIsSound(&out) {
		t.Fatalf("failed %v, sound %v", out.BooleanFailed, meshIsSound(&out))
	}
}

// TestSphereCutAlongAxis cuts a lathed sphere by a plane parallel to
// its axis, a little off it, so that the circle of the cut runs near
// both poles: the sphere's patches there must not join two points of
// that circle by an edge, which lies in the plane's face, inside its
// outline, and crosses its triangles (roadmap 24.1).
func TestSphereCutAlongAxis(t *testing.T) {
	sweepChordTol(t)
	const r = 7.0
	sphere := func() *SShell {
		var sh SShell
		sh.MakeFromRevolutionOf(sphereLoopSet(r), geom.Vector{}, geom.Vector{Z: 1}, core.RgbaColor{}, noFaces{})
		return &sh
	}
	for _, x := range []float64{0.146 * r, 0.3 * r, 0.6 * r} {
		box := extrudedBox(geom.Vector{X: x, Y: -20, Z: -20}, geom.Vector{X: 20, Y: 20, Z: 20})
		for _, op := range []SurfaceCombineAs{SurfaceDiff, SurfaceInter} {
			var out SShell
			out.MakeFromBoolean(sphere(), box, op)
			if out.BooleanFailed || !meshIsSound(&out) {
				t.Errorf("x = %g, %v: failed %v, sound %v", x, op, out.BooleanFailed, meshIsSound(&out))
			}
		}
	}
}

// TestCylinderAcrossEndSeam crosses the upright cylinder at 45 degrees
// so that the ellipse where the crossed one meets the upright's flat
// end runs across the seam between two quarters of that end. Each
// quarter must get its own part of the curve, split at the seam, or
// neither quarter's trim closes and the union fails (roadmap 13.2).
func TestCylinderAcrossEndSeam(t *testing.T) {
	sweepChordTol(t)
	if err := cylinderCase(4, 0.7, 60); err != nil {
		t.Fatal(err)
	}
	var out SShell
	out.MakeFromBoolean(lathedTube(0, 5, 0, 20), crossedCylinder(4, 0.7, 45), SurfaceUnion)
	if out.BooleanFailed || !meshIsSound(&out) {
		t.Fatalf("union: failed %v, sound %v", out.BooleanFailed, meshIsSound(&out))
	}
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
		MaxFailRate: cylinderMaxFailRate,
		Sample:      10,
	}
	grid.Run(func(p sweep.Point) error {
		return cylinderCase(p.Float("r"), p.Float("dy"), p.Float("angle"))
	}).Require(t)
}

// cylinderMaxFailRate is the fraction of TestCylinderThroughCylinderSweep's
// grid allowed to fail: none since roadmap 24.1. The last two, from
// 13.2 to 24.1, were differences (r=4 at dy=0, 30 degrees, and dy=0.7,
// 45 degrees) where the crossed cylinder's seam pierces the upright
// wall: the wall's strip there was ear-clipped into a sliver the full
// height of the wall with the pierce point a hair off its long edge,
// which turned over in xyz (SContour.grazed). It was 2 from 13.2 to
// 24.1, 11 on 2026-10-03 and 19 on 2026-10-02.
const cylinderMaxFailRate = 0

// sweepChordTol sets the chord tolerance the sweeps run at: fine enough
// that the volumes of the meshes add up as the solids' do.
func sweepChordTol(t *testing.T) {
	old := ChordTolMm()
	SetChordTolMm(0.01)
	t.Cleanup(func() { SetChordTolMm(old) })
}

// lathedCone is the solid cone of base radius r on z = 0 and apex at
// height h on the z axis, lathed from a triangle: its curved patches
// fold to a point at the apex.
func lathedCone(r, h float64) *SShell {
	p := []geom.Vector{{}, {X: r}, {Z: h}}
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: 1}, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, SBezierFrom2(p[i], p[(i+1)%3]))
	}
	var sh SShell
	sh.MakeFromRevolutionOf(sbls, geom.Vector{}, geom.Vector{Z: 1}, core.RgbaColor{}, noFaces{})
	return &sh
}

// coneSegment is the volume of the part of the cone of base radius r
// and height h beyond the plane x = d.
func coneSegment(r, h, d float64) float64 {
	const n = 20000
	v := 0.0
	for i := range n {
		z := (float64(i) + 0.5) * h / n
		if rz := r * (1 - z/h); rz > d {
			v += (rz*rz*math.Acos(d/rz) - d*math.Sqrt(rz*rz-d*d)) * h / n
		}
	}
	return v
}

// TestConeCutNearApex cuts a cone by planes parallel to its axis,
// near it: the cut runs up close to the apex, where the curved patches
// fold to a point. Each patch's outline in uv must run along the
// folded side, not across the patch on a diagonal to one corner of
// it, or the Boolean drops part of the cut; and the patch, which has
// no grid along its straight lines, must not be meshed by chords
// across the cone (roadmap 24.2).
func TestConeCutNearApex(t *testing.T) {
	sweepChordTol(t)
	const r, h = 6.0, 9.0
	cone := lathedCone(r, h)
	whole := math.Pi * r * r * h / 3
	for _, d := range []float64{0.4, 0.8772, 2} {
		box := extrudedBox(geom.Vector{X: d, Y: -20, Z: -20}, geom.Vector{X: 20, Y: 20, Z: 20})
		want := map[SurfaceCombineAs]float64{SurfaceDiff: whole - coneSegment(r, h, d), SurfaceInter: coneSegment(r, h, d)}
		for _, op := range []SurfaceCombineAs{SurfaceDiff, SurfaceInter} {
			var out SShell
			out.MakeFromBoolean(cone, box, op)
			if out.BooleanFailed || !meshIsSound(&out) {
				t.Errorf("d = %g, %v: failed %v, sound %v", d, op, out.BooleanFailed, meshIsSound(&out))
				continue
			}
			if got := shellVolume(&out); math.Abs(got-want[op]) > 5e-3*whole {
				t.Errorf("d = %g, %v: volume %.4f, want %.4f", d, op, got, want[op])
			}
		}
	}
}
