package srf

import (
	"github.com/telesapr/telesapr/pkg/geom"
)

// A face of one operand may meet a round face of the other along an
// edge of its own, where the face across that edge lies on the round
// face's cylinder or cone: a fillet, tangent along its edge to a rod,
// or a chamfer meeting the rod at its edge, against a cutter's face
// lying on the rod's cylinder (the fillet past a curved side carried on
// above its face, the cutter less the rod). Tangent, the two surfaces
// touch along the edge rather than cross, and the march found no curve
// there but a few points where seams cross it; across, the march
// followed the edge with points of its own, and the faces either side
// did not meet. The edge, where it lies inside the round face's patch,
// is the curve where the two meet, as the edge of a block standing on a
// plane is the line where its side meets that plane.

// alongRoundEdges adds to into, as curves where s (of agnstA) meets b
// (of agnstB), the pieces of the operands' curves, split so far, that
// bound s along a face lying on b's cylinder or cone, or b along one on
// s's, where they lie inside the other's patch; and reports whether it
// added any.
func (s *SSurface) alongRoundEdges(b *SSurface, agnstA, agnstB *SShell, into *curveSink) bool {
	added := false
	for _, own := range []struct {
		srf, other *SSurface
		sh         *SShell
	}{{s, b, agnstA}, {b, s, agnstB}} {
		if own.sh == nil || (own.other.DegM != 1 && own.other.DegN != 1) {
			continue
		}
		f, ok := RoundFaceOf(own.other)
		if !ok {
			continue
		}
		for _, t := range own.srf.Trim {
			pc := own.sh.FindCurve(t.Curve)
			if pc == nil {
				continue
			}
			h := pc.SurfA
			if h == own.srf.H {
				h = pc.SurfB
			}
			nb := own.sh.FindSurface(h)
			if nb == nil || !sameRoundSurface(nb, own.other) {
				continue
			}
			c := into.shell.FindCurve(pc.NewH)
			if c == nil || len(c.Points) < 2 {
				continue
			}
			for _, run := range runsInside(c.Points, own.other, f) {
				into.add(SCurve{SurfA: s.H, SurfB: b.H, Source: CurveSourceIntersection, Points: run})
				added = true
			}
		}
	}
	return added
}

// runsInside are the runs of pts, a curve's points lying on the
// cylinder or cone f of patch, whose segments lie inside the patch's
// trim, each from one of pts' vertices to another. A segment is taken
// by its middle carried onto f, the chord dipping off it.
func runsInside(pts []SCurvePt, patch *SSurface, f RoundFace) [][]SCurvePt {
	var out [][]SCurvePt
	var run []SCurvePt
	flush := func() {
		// A run ends at a vertex of the curve, where it leaves the patch
		// at the patch's trim; one that ends between vertices is cut
		// back to them.
		for len(run) > 0 && !run[len(run)-1].Vertex {
			run = run[:len(run)-1]
		}
		for len(run) > 0 && !run[0].Vertex {
			run = run[1:]
		}
		if len(run) >= 2 {
			out = append(out, run)
		}
		run = nil
	}
	reach := patch.Tol.ChordTolMm() + geom.LengthEps
	for k := 1; k < len(pts); k++ {
		m := f.onSurface(pts[k-1].P.Plus(pts[k].P).ScaledBy(0.5))
		var uv geom.Point2d
		patch.ClosestPointTo(m, &uv, true)
		in := patch.PointAtUv(uv).Minus(m).Magnitude() < reach && inTrimUv(patch, uv) &&
			(patch.Bsp == nil || patch.Bsp.ClassifyPoint(uv, geom.Point2d{}, patch) == BspUvInside)
		if !in {
			flush()
			continue
		}
		if len(run) == 0 {
			run = append(run, pts[k-1])
		}
		run = append(run, pts[k])
	}
	flush()
	return out
}
