package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// A group screen's rows for edges and faces of the solid by name, a
// chamfer's or fillet's SolidEdges and a shell's SolidFaces, hover and
// select them on the model as an entity's row does an entity. The edge
// or face is of the solid before the group, which the selection
// carries (Selection.Solid), so it is drawn there even while the group
// itself is active and the edge is cut away or the face taken away; a
// face is highlighted on the mesh shown, so it shows while the group
// before is active.

// solidRow is the group shown and the group whose running solid its
// named edges and faces are of, or nil when the screen shows no group.
func (tw *textWindow) solidRow() (*sketch.Group, core.HGroup) {
	sk := tw.ss.SK
	g := sk.Group(tw.shown.group)
	if g == nil {
		return nil, core.HGroup{}
	}
	prev := g.PreviousGroup(sk)
	if prev == nil {
		return g, core.HGroup{}
	}
	if solid := sk.SolidAt(prev.H); solid != nil {
		return g, solid.H
	}
	return g, core.HGroup{}
}

// ScreenHoverSolidEdge hovers edge v of the shown group's SolidEdges.
func (tw *textWindow) ScreenHoverSolidEdge(link int, v uint32) {
	g, solid := tw.solidRow()
	if g == nil || int(v) >= len(g.SolidEdges) {
		return
	}
	gw := tw.ss.GW
	gw.hover.Clear()
	gw.hover.Edge, gw.hover.Solid = g.SolidEdges[v], solid
}

// ScreenSelectSolidEdge selects edge v of the shown group's SolidEdges.
func (tw *textWindow) ScreenSelectSolidEdge(link int, v uint32) {
	g, solid := tw.solidRow()
	if g == nil || int(v) >= len(g.SolidEdges) {
		return
	}
	gw := tw.ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Edge: g.SolidEdges[v], Solid: solid})
}

// ScreenHoverSolidFace hovers face v of the shown group's SolidFaces.
func (tw *textWindow) ScreenHoverSolidFace(link int, v uint32) {
	g, solid := tw.solidRow()
	if g == nil || int(v) >= len(g.SolidFaces) {
		return
	}
	gw := tw.ss.GW
	gw.hover.Clear()
	gw.hover.Face, gw.hover.Solid = g.SolidFaces[v], solid
}

// ScreenSelectSolidFace selects face v of the shown group's SolidFaces.
func (tw *textWindow) ScreenSelectSolidFace(link int, v uint32) {
	g, solid := tw.solidRow()
	if g == nil || int(v) >= len(g.SolidFaces) {
		return
	}
	gw := tw.ss.GW
	gw.ClearSelection()
	gw.selection = append(gw.selection, Selection{Face: g.SolidFaces[v], Solid: solid})
}
