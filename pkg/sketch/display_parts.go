package sketch

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// While a part is dragged, an assembly of linked parts is drawn from
// each part's mesh made once in its own frame and moved by where its
// link places it now, rather than by triangulating the assembly again
// at every step of the drag: a part moves rigidly, so its mesh in its
// own frame is the one it had a step before. A chain of ten bars took
// some 120 ms a frame to mesh, against a millisecond to move. The mesh
// moved can differ from the assembly's own in the last bits, so it is
// drawn only during the drag: when the drag ends the display is made
// again as ever (RemakeMovedDisplays), and nothing saved or exported
// sees it.

// DragWatcher is Hooks whose owner can say a drag is going on, as the
// window can: the display of an assembly of linked parts is then made
// from their meshes moved (display_parts.go).
type DragWatcher interface {
	Dragging() bool
}

// dragging reports whether the sketch's owner says a drag is going on.
func (sk *Sketch) dragging() bool {
	w, ok := sk.hooks().(DragWatcher)
	return ok && w.Dragging()
}

// partDisplay is a link group's display mesh and outlines in its part's
// own frame (scaled, not moved or turned), and what they were made for.
type partDisplay struct {
	key      partDisplayKey
	mesh     srf.SMesh
	outlines srf.SOutlineList
}

// partDisplayKey is what a part's display depends on but its place:
// the linked shell (a reload makes a new one), the scale, the
// tolerance it is meshed at, and whether outlines are drawn.
type partDisplayKey struct {
	surface  *srf.SSurface
	surfaces int
	scale    float64
	tol      srf.Tolerance
	outlines bool
}

// movedDisplayParts are the link groups whose parts make up group g's
// solid, in order, when that solid is an assembly of linked parts
// only, each read as a shell and assembled with what is before it,
// none mirrored, none forced to a mesh: then g's display can be their
// meshes moved. Nil when g's solid is anything else.
func (sk *Sketch) movedDisplayParts(g *Group) []*Group {
	var parts []*Group
	for _, hg := range sk.GroupOrder {
		h := sk.Group(hg)
		if h.SuppressedBy.V == 0 && !h.Suppress && (len(h.ThisShell.Surface) > 0 || len(h.ThisMesh.Triangles) > 0) {
			if h.Type != GroupLinked || h.ForceToMesh || h.Scale <= 0 || len(h.ImpMesh.Triangles) > 0 ||
				len(h.ImpShell.Surface) == 0 || len(parts) > 0 && h.MeshCombine != CombineAssemble {
				return nil
			}
			parts = append(parts, h)
		}
		if hg == g.H {
			break
		}
	}
	if len(parts) < 2 {
		// One part is meshed as it is: nothing to gain.
		return nil
	}
	return parts
}

// partDisplayOf is link group g's display in its part's own frame,
// made now if what it depends on changed.
func (sk *Sketch) partDisplayOf(g *Group) *partDisplay {
	show := sk.visibility()
	key := partDisplayKey{surface: &g.ImpShell.Surface[0], surfaces: len(g.ImpShell.Surface), scale: g.Scale,
		tol: sk.tol, outlines: show.Edges || show.Outlines}
	if g.partDisplay != nil && g.partDisplay.key == key {
		return g.partDisplay
	}
	pd := &partDisplay{key: key}
	var sh srf.SShell
	sh.Tol = sk.tol
	sh.MakeFromTransformationOf(&g.ImpShell, geom.Vector{}, geom.QuaternionIdentity, g.Scale)
	sh.RemapFaces(shellGroup{g: g, sk: sk}, 0)
	giveBack := sk.lendPace(&sh)
	sh.TriangulateInto(&pd.mesh)
	giveBack()
	if key.outlines {
		var raw srf.SOutlineList
		pd.mesh.MakeOutlinesPacedInto(&raw, srf.EdgeSharpOrFace, sk.kernelPace())
		var builder srf.PolylineBuilder
		builder.MakeFromOutlines(&raw)
		builder.GenerateOutlines(&pd.outlines)
	}
	g.partDisplay = pd
	return pd
}

// generateMovedDisplayItems makes group g's display from its parts'
// meshes moved to where their links place them now, reporting false,
// with nothing made, where g's solid is no assembly of linked parts.
func (g *Group) generateMovedDisplayItems(sk *Sketch) bool {
	parts := sk.movedDisplayParts(g)
	if parts == nil {
		return false
	}
	g.DisplayMesh.Clear()
	g.DisplayOutlines.Clear()
	for _, p := range parts {
		pd := sk.partDisplayOf(p)
		t, q := p.placement(sk)
		for _, tr := range pd.mesh.Triangles {
			tr.A, tr.B, tr.C = q.Rotate(tr.A).Plus(t), q.Rotate(tr.B).Plus(t), q.Rotate(tr.C).Plus(t)
			tr.NA, tr.NB, tr.NC = q.Rotate(tr.NA), q.Rotate(tr.NB), q.Rotate(tr.NC)
			g.DisplayMesh.Triangles = append(g.DisplayMesh.Triangles, tr)
		}
		for _, o := range pd.outlines.Outlines {
			o.A, o.B = q.Rotate(o.A).Plus(t), q.Rotate(o.B).Plus(t)
			o.NL, o.NR = q.Rotate(o.NL), q.Rotate(o.NR)
			g.DisplayOutlines.Outlines = append(g.DisplayOutlines.Outlines, o)
		}
	}
	g.displayMoved = true
	return true
}

// placement is where link group g places its part: its move and its
// turn, as generateLinkedShell reads them.
func (g *Group) placement(sk *Sketch) (geom.Vector, geom.Quaternion) {
	t := geom.Vector{X: paramFloat(sk, g.H.Param(0)), Y: paramFloat(sk, g.H.Param(1)), Z: paramFloat(sk, g.H.Param(2))}
	q := geom.Quaternion{W: paramFloat(sk, g.H.Param(3)), X: paramFloat(sk, g.H.Param(4)),
		Y: paramFloat(sk, g.H.Param(5)), Z: paramFloat(sk, g.H.Param(6))}
	return t, q
}

// RemakeMovedDisplays has the display of every group drawn from moved
// parts' meshes during a drag made again as ever, as the window asks
// when the drag ends.
func (sk *Sketch) RemakeMovedDisplays() {
	for i := range sk.Groups {
		if g := &sk.Groups[i]; g.displayMoved {
			g.displayMoved = false
			g.DisplayDirty = true
		}
	}
}

// RangeMovedDisplays calls fn once for each group whose display is made
// from moved parts' meshes now; for a test of the window.
func (sk *Sketch) RangeMovedDisplays(fn func()) {
	for i := range sk.Groups {
		if sk.Groups[i].displayMoved {
			fn()
		}
	}
}
