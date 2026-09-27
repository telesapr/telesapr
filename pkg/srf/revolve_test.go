package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// profileLoopSet returns the rectangle r0 <= x <= r1, z0 <= z <= z1 in
// the y=0 plane, wound clockwise about its +Y normal as C++
// FindOuterFacesFrom leaves outer loops.
func profileLoopSet(r0, r1, z0, z1 float64) *SBezierLoopSet {
	p := []geom.Vector{{X: r0, Z: z0}, {X: r1, Z: z0}, {X: r1, Z: z1}, {X: r0, Z: z1}}
	sbls := &SBezierLoopSet{Normal: geom.Vector{Y: 1}, Point: p[0]}
	for i := range p {
		sbls.Outer.Curves = append(sbls.Outer.Curves, SBezierFrom2(p[i], p[(i+1)%4]))
	}
	return sbls
}

func lathedTube(r0, r1, z0, z1 float64) *SShell {
	sh := &SShell{}
	sh.MakeFromRevolutionOf(profileLoopSet(r0, r1, z0, z1), geom.Vector{}, geom.Vector{Z: 1}, core.RgbaColor{}, noFaces{})
	return sh
}

// fineChordTol sets a chord tolerance fine enough for the meshes to
// approach the analytic volumes, restoring the old one on cleanup.
func fineChordTol(t *testing.T) {
	old := ChordTolMm()
	SetChordTolMm(1e-4)
	t.Cleanup(func() { SetChordTolMm(old) })
}

// checkVolume compares a triangulated volume against the exact one,
// which the mesh approaches as the chord tolerance shrinks.
func checkVolume(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-3*want {
		t.Errorf("%s: volume=%v want=%v (rel err %.2g)", name, got, want, (got-want)/want)
	}
}

func TestRevolveVolumes(t *testing.T) {
	fineChordTol(t)
	// A full lathe of a rectangle away from the axis: a tube.
	checkVolume(t, "tube", shellVolume(lathedTube(1, 2, 0, 1)), 3*math.Pi)
	// A rectangle with an edge on the axis: that edge makes no
	// surface, and the ends become planes (a solid cylinder).
	cyl := lathedTube(0, 2, 0, 1)
	checkVolume(t, "cylinder", shellVolume(cyl), 4*math.Pi)
	planes := 0
	for _, s := range cyl.Surface {
		if s.TrimBuilt == false || len(s.Trim) == 0 {
			t.Errorf("surface %v has no trims", s.H)
		}
		if s.DegM == 1 && s.DegN == 1 {
			planes++
		}
	}
	// Three curves off the axis, four quarter-turn surfaces each; the
	// two end lines revolve into planes.
	if len(cyl.Surface) != 12 || planes != 8 {
		t.Errorf("cylinder: %d surfaces, %d planes; want 12, 8", len(cyl.Surface), planes)
	}

	// A partial revolution (MakeFromHelicalRevolutionOf with no
	// translation), in either direction: Pappus, area * rbar * angle.
	for _, a := range []float64{math.Pi / 3, -2.5} {
		var sh SShell
		sh.MakeFromHelicalRevolutionOf(profileLoopSet(1, 2, 0, 1), geom.Vector{}, geom.Vector{Z: 1},
			core.RgbaColor{}, noFaces{}, 0, a, 0, 0)
		checkVolume(t, "partial revolve", shellVolume(&sh), 1.5*math.Abs(a))
	}

	// A helix: the axial shear keeps Pappus' volume.
	var hx SShell
	hx.MakeFromHelicalRevolutionOf(profileLoopSet(1, 2, 0, 1), geom.Vector{}, geom.Vector{Z: 1},
		core.RgbaColor{}, noFaces{}, -math.Pi, math.Pi, -1, 1)
	checkVolume(t, "helix", shellVolume(&hx), 1.5*2*math.Pi)
}

