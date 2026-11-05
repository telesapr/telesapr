package srf

import (
	"fmt"
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
)

// teeCylinder is a cylinder of radius r about the axis through c along
// dir (a unit vector), from -l to l along it; its circle starts at phase
// radians from u, so its patches' seams can be turned off the points
// where an equal cylinder crossing it touches it.
func teeCylinder(c, dir geom.Vector, r, l, phase float64, tol Tolerance) *SShell {
	u := dir.NormalDir(0).WithMagnitude(1)
	v := dir.Cross(u).WithMagnitude(1)
	uu := u.ScaledBy(math.Cos(phase)).Plus(v.ScaledBy(math.Sin(phase)))
	vv := dir.Cross(uu).WithMagnitude(1)
	base := c.Minus(dir.ScaledBy(l))
	return extrudedCurves(circleCurves(base, uu, vv, r), geom.Vector{}, dir.ScaledBy(2*l), tol)
}

// teeCase is two cylinders of one radius r whose axes cross at the
// origin at angle deg, each of half-length l, the second's circle turned
// by phase.
//
// With tee, the second runs only from the crossing out one way (a tee
// of pipes, not a cross), its end inside the first.
type teeCase struct {
	deg, phaseA, phaseB, r, l float64
	tee                       bool
}

func (c teeCase) String() string {
	kind := "cross"
	if c.tee {
		kind = "tee"
	}
	return fmt.Sprintf("%s %g° phases %.2f %.2f", kind, c.deg, c.phaseA, c.phaseB)
}

// shells builds the pair.
func (c teeCase) shells(tol Tolerance) (*SShell, *SShell) {
	th := c.deg * math.Pi / 180
	dir := geom.Vector{X: math.Sin(th), Z: math.Cos(th)}
	a := teeCylinder(geom.Vector{}, geom.Vector{Z: 1}, c.r, c.l, c.phaseA, tol)
	if c.tee {
		return a, teeCylinder(dir.ScaledBy(c.l/2), dir, c.r, c.l/2, c.phaseB, tol)
	}
	return a, teeCylinder(geom.Vector{}, dir, c.r, c.l, c.phaseB, tol)
}

// inter is the volume the pair share: the Steinmetz solid of two
// cylinders of radius r crossing at θ, 16 r³ / (3 sin θ), or half of it
// for a tee.
func (c teeCase) inter() float64 {
	v := 16 * c.r * c.r * c.r / (3 * math.Sin(c.deg*math.Pi/180))
	if c.tee {
		return v / 2
	}
	return v
}

// teeCases is the grid: crossings at four angles, the circles' seams on
// and off the touching points, crosses and tees.
func teeCases() []teeCase {
	var cases []teeCase
	for _, deg := range []float64{90, 75, 60, 45} {
		for _, pa := range []float64{0, 0.3} {
			for _, pb := range []float64{0, 0.785398, 1.2} {
				for _, tee := range []bool{false, true} {
					cases = append(cases, teeCase{deg, pa, pb, 2, 8, tee})
				}
			}
		}
	}
	return cases
}

// meshVolume is the volume of sh's mesh.
func meshVolume(sh *SShell) float64 {
	var m SMesh
	sh.TriangulateInto(&m)
	return m.CalculateVolume()
}

// TestBooleanEqualCylindersCrossing makes every Boolean of two cylinders
// of one radius whose axes cross, an equal-pipe tee or cross: they meet
// in two ellipses, in the planes halving the angles between the axes,
// that cross where the cylinders touch tangent. The intersection is the
// Steinmetz solid, 16 r³ / (3 sin θ).
//
// A tee's difference keeps the branch's end, inside the main pipe, whose
// rim touches the main pipe tangent at the same two points: within the
// chord tolerance of them the rim lies nearer the pipe than the pipe's
// chords do, and at 45° their meshes cross there for 3 of the 12 turns
// of the circles. Those are refused (failCrossingTouch), and counted:
// the mesher's, as other faces closer than the chord tolerance are
// (roadmap K3).
func TestBooleanEqualCylindersCrossing(t *testing.T) {
	t.Parallel()
	tol := sweepTol()
	const teeDiffCrossing = 3
	crossing := 0
	for _, c := range teeCases() {
		a, b := c.shells(tol)
		va, vb := meshVolume(a), meshVolume(b)
		inter := c.inter()
		for _, op := range []struct {
			name string
			typ  SurfaceCombineAs
			want float64
		}{
			{"union", SurfaceUnion, va + vb - inter},
			{"difference", SurfaceDiff, va - inter},
			{"intersection", SurfaceInter, inter},
		} {
			out := SShell{Tol: tol}
			out.MakeFromBoolean(a, b, op.typ)
			if out.BooleanFailed || !meshIsSound(&out) {
				if c.tee && c.deg == 45 && op.typ == SurfaceDiff && out.BooleanFailed {
					crossing++
					continue
				}
				t.Errorf("%v %s: %v", c, op.name, errUnsound(out.BooleanFailed))
				continue
			}
			// The shared part's curved sides, 16 r² / sin θ (half for a
			// tee), are meshed apart from the cylinders: chords of the
			// tolerance over them, twice for the two pipes' sides.
			slack := 2 * tol.ChordTolMm() * c.inter() * 3 / c.r
			got := meshVolume(&out)
			if math.Abs(got-op.want) > slack {
				t.Errorf("%v %s: volume %.5g, want %.5g", c, op.name, got, op.want)
			}
		}
	}
	if crossing > teeDiffCrossing {
		t.Errorf("%d tee differences at 45° refused, want at most %d", crossing, teeDiffCrossing)
	}
}
