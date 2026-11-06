package ui

import (
	"math"
	"testing"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// meshExtent is the mesh's triangle count, volume and bounds.
func meshExtent(m *srf.SMesh) (n int, vol float64, lo, hi geom.Vector) {
	lo = geom.Vector{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)}
	hi = lo.ScaledBy(-1)
	for _, t := range m.Triangles {
		vol += t.A.Dot(t.B.Cross(t.C)) / 6
		for _, p := range []geom.Vector{t.A, t.B, t.C} {
			lo = geom.Vector{X: min(lo.X, p.X), Y: min(lo.Y, p.Y), Z: min(lo.Z, p.Z)}
			hi = geom.Vector{X: max(hi.X, p.X), Y: max(hi.Y, p.Y), Z: max(hi.Z, p.Z)}
		}
	}
	return len(m.Triangles), vol, lo, hi
}

// During a drag of a mechanism's part the assembly is drawn from its
// parts' own meshes moved; when the drag ends it is meshed as ever. The
// two have the same bounds and the same volume to well within the
// chords' sag, but not the same triangles: a part meshed where it is
// turned can be cut otherwise than in its own frame (the ear clipper's
// order on a plane, the grid on a curved face), here 1700 triangles
// against 1684 and a volume 7e-6 apart. So the moved mesh is only
// drawn, and only during the drag.
func TestDragDrawsMovedParts(t *testing.T) {
	t.Parallel()
	ss, w, end := mechanismChain(t, 4)
	q := ss.GW.ProjectPoint(end.Pos())
	startDrag(ss, end.H, q)
	ss.GW.dragPoints(q.U+15, q.V-10, false, false)
	ss.Refresh()
	ss.GW.Invalidate(false)
	w.Render()
	g := ss.SK.Group(ss.GW.activeGroupH())
	if !g.DisplayDirty && !hasMoved(ss) {
		t.Fatal("the drag drew the assembly meshed as ever")
	}
	moved := g.DisplayMesh.ExactCopy()
	movedOutlines := len(g.DisplayOutlines.Outlines)

	ss.GW.MouseLeftUp(q.U+15, q.V-10, false, false)
	if hasMoved(ss) {
		t.Fatal("the drag's end left the moved display")
	}
	ss.GW.Invalidate(false)
	w.Render()
	g = ss.SK.Group(ss.GW.activeGroupH())
	n0, v0, lo0, hi0 := meshExtent(&moved)
	n1, v1, lo1, hi1 := meshExtent(&g.DisplayMesh)
	near := func(a, b int) bool { return math.Abs(float64(a-b)) <= 0.05*float64(b) }
	if !near(n0, n1) || math.Abs(v0-v1) > 1e-4*math.Abs(v1) || lo0.Minus(lo1).Magnitude() > 1e-6 || hi0.Minus(hi1).Magnitude() > 1e-6 {
		t.Errorf("moved: %d triangles, volume %g, %v to %v; meshed: %d, %g, %v to %v", n0, v0, lo0, hi0, n1, v1, lo1, hi1)
	}
	if !near(movedOutlines, len(g.DisplayOutlines.Outlines)) {
		t.Errorf("%d outlines moved, %d meshed", movedOutlines, len(g.DisplayOutlines.Outlines))
	}
}

// hasMoved reports whether some group's display was made from moved
// parts' meshes.
func hasMoved(ss *Application) bool {
	moved := false
	ss.SK.RangeMovedDisplays(func() { moved = true })
	return moved
}
