package srf

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/spacesolve/spacesolve/pkg/geom"
)

func TestBernsteinHighDegree(t *testing.T) {
	for deg := 1; deg <= MaxDegree; deg++ {
		for _, u := range []float64{0, 0.13, 0.5, 0.77, 1} {
			sum, dsum := 0.0, 0.0
			for k := 0; k <= deg; k++ {
				sum += bernstein(k, deg, u)
				dsum += bernsteinDerivative(k, deg, u)
				// Against the closed form, and the derivative against
				// a difference.
				want := binomial[deg][k] * math.Pow(u, float64(k)) * math.Pow(1-u, float64(deg-k))
				if math.Abs(bernstein(k, deg, u)-want) > 1e-12 {
					t.Errorf("B(%d,%d,%v) = %v, want %v", k, deg, u, bernstein(k, deg, u), want)
				}
				const h = 1e-6
				if u > h && u < 1-h {
					fd := (bernstein(k, deg, u+h) - bernstein(k, deg, u-h)) / (2 * h)
					if math.Abs(bernsteinDerivative(k, deg, u)-fd) > 1e-6 {
						t.Errorf("B'(%d,%d,%v) = %v, want %v", k, deg, u, bernsteinDerivative(k, deg, u), fd)
					}
				}
			}
			if math.Abs(sum-1) > 1e-12 || math.Abs(dsum) > 1e-9 {
				t.Errorf("degree %d at %v: sum %v, derivative sum %v", deg, u, sum, dsum)
			}
		}
	}
}

// randomBezier is a rational curve of the degree with weights in
// [0.5, 2].
func randomBezier(r *rand.Rand, deg int) SBezier {
	b := SBezier{Degree: deg}
	for i := 0; i <= deg; i++ {
		b.Ctrl[i] = geom.Vector{X: r.Float64() * 10, Y: r.Float64() * 10, Z: r.Float64() * 10}
		b.Weight[i] = 0.5 + 1.5*r.Float64()
	}
	return b
}

func TestCurveElevationAndSplit(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for deg := 1; deg < MaxDegree; deg++ {
		b := randomBezier(r, deg)
		up, err := b.ElevatedTo(MaxDegree)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := up.Elevated(); err == nil {
			t.Errorf("degree %d raised past MaxDegree", up.Degree)
		}
		for _, u := range []float64{0, 0.2, 0.5, 0.9, 1} {
			if d := b.PointAt(u).Minus(up.PointAt(u)).Magnitude(); d > 1e-9 {
				t.Errorf("degree %d raised to %d: %v apart at %v", deg, up.Degree, d, u)
			}
			// The tangent's direction stays; its length goes with the
			// parameter, which elevation keeps too.
			if d := b.TangentAt(u).Minus(up.TangentAt(u)).Magnitude(); d > 1e-7 {
				t.Errorf("degree %d raised: tangents %v apart at %v", deg, d, u)
			}
		}
		// Split at a parameter: the halves run over the curve.
		var bef, aft SBezier
		up.SplitAt(0.3, &bef, &aft)
		for _, s := range []float64{0, 0.25, 0.6, 1} {
			if d := bef.PointAt(s).Minus(up.PointAt(0.3 * s)).Magnitude(); d > 1e-9 {
				t.Errorf("degree %d: first half off by %v at %v", up.Degree, d, s)
			}
			if d := aft.PointAt(s).Minus(up.PointAt(0.3 + 0.7*s)).Magnitude(); d > 1e-9 {
				t.Errorf("degree %d: second half off by %v at %v", up.Degree, d, s)
			}
		}
	}
}

func TestPatchElevationSplitAndClosest(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	// A torus patch, rational quadratic both ways, raised to MaxDegree.
	s := FromRevolutionOf(&SBezier{Degree: 2,
		Ctrl:   [MaxDegree + 1]geom.Vector{{X: 3}, {X: 3, Z: 1}, {X: 2, Z: 1}},
		Weight: [MaxDegree + 1]float64{1, math.Sqrt2 / 2, 1}},
		geom.Vector{}, geom.Vector{Z: 1}, 0, math.Pi/2, 0, 0)
	up, err := s.ElevatedTo(MaxDegree, MaxDegree)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := up.Elevated(true); err == nil {
		t.Error("a patch was raised past MaxDegree")
	}
	for k := 0; k < 20; k++ {
		u, v := r.Float64(), r.Float64()
		if d := s.PointAt(u, v).Minus(up.PointAt(u, v)).Magnitude(); d > 1e-9 {
			t.Errorf("raised torus %v apart at (%v, %v)", d, u, v)
		}
		if d := s.NormalAt(u, v).WithMagnitude(1).Minus(up.NormalAt(u, v).WithMagnitude(1)).Magnitude(); d > 1e-9 {
			t.Errorf("raised torus's normal %v off at (%v, %v)", d, u, v)
		}
		// Closest point finds the parameters again.
		var gu, gv float64
		up.ClosestPointToUV(up.PointAt(u, v), &gu, &gv, true)
		if math.Abs(gu-u) > 1e-6 || math.Abs(gv-v) > 1e-6 {
			t.Errorf("closest point at (%v, %v) for (%v, %v)", gu, gv, u, v)
		}
	}
	for _, byU := range []bool{true, false} {
		var a, b SSurface
		up.SplitInHalf(byU, &a, &b)
		for k := 0; k < 10; k++ {
			x, y := r.Float64(), r.Float64()
			pa, wa := a.PointAt(x, y), up.PointAt(x/2, y)
			pb, wb := b.PointAt(x, y), up.PointAt(0.5+x/2, y)
			if !byU {
				pa, wa = a.PointAt(y, x), up.PointAt(y, x/2)
				pb, wb = b.PointAt(y, x), up.PointAt(y, 0.5+x/2)
			}
			if pa.Minus(wa).Magnitude() > 1e-9 || pb.Minus(wb).Magnitude() > 1e-9 {
				t.Errorf("split by u=%v at (%v, %v): %v %v", byU, x, y, pa.Minus(wa).Magnitude(), pb.Minus(wb).Magnitude())
			}
		}
	}
}

