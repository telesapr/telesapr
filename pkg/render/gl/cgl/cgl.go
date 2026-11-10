//go:build darwin && cgo && !nogl

package cgl

/*
#cgo CFLAGS: -fobjc-arc -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa -framework OpenGL
#include "cgl_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

// Context is an NSOpenGLContext with the view it draws in.
type Context struct {
	box    C.cglBox
	double bool
}

// NewForWindow makes a context drawing over the toolkit's view with the
// window ID id, which is its canvas's platform ID (an index into
// takigo's registry, not an NSView pointer).
func NewForWindow(id uint64) (*Context, error) {
	if id == 0 {
		return nil, errors.New("cgl: no view")
	}
	var why *C.char
	box := C.cglNew(C.uint64_t(id), (**C.char)(unsafe.Pointer(&why)))
	if box == nil {
		return nil, fmt.Errorf("cgl: %s", C.GoString(why))
	}
	return &Context{box: box, double: C.cglDouble(box) != 0}, nil
}

// MakeCurrent makes the view follow the canvas's size, then binds the
// context to the calling OS thread, which must stay locked to its
// goroutine while it draws.
func (c *Context) MakeCurrent() error {
	if c.box == nil {
		return errors.New("cgl: context destroyed")
	}
	if C.cglMakeCurrent(c.box) == 0 {
		return errors.New("cgl: can't make the context current")
	}
	return nil
}

// DoubleBuffered reports whether frames are drawn into a back buffer
// that SwapBuffers shows.
func (c *Context) DoubleBuffered() bool { return c.double }

// SwapBuffers shows the back buffer.
func (c *Context) SwapBuffers() {
	if c.box != nil {
		C.cglSwap(c.box)
	}
}

// Size is the view's size in points.
func (c *Context) Size() (width, height int) {
	if c.box == nil {
		return 0, 0
	}
	var w, h C.int
	C.cglSize(c.box, &w, &h)
	return int(w), int(h)
}

// Destroy frees the context and removes its view. The toolkit's view
// stays.
func (c *Context) Destroy() {
	if c.box != nil {
		C.cglDestroy(c.box)
		c.box = nil
	}
}
