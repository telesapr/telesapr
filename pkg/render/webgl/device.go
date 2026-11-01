package webgl

import "github.com/telesapr/telesapr/pkg/res"

// scene is what a run of triangles is drawn under besides its pipe: the
// projection, the lights and the clip plane. It changes a few times a
// frame, so the device compares it with the last it applied.
type scene struct {
	mvp [16]float32

	// lightDir are the directions toward the lights in model space,
	// unit length; lightI their intensities, 0 for a light that is off.
	lightDir [2][3]float32
	lightI   [2]float32
	ambient  float32
	// view points toward the viewer and up is the camera's up, both
	// unit length, for the shaded lighting.
	view, up [3]float32

	// clip keeps what has clip . (x, y, z, 1) >= 0 when clipOn.
	clip   [4]float32
	clipOn bool
}

// drawCall is one run of triangles to draw.
type drawCall struct {
	verts []float32 // vertexFloats each, three to a triangle
	pipe  pipe
	scene scene
}

// device draws what the renderer builds. The WebGL one is in
// device_js.go; the tests' records the calls.
type device interface {
	// resize sets the size of the drawing buffer and the viewport, in
	// device pixels.
	resize(width, height int)
	// clear fills the colour buffer with color, if colorToo, and the
	// depth buffer with the far plane.
	clear(color [4]float32, colorToo bool)
	// draw draws a run. The device must not keep c.verts.
	draw(c *drawCall)
	// invalidate drops the texture made of pm.
	invalidate(pm *res.Pixmap)
	// read is the frame drawn so far as RGBA, the bottom row first.
	read() (pix []byte, width, height int)
	ident() (vendor, renderer, version string)
	flush()
	finish()
	close()
}
