// Package cgl makes OpenGL contexts on macOS, with NSOpenGLContext in
// its legacy profile (the fixed-function GL 2.1 the renderer draws
// with). It needs cgo; built without it (or with the nogl tag) every
// constructor returns ErrUnavailable, so callers fall back to the
// software renderer.
//
// The context draws in a view of its own laid over the toolkit's
// canvas view, which follows the canvas's size and passes mouse events
// through. Its surface has a pixel per device pixel, and PixelRatio
// tells the renderer how many that is to a point.
package cgl

import "errors"

// ErrUnavailable is returned where the program was built without
// OpenGL on macOS.
var ErrUnavailable = errors.New("cgl: built without OpenGL support")
