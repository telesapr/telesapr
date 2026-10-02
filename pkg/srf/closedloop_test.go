package srf

import (
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
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
