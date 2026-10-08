// Package gl is an OpenGL 1 renderer: a render.ViewportCanvas drawing
// with fixed-function GL through go-gl. Polygons are triangulated by
// the kernel's triangulator.
//
// The renderer doesn't make its context: it is given one (see package
// glx) and makes it current before it draws. GL output depends on the
// driver, so goldens use the software renderer (package soft), which
// follows the same semantics; this one is pinned by smoke tests.
//
// Built without cgo, or with the nogl tag, New returns ErrUnavailable.
package gl

import "errors"

// ErrUnavailable is returned where the program was built without
// OpenGL.
var ErrUnavailable = errors.New("gl: built without OpenGL support")

// Context is the GL context a renderer draws with.
type Context interface {
	// MakeCurrent binds the context to the calling thread.
	MakeCurrent() error
	// DoubleBuffered reports whether frames are drawn into a back
	// buffer.
	DoubleBuffered() bool
}
