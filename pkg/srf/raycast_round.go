package srf

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
)

// sameRoundSurface reports whether patches a and b lie on one cylinder
// or one cone: the same axis line, and the same radius, or the same apex
// and half angle opening the same way, each within LengthEps (1e-9 for
// the angle). Their normals may point either way.
//
// Two such patches, cut at other angles round the axis (a cutter swept
// through its own breaks lying on a bore lathed in quarters), have an
// edge of one running across the inside of the other, where the
// classification's normals, taken along chords, dip off the surface by
// its curvature, past the coincidence test of ClassifyRegion: the region
// beside the edge read as inside or outside, not as lying on the face,
// and the Boolean lost its loops (roadmap K3, 2026-11-18). The rules
// that take such a region for coincident hold for spheres and for these
// only: tried on every curved face lying within a tenth of the chord
// tolerance of the other, they moved threads and text wrapped on
// cylinders and cones, whose faces are fitted, not exact (2026-11-04).
func sameRoundSurface(a, b *SSurface) bool {
	if a == nil || b == nil || (a.DegM < 2 && a.DegN < 2) || (b.DegM < 2 && b.DegN < 2) {
		return false
	}
	fa, ok := RoundFaceOf(a)
	if !ok {
		return false
	}
	fb, ok := RoundFaceOf(b)
	if !ok || fa.IsCone() != fb.IsCone() {
		return false
	}
	dot := fa.Axis.Dot(fb.Axis)
	if math.Abs(dot) < 1-1e-9 {
		return false
	}
	if !fa.IsCone() {
		return math.Abs(fa.Radius-fb.Radius) <= geom.LengthEps &&
			fb.Point.DistanceToLine(fa.Point, fa.Axis) <= geom.LengthEps
	}
	return dot > 0 && fa.Point.Equals(fb.Point) && math.Abs(fa.HalfAngle-fb.HalfAngle) <= 1e-9
}
