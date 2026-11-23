//go:build darwin && cgo && !nogl

package cgl

/*
#include "cgl_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// NewOffscreen makes a context that draws into a framebuffer object of
// width by height pixels, with no window. It takes an accelerated
// pixel format and, where the machine has none (a virtual one with no
// GPU), the software renderer; Software says which. Nothing needs the
// main thread or a running application. The context is not made
// current.
func NewOffscreen(width, height int) (*Context, error) {
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("cgl: a framebuffer of %d by %d", width, height)
	}
	var why *C.char
	var kind C.int
	off := C.cglOffNew(C.int(width), C.int(height), (**C.char)(unsafe.Pointer(&why)), &kind)
	if off == nil {
		return nil, fmt.Errorf("cgl: %s", C.GoString(why))
	}
	return &Context{off: off, width: width, height: height, software: kind == 2}, nil
}

// Software reports whether an offscreen context is on the software
// renderer, not on a GPU.
func (c *Context) Software() bool { return c.software }

// Framebuffered reports whether the context draws into a framebuffer
// object, which a frame is read back from its colour attachment: true
// for an offscreen context.
func (c *Context) Framebuffered() bool { return c.off != nil }