// elevatedShell is sh with every patch raised to degree m by n and
// every exact trim curve raised by up: the same solid, none of whose
// surfaces the exact special cases (planes, extrusions) recognise.
func elevatedShell(t *testing.T, sh *SShell, m, n, up int) *SShell {
	t.Helper()
	out := &SShell{}
	out.MakeFromCopyOf(sh)
	for i := range out.Surface {
		s, err := out.Surface[i].ElevatedTo(m, n)
		if err != nil {
			t.Fatal(err)
		}
		out.Surface[i] = s
	}
	for i := range out.Curve {
		c := &out.Curve[i]
		if c.IsExact {
			e, err := c.Exact.ElevatedTo(min(MaxDegree, c.Exact.Degree+up))
			if err != nil {
				t.Fatal(err)
			}
			c.Exact = e
		}
	}
	return out
}

// TestElevatedSolids builds solids from patches of degree 4 and 5 and
// checks their meshes and Booleans against the same solids of the
// patches they were raised from.
func TestElevatedSolids(t *testing.T) {
	fineChordTol(t)
	type solid struct {
		name string
		sh   func() *SShell
		vol  float64
	}
	solids := []solid{
		{"box", func() *SShell { return extrudedBox(geom.Vector{}, geom.Vector{X: 2, Y: 3, Z: 1}) }, 6},
		{"tube", func() *SShell { return lathedTube(1, 2, 0, 1) }, 3 * math.Pi},
		{"cylinder", func() *SShell { return lathedTube(0, 2, -0.5, 1.5) }, 8 * math.Pi},
	}
	for _, deg := range [][2]int{{4, 4}, {MaxDegree, MaxDegree}, {MaxDegree, 2}} {
		for _, s := range solids {
			name := fmt.Sprintf("%s raised to %dx%d", s.name, deg[0], deg[1])
			sh := elevatedShell(t, s.sh(), deg[0], deg[1], 2)
			if !meshIsSound(sh) {
				t.Errorf("%s: the mesh is not sound", name)
			}
			checkVolume(t, name, shellVolume(sh), s.vol)
		}
		// A Boolean between raised solids: a tube through a box, so the
		// intersection runs over raised planes and raised cylinders.
		name := fmt.Sprintf("box less tube raised to %dx%d", deg[0], deg[1])
		box := elevatedShell(t, extrudedBox(geom.Vector{X: -3, Y: -3, Z: 0}, geom.Vector{X: 3, Y: 3, Z: 1}), deg[0], deg[1], 2)
		tube := elevatedShell(t, lathedTube(1, 2, -1, 2), deg[0], deg[1], 2)
		var diff SShell
		diff.MakeFromBoolean(box, tube, SurfaceDiff)
		if diff.BooleanFailed {
			t.Errorf("%s: the Boolean failed", name)
		}
		if !meshIsSound(&diff) {
			t.Errorf("%s: the mesh is not sound", name)
		}
		checkVolume(t, name, shellVolume(&diff), 36-3*math.Pi)
	}
}

// BenchmarkElevatedBoolean is a box less a tube with every patch of
// degree 5, which only the general intersection and mesher handle.
func BenchmarkElevatedBoolean(b *testing.B) {
	t := &testing.T{}
	box := elevatedShell(t, extrudedBox(geom.Vector{X: -3, Y: -3, Z: 0}, geom.Vector{X: 3, Y: 3, Z: 1}), MaxDegree, MaxDegree, 2)
	tube := elevatedShell(t, lathedTube(1, 2, -1, 2), MaxDegree, MaxDegree, 2)
	for b.Loop() {
		var diff SShell
		diff.MakeFromBoolean(box, tube, SurfaceDiff)
		var m SMesh
		diff.TriangulateInto(&m)
	}
}
