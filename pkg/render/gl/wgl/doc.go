// Package wgl makes OpenGL contexts with WGL on Windows. It needs cgo
// and a C compiler; built without them (or with the nogl tag) every
// constructor returns ErrUnavailable, so callers fall back to the
// software renderer.
package wgl

import "errors"

// ErrUnavailable is returned where the program was built without WGL.
var ErrUnavailable = errors.New("wgl: built without OpenGL support")