// TestBooleanRevolveBoxVolume combines the tube 1 <= r <= 2,
// 0 <= z <= 1 with boxes, first one cutting it across the axis
// (a plane through the cylinders) and then one cutting it with a
// plane parallel to the axis (lines on the cylinders).
func TestBooleanRevolveBoxVolume(t *testing.T) {
	fineChordTol(t)
	tube := 3 * math.Pi
	// Area of the part x > a of the disc of radius r.
	seg := func(r, a float64) float64 { return r*r*math.Acos(a/r) - a*math.Sqrt(r*r-a*a) }

	cases := []struct {
		name     string
		lo, hi   geom.Vector
		interVol float64
	}{
		{"slab", geom.Vector{X: -3, Y: -3, Z: 0.25}, geom.Vector{X: 3, Y: 3, Z: 2}, 0.75 * tube},
		{"side", geom.Vector{X: 0.5, Y: -3, Z: -1}, geom.Vector{X: 3, Y: 3, Z: 2}, 1 * (seg(2, 0.5) - seg(1, 0.5))},
	}
	for _, c := range cases {
		box := (c.hi.X - c.lo.X) * (c.hi.Y - c.lo.Y) * (c.hi.Z - c.lo.Z)
		for _, op := range []struct {
			name string
			op   SurfaceCombineAs
			want float64
		}{
			{"union", SurfaceUnion, tube + box - c.interVol},
			{"difference", SurfaceDiff, tube - c.interVol},
			{"intersection", SurfaceInter, c.interVol},
		} {
			a := lathedTube(1, 2, 0, 1)
			b := extrudedBox(c.lo, c.hi)
			var r SShell
			r.MakeFromBoolean(a, b, op.op)
			if r.BooleanFailed {
				t.Errorf("%s %s: boolean failed", c.name, op.name)
				continue
			}
			checkVolume(t, c.name+" "+op.name, shellVolume(&r), op.want)
		}
	}
}

// TestRevolveParityCpp pins the mesh volumes at the default chord
// tolerance to those of the C++ SShell::MakeFromRevolutionOf,
// MakeFromHelicalRevolutionOf and MakeFromBoolean on the same inputs
// (a harness linked against libsolvespace-core, chordTolCalculated
// 0.1, maxSegments 20).
func TestRevolveParityCpp(t *testing.T) {
	check := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-8 {
			t.Errorf("%s: volume=%.12g, C++ %.12g", name, got, want)
		}
	}
	check("tube", shellVolume(lathedTube(1, 2, 0, 1)), 9.18326468473)
	check("cylinder", shellVolume(lathedTube(0, 2, 0, 1)), 12.244352913)
	for _, c := range []struct {
		angles, anglef, dists, distf, want float64
	}{
		{0, math.Pi / 3, 0, 0, 1.56630902883},
		{0, -2.5, 0, 0, 3.73472309685},
		{-math.Pi, math.Pi, -1, 1, 9.38597257969},
	} {
		var sh SShell
		sh.MakeFromHelicalRevolutionOf(profileLoopSet(1, 2, 0, 1), geom.Vector{}, geom.Vector{Z: 1},
			core.RgbaColor{}, noFaces{}, c.angles, c.anglef, c.dists, c.distf)
		check(fmt.Sprintf("helical %v..%v", c.anglef, c.distf), shellVolume(&sh), c.want)
	}
	boxes := []struct {
		name   string
		lo, hi geom.Vector
		want   [3]float64 // union, difference, intersection
	}{
		{"slab", geom.Vector{X: -3, Y: -3, Z: 0.25}, geom.Vector{X: 3, Y: 3, Z: 2},
			[3]float64{65.2958161712, 2.29581617118, 6.88744851355}},
		{"side", geom.Vector{X: 0.5, Y: -3, Z: -1}, geom.Vector{X: 3, Y: 3, Z: 2},
			[3]float64{50.6126559863, 5.61265598634, 3.5863097602}},
	}
	for _, c := range boxes {
		for k, op := range []SurfaceCombineAs{SurfaceUnion, SurfaceDiff, SurfaceInter} {
			var r SShell
			r.MakeFromBoolean(lathedTube(1, 2, 0, 1), extrudedBox(c.lo, c.hi), op)
			check(fmt.Sprintf("%s op %d", c.name, op), shellVolume(&r), c.want[k])
		}
	}
}
