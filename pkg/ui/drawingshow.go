package ui

import (
	"fmt"
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/srf"
)

// The drawing sheet shown in the window (roadmap 29.1): View > Show
// Drawing Sheet draws the sheet, as File > Export > Drawing writes it, in
// place of the model, fitted to the window, until it is chosen again.
// It is view state, not saved. While it is shown the mouse picks
// nothing in the model; the drawing's screen still edits the sheet, and
// what is shown follows each change.

// sheetCache is the sheet last made for the window, and what it was
// made from.
type sheetCache struct {
	key   string
	edges srf.SEdgeList
	err   error
}

// SheetShown reports whether the window shows the drawing sheet.
func (gw *graphicsWindow) SheetShown() bool { return gw.sheetShown }

// ShowSheet shows the drawing sheet in the window, or the model again.
// It is an error to show a sheet that can't be made, as of a group with
// no solid; the window then stays on the model.
func (gw *graphicsWindow) ShowSheet(on bool) error {
	if on {
		if _, err := gw.sheetEdges(); err != nil {
			return err
		}
	}
	gw.sheetShown = on
	gw.syncSheetMenu()
	gw.persistentDirty = true
	gw.Invalidate(true)
	gw.ss.ScheduleShowTW()
	return nil
}

// MenuShowDrawingSheet turns the sheet in the window on or off.
func (gw *graphicsWindow) MenuShowDrawingSheet() {
	if err := gw.ShowSheet(!gw.sheetShown); err != nil {
		gw.ss.Error("Can't show the drawing sheet: %s.", err.Error())
	}
}

// syncSheetMenu checks View > Show Drawing Sheet when the sheet is shown.
func (gw *graphicsWindow) syncSheetMenu() {
	if gw.sheetMenuItem != nil {
		gw.sheetMenuItem.SetActive(gw.sheetShown)
	}
}

// sheetEdges is the drawing's sheet as it would be written, made again
// only when the model, the active group or the drawing has changed.
func (gw *graphicsWindow) sheetEdges() (*srf.SEdgeList, error) {
	ss := gw.ss
	d := ss.drawing()
	key := fmt.Sprintf("%d %x %v", ss.SK.Generation, gw.activeGroupH().V, *d)
	if gw.sheet.key != key {
		el, _, _, err := ss.DrawingSheet(d)
		gw.sheet = sheetCache{key: key, edges: el, err: err}
	}
	return &gw.sheet.edges, gw.sheet.err
}

// sheetCamera looks square at the sheet, which lies in the plane z = 0
// in mm from its bottom left, fitted to a window the size of camera's.
func (gw *graphicsWindow) sheetCamera(camera render.Camera) render.Camera {
	w, h := gw.ss.drawing().Size()
	c := camera
	c.ProjRight, c.ProjUp = geom.Vector{X: 1}, geom.Vector{Y: 1}
	c.Offset = geom.Vector{X: -w / 2, Y: -h / 2}
	c.Tangent = 0
	c.Scale = math.Min(camera.Width/(w*1.06), camera.Height/(h*1.06))
	return c
}

// drawSheet draws the drawing sheet in place of the model: the paper's
// edge, then the sheet's lines in their styles, hidden edges dashed.
// A canvas with no camera of its own to set, as the picker's, gets
// nothing: the sheet is not picked.
func (gw *graphicsWindow) drawSheet(canvas render.Canvas) {
	ss := gw.ss
	vc, ok := canvas.(render.ViewportCanvas)
	if !ok {
		return
	}
	vc.SetCamera(gw.sheetCamera(*canvas.Camera()))
	w, h := ss.drawing().Size()
	var paper srf.SEdgeList
	rect(&paper, geom.Vector{}, geom.Vector{X: w, Y: h}, core.StyleSolidEdge)
	el, _ := gw.sheetEdges()
	solid, hidden := srf.SEdgeList{}, srf.SEdgeList{}
	for _, e := range append(paper.Edges, el.Edges...) {
		if e.AuxA == core.StyleHiddenEdge {
			hidden.Edges = append(hidden.Edges, e)
		} else {
			solid.Edges = append(solid.Edges, e)
		}
	}
	strokeSolid := ss.styleStroke(hstyle(core.StyleSolidEdge))
	strokeSolid.Layer = render.LayerFront
	canvas.DrawEdges(&solid, canvas.InternStroke(strokeSolid))
	strokeHidden := ss.styleStroke(hstyle(core.StyleHiddenEdge))
	strokeHidden.Layer = render.LayerFront
	canvas.DrawEdges(&hidden, canvas.InternStroke(strokeHidden))
}
