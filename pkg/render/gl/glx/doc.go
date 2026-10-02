// Package glx makes OpenGL contexts with GLX. It needs cgo, Xlib and
// libGL; built without them (or with the nogl tag) every constructor
// returns ErrUnavailable, so callers fall back to the software renderer.
package glx

import "errors"

// ErrUnavailable is returned where the program was built without GLX.
var ErrUnavailable = errors.New("glx: built without OpenGL support")
