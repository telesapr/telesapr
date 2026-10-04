package ui

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/soft"
	"github.com/telesapr/telesapr/pkg/sketch"
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
	// zoomToFit moves the offset by where the model falls with none,
	// so it starts from none, not from wherever the window looks.
	gw.view.Offset = geom.Vector{}
	gw.view.Scale = gw.zoomToFit(&camera, false, false)
	camera.Scale = gw.view.Scale
	camera.Offset = gw.view.Offset
	ss.GenerateAll(sketch.GenerateAll)
	return ss.writePicture(filename, camera)
}

// WriteWindowPicture draws a PNG of width by height pixels of the view
// the window shows: its direction and middle, scaled so that as much of
// the model goes across the picture as across the window.
func (ss *SolveSpaceUI) WriteWindowPicture(filename string, width, height uint) error {
	gw := ss.GW
	camera := gw.GetCamera()
	scale := camera.Scale
	if camera.Width > 0 {
		scale *= float64(width) / camera.Width
	}
	camera.PixelRatio, camera.GridFit = 1, true
	camera.Width, camera.Height, camera.Scale = float64(width), float64(height), scale
	gw.view.Scale = scale
	ss.GenerateAll(sketch.GenerateAll)
	return ss.writePicture(filename, camera)
}

// writePicture draws the model with camera into a PNG file.
func (ss *SolveSpaceUI) writePicture(filename string, camera render.Camera) error {
	gw := ss.GW
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
