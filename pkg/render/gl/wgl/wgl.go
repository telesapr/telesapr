//go:build windows && cgo && !nogl

package wgl

/*
#cgo LDFLAGS: -lopengl32 -lgdi32 -luser32
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <GL/gl.h>

// wglSetup gives hdc a double-buffered RGBA pixel format with a depth
// buffer and makes a context on it. It returns 0 and the reason in
// *step, naming the call that failed, or 1.
static int wglSetup(HDC hdc, HGLRC *out, int *double_buffered, const char **step) {
	PIXELFORMATDESCRIPTOR pfd;
	ZeroMemory(&pfd, sizeof pfd);
	pfd.nSize = sizeof pfd;
	pfd.nVersion = 1;
	pfd.dwFlags = PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER;
	pfd.iPixelType = PFD_TYPE_RGBA;
	pfd.cColorBits = 24;
	pfd.cAlphaBits = 8;
	pfd.cDepthBits = 24;
	pfd.cStencilBits = 8;
	pfd.iLayerType = PFD_MAIN_PLANE;
	int fmt = ChoosePixelFormat(hdc, &pfd);
	if(fmt == 0) {
		*step = "ChoosePixelFormat";
		return 0;
	}
	if(!SetPixelFormat(hdc, fmt, &pfd)) {
		*step = "SetPixelFormat";
		return 0;
	}
	PIXELFORMATDESCRIPTOR got;
	DescribePixelFormat(hdc, fmt, sizeof got, &got);
	// A generic (GDI) format is Microsoft's software GL 1.1: slower than
	// the program's own renderer, so refuse it.
	if((got.dwFlags & PFD_GENERIC_FORMAT) && !(got.dwFlags & PFD_GENERIC_ACCELERATED)) {
		*step = "no hardware pixel format";
		return 0;
	}
	*double_buffered = (got.dwFlags & PFD_DOUBLEBUFFER) != 0;
	HGLRC rc = wglCreateContext(hdc);
	if(rc == NULL) {
		*step = "wglCreateContext";
		return 0;
	}
	*out = rc;
	return 1;
}

static void wglClientSize(HWND hwnd, int *w, int *h) {
	RECT r;
	if(GetClientRect(hwnd, &r)) {
		*w = r.right - r.left;
		*h = r.bottom - r.top;
	}
}

// wglHandle turns the toolkit's window handle back into an HWND.
static HWND wglHandle(unsigned long long v) {
	return (HWND)(UINT_PTR)v;
}

static HWND wglHiddenWindow(int w, int h) {
	return CreateWindowExA(0, "STATIC", "", WS_POPUP, 0, 0, w, h, NULL, NULL, GetModuleHandleA(NULL), NULL);
}
*/
import "C"

import (
	"errors"
	"fmt"
)

// Context is a WGL context with its window.
type Context struct {
	hwnd   C.HWND
	hdc    C.HDC
	rc     C.HGLRC
	own    bool // hwnd is a hidden window made here
	double bool
	width  int
	height int
}

// NewForWindow makes a context on the window with the handle hwnd. The
// window's pixel format is set once and can't be changed, so a window
// is given one context.
func NewForWindow(hwnd uint64) (*Context, error) {
	c := &Context{hwnd: C.wglHandle(C.ulonglong(hwnd))}
	if c.hwnd == nil {
		return nil, errors.New("wgl: no window")
	}
	var w, h C.int
	C.wglClientSize(c.hwnd, &w, &h)
	c.width, c.height = int(w), int(h)
	if err := c.create(); err != nil {
		return nil, err
	}
	return c, nil
}

// NewOffscreen makes a context on a hidden window of the given size.
// Windows has no GL context without a window's device context.
func NewOffscreen(width, height int) (*Context, error) {
	hwnd := C.wglHiddenWindow(C.int(width), C.int(height))
	if hwnd == nil {
		return nil, errors.New("wgl: can't make the hidden window")
	}
	c := &Context{hwnd: hwnd, own: true, width: width, height: height}
	if err := c.create(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Context) create() error {
	// The window class has no CS_OWNDC, so this is a shared device
	// context; it is kept until Destroy, which makes it ours.
	c.hdc = C.GetDC(c.hwnd)
	if c.hdc == nil {
		c.release()
		return errors.New("wgl: can't get the window's device context")
	}
	var step *C.char
	var double C.int
	if C.wglSetup(c.hdc, &c.rc, &double, &step) == 0 {
		err := fmt.Errorf("wgl: %s failed", C.GoString(step))
		c.release()
		return err
	}
	c.double = double != 0
	return nil
}

// release frees the device context and the hidden window.
func (c *Context) release() {
	if c.hdc != nil {
		C.ReleaseDC(c.hwnd, c.hdc)
		c.hdc = nil
	}
	if c.own && c.hwnd != nil {
		C.DestroyWindow(c.hwnd)
	}
	c.hwnd = nil
}

// MakeCurrent binds the context to the calling OS thread, which must
// stay locked to its goroutine while it draws.
func (c *Context) MakeCurrent() error {
	if c.rc == nil {
		return errors.New("wgl: context destroyed")
	}
	if C.wglGetCurrentContext() == c.rc && C.wglGetCurrentDC() == c.hdc {
		return nil
	}
	if C.wglMakeCurrent(c.hdc, c.rc) == 0 {
		return errors.New("wgl: can't make the context current")
	}
	return nil
}

// DoubleBuffered reports whether frames are drawn into a back buffer
// that SwapBuffers shows.
func (c *Context) DoubleBuffered() bool { return c.double }

// SwapBuffers shows the back buffer.
func (c *Context) SwapBuffers() {
	if c.rc == nil {
		return
	}
	if c.double {
		C.SwapBuffers(c.hdc)
	}
}

// Size is the window's client size when the context was made.
func (c *Context) Size() (width, height int) { return c.width, c.height }

// Destroy frees the context, the device context and the hidden window
// if one was made. The toolkit's window stays.
func (c *Context) Destroy() {
	if c.rc == nil {
		return
	}
	if C.wglGetCurrentContext() == c.rc {
		C.wglMakeCurrent(nil, nil)
	}
	C.wglDeleteContext(c.rc)
	c.rc = nil
	c.release()
}
