package ui

import (
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/soft"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// WriteThumbnail renders the sketch seen along the projection vectors,
// zoomed to fit a width by height viewport, and saves it as a PNG. It
// draws without a window, with the software renderer.
func (ss *Application) WriteThumbnail(filename string, width, height uint, projRight, projUp geom.Vector) error {
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
	gw.view.Scale = gw.zoomToFit(&camera, cameraArea(&camera), false, false)
	camera.Scale = gw.view.Scale
	camera.Offset = gw.view.Offset
	ss.GenerateAll(sketch.GenerateAll)
	return ss.writePicture(filename, camera)
}

// WriteWindowPicture draws a PNG of width by height pixels of the view
// the window shows (windowPictureCamera).
func (ss *Application) WriteWindowPicture(filename string, width, height uint) error {
	gw := ss.GW
	camera := gw.windowPictureCamera(width, height)
	gw.view.Scale, gw.view.Offset = camera.Scale, camera.Offset
	ss.GenerateAll(sketch.GenerateAll)
	return ss.writePicture(filename, camera)
}

// windowPictureCamera is the camera of a picture of width by height
// pixels of the view the window shows: its direction, and the part of
// the window the toolbar leaves free, whose middle is the picture's,
// scaled so that as much of the model goes across the picture as across
// that part. What Zoom To Fit fitted in the window fills the picture as
// it does the window, with no room left for a toolbar the picture
// doesn't have.
func (gw *graphicsWindow) windowPictureCamera(width, height uint) render.Camera {
	camera := gw.Camera()
	area := gw.freeArea(&camera)
	// The point drawn in the middle of the free area goes to the middle.
	camera.Offset = camera.Offset.Minus(camera.ProjRight.ScaledBy(area.Mid.U / camera.Scale)).Minus(
		camera.ProjUp.ScaledBy(area.Mid.V / camera.Scale))
	scale := camera.Scale
	if area.Width > 0 {
		scale *= float64(width) / area.Width
	}
	camera.PixelRatio, camera.GridFit = 1, true
	camera.Width, camera.Height, camera.Scale = float64(width), float64(height), scale
	return camera
}

// writePicture draws the model with camera into a PNG file.
func (ss *Application) writePicture(filename string, camera render.Camera) error {
	gw := ss.GW
	canvas := soft.New()
	gw.shareBitmapFont(canvas)
	lighting := gw.Lighting()
	gw.lighting = lighting
	canvas.SetLighting(lighting)
	canvas.SetCamera(camera)

	canvas.StartFrame()
	gw.inPicture = true
	gw.Draw(canvas)
	gw.inPicture = false
	canvas.FlushFrame()
	canvas.FinishFrame()
	err := writeFramePng(canvas.ReadFrame(), filename)
	canvas.Clear()
	return err
}

// SetProjection turns the view to look along projRight and projUp.
func (gw *graphicsWindow) SetProjection(projRight, projUp geom.Vector) {
	gw.view.ProjRight = projRight
	gw.view.ProjUp = projUp
}
