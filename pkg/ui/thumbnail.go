package ui

import (
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/render"
	"github.com/spacesolve/spacesolve/pkg/render/soft"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

// WriteThumbnail renders the sketch seen along the projection vectors,
// zoomed to fit a width by height viewport, and saves it as a PNG (the
// thumbnail command of C++ solvespace-cli). It draws without a window,
// with the software renderer.
func (ss *SolveSpaceUI) WriteThumbnail(filename string, width, height uint, projRight, projUp geom.Vector) error {
	gw := ss.GW
	camera := render.Camera{
		PixelRatio: 1,
		GridFit:    true,
		Width:      float64(width),
		Height:     float64(height),
		ProjUp:     projUp,
		ProjRight:  projRight,
	}

	gw.view.ProjUp = projUp
	gw.view.ProjRight = projRight
	gw.view.Scale = gw.zoomToFit(&camera, false, false)
	camera.Scale = gw.view.Scale
	camera.Offset = gw.view.Offset
	ss.GenerateAll(sketch.GenerateAll)

	canvas := soft.New()
	gw.shareBitmapFont(canvas)
	lighting := gw.GetLighting()
	gw.lighting = lighting
	canvas.SetLighting(lighting)
	canvas.SetCamera(camera)

	canvas.StartFrame()
	gw.Draw(canvas)
	canvas.FlushFrame()
	canvas.FinishFrame()
	err := writeFramePng(canvas.ReadFrame(), filename)
	canvas.Clear()
	return err
}

// SetProjection turns the view to look along projRight and projUp.
func (gw *GraphicsWindow) SetProjection(projRight, projUp geom.Vector) {
	gw.view.ProjRight = projRight
	gw.view.ProjUp = projUp
}
