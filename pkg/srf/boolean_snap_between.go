package srf

import (
	"math"
)

// snapBetween is the fuse of roadmap K3 for round faces that lie closer
// than the chord tolerance but further than the tenth of it the first
// fuse takes (snapToCylinders, snapToCones, snapToSpheres, snapTangent):
// a pin 0.03 mm over a bore's size at a chord tolerance of 0.1 mm. Their
// sides cut each other in curves the mesh cannot tell apart, and of 576
// such Booleans of round faces (TestBetweenContactSweep, 2026-11-05)
// 143 failed and 31 left the mesh open. It returns b fused by
// radii up to the chord tolerance, or b itself if no pair lies in that
// band; MakeFromBoolean tries the moved one only where b as drawn gave
// a Boolean that failed or a mesh that is not sound (retryFused). The
// planes are left: two planes as close come out right as drawn.
func snapBetween(a, b *SShell) *SShell {
	chord := a.Tol.Or(b.Tol).ChordTolMm()
	if chord/snapPlaneDiv <= snapPlaneFloor {
		return b
	}
	return snapTangent(a, snapToSpheres(a, snapToCones(a, snapToCylinders(a, b, chord), chord), chord), chord)
}

// retryFused makes the Boolean of a and wide, b fused by snapBetween,
// where the Boolean of a and b made into s failed or left its mesh open,
// and keeps it if it is sound, saying how far b moved (Fused). Where
// neither is sound and a and b are, the Boolean fails: its mesh open
// along the meeting, the solid was not what was drawn, and nothing in
// regeneration meshes it to tell.
func (s *SShell) retryFused(a, b, wide *SShell, op SurfaceCombineAs) {
	if !s.BooleanFailed && !s.meshUnsound() {
		return
	}
	var w SShell
	w.Tol, w.pace = s.Tol, s.pace
	w.makeBoolean(a, wide, op)
	if !w.BooleanFailed && !w.meshUnsound() {
		w.Fused = largestMove(b, wide)
		*s = w
		return
	}
	if !s.BooleanFailed && !a.meshUnsound() && !b.meshUnsound() {
		s.BooleanFailed = true
	}
}

// meshUnsound says whether s's mesh has naked edges or crosses itself.
func (s *SShell) meshUnsound() bool {
	var m SMesh
	s.TriangulateInto(&m)
	if m.IsEmpty() {
		return false
	}
	var naked SEdgeList
	var inters, leaks bool
	SKdNodeFrom(&m).MakeCertainEdgesInto(&naked, EdgeNakedOrSelfInter, true, &inters, &leaks, 0)
	return inters || leaks || len(naked.Edges) > 0
}

// largestMove is how far any point of a grid over b's curved patches
// lies from the same one of moved, a copy of b with its points moved:
// the patches' control points off them move further than their surfaces
// do (an arc's middle one by √2 as far), and a flat cap's corners, past
// its trim, further than its outline. A b of planes alone, moved whole
// onto a's round (snapTangent), moved by as much as any of its corners.
func largestMove(b, moved *SShell) float64 {
	curved, flat := 0.0, 0.0
	for i := range min(len(b.Surface), len(moved.Surface)) {
		sb, sm := &b.Surface[i], &moved.Surface[i]
		d := &curved
		if sb.DegM == 1 && sb.DegN == 1 {
			d = &flat
		}
		for u := 0.0; u <= 1; u += 0.25 {
			for v := 0.0; v <= 1; v += 0.25 {
				*d = math.Max(*d, sb.PointAt(u, v).Minus(sm.PointAt(u, v)).Magnitude())
			}
		}
	}
	if curved > 0 {
		return curved
	}
	return flat
}
