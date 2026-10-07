//go:build linux && cgo && !nogl

package glx

/*
#cgo pkg-config: x11 gl
#include <stdlib.h>
#include <X11/Xlib.h>
#include <GL/glx.h>

static int glxErrorCode;

static int glxTrapErrors(Display *dpy, XErrorEvent *ev) {
	(void)dpy;
	if(glxErrorCode == 0) {
		glxErrorCode = ev->error_code;
	}
	return 0;
}

typedef int (*glxHandler)(Display *, XErrorEvent *);

static glxHandler glxTrapStart(void) {
	glxErrorCode = 0;
	return XSetErrorHandler(glxTrapErrors);
}

static int glxTrapEnd(Display *dpy, glxHandler prev) {
	XSync(dpy, False);
	XSetErrorHandler(prev);
	return glxErrorCode;
}

static int glxDefaultScreen(Display *dpy) {
	return DefaultScreen(dpy);
}
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

// Context is a GLX context with its drawable.
type Context struct {
	dpy      *C.Display
	ctx      C.GLXContext
	drawable C.GLXDrawable
	pbuffer  bool
	double   bool
	width    int
	height   int
}

var initThreads sync.Once

func openDisplay() (*C.Display, error) {
	// Xlib must know of threads before the first call that opens a
	// display; a toolkit that called it already makes this a no-op.
	initThreads.Do(func() { C.XInitThreads() })
	dpy := C.XOpenDisplay(nil)
	if dpy == nil {
		return nil, errors.New("glx: can't open the X display")
	}
	var errBase, evBase C.int
	if C.glXQueryExtension(dpy, &errBase, &evBase) == 0 {
		C.XCloseDisplay(dpy)
		return nil, errors.New("glx: the X server has no GLX extension")
	}
	var major, minor C.int
	if C.glXQueryVersion(dpy, &major, &minor) == 0 || major < 1 || (major == 1 && minor < 3) {
		C.XCloseDisplay(dpy)
		return nil, fmt.Errorf("glx: GLX %d.%d, need 1.3", major, minor)
	}
	return dpy, nil
}

// chooseConfig picks the RGBA config of the given drawable type with
// the deepest depth buffer; with visualID set, only one of that X
// visual qualifies, since a window can only be drawn with its own.
func chooseConfig(dpy *C.Display, drawableType C.int, double bool, visualID C.VisualID) (C.GLXFBConfig, error) {
	db := C.int(C.False)
	if double {
		db = C.True
	}
	attrs := []C.int{
		C.GLX_X_RENDERABLE, C.True,
		C.GLX_DRAWABLE_TYPE, drawableType,
		C.GLX_RENDER_TYPE, C.GLX_RGBA_BIT,
		C.GLX_RED_SIZE, 8,
		C.GLX_GREEN_SIZE, 8,
		C.GLX_BLUE_SIZE, 8,
		C.GLX_DEPTH_SIZE, 16,
		C.GLX_DOUBLEBUFFER, db,
		C.None,
	}
	var n C.int
	cfgs := C.glXChooseFBConfig(dpy, C.glxDefaultScreen(dpy), &attrs[0], &n)
	if cfgs == nil || n == 0 {
		return nil, errors.New("glx: no RGBA config with a depth buffer")
	}
	defer C.XFree(unsafe.Pointer(cfgs))
	list := unsafe.Slice(cfgs, int(n))
	var best C.GLXFBConfig
	bestDepth := C.int(-1)
	for _, cfg := range list {
		if visualID != 0 {
			var vid C.int
			C.glXGetFBConfigAttrib(dpy, cfg, C.GLX_VISUAL_ID, &vid)
			if C.VisualID(vid) != visualID {
				continue
			}
		}
		var depth C.int
		C.glXGetFBConfigAttrib(dpy, cfg, C.GLX_DEPTH_SIZE, &depth)
		if depth > bestDepth {
			best, bestDepth = cfg, depth
		}
	}
	if best == nil {
		return nil, fmt.Errorf("glx: no config for X visual 0x%x", uint64(visualID))
	}
	return best, nil
}

// NewForWindow makes a double-buffered context that draws on the X
// window with the given id, which belongs to another connection to the
// same display.
func NewForWindow(window uint64) (*Context, error) {
	dpy, err := openDisplay()
	if err != nil {
		return nil, err
	}
	c := &Context{dpy: dpy, double: true}
	prev := C.glxTrapStart()
	var attrs C.XWindowAttributes
	ok := C.XGetWindowAttributes(dpy, C.Window(window), &attrs) != 0 //nolint:gocritic // cgo expands the call; the comparison is not duplicated
	if code := C.glxTrapEnd(dpy, prev); code != 0 || !ok {
		C.XCloseDisplay(dpy)
		return nil, fmt.Errorf("glx: no X window 0x%x (error %d)", window, int(code))
	}
	cfg, err := chooseConfig(dpy, C.GLX_WINDOW_BIT, true, C.XVisualIDFromVisual(attrs.visual))
	if err != nil {
		C.XCloseDisplay(dpy)
		return nil, err
	}
	if err := c.create(cfg); err != nil {
		return nil, err
	}
	c.drawable = C.GLXDrawable(window)
	c.width, c.height = int(attrs.width), int(attrs.height)
	return c, nil
}

// NewOffscreen makes a context that draws on a pixel buffer of width
// by height pixels, with no window.
func NewOffscreen(width, height int) (*Context, error) {
	if width < 1 || height < 1 {
		return nil, fmt.Errorf("glx: a pixel buffer of %d by %d", width, height)
	}
	dpy, err := openDisplay()
	if err != nil {
		return nil, err
	}
	cfg, err := chooseConfig(dpy, C.GLX_PBUFFER_BIT, false, 0)
	if err != nil {
		C.XCloseDisplay(dpy)
		return nil, err
	}
	c := &Context{dpy: dpy, pbuffer: true, width: width, height: height}
	if err := c.create(cfg); err != nil {
		return nil, err
	}
	attrs := []C.int{C.GLX_PBUFFER_WIDTH, C.int(width), C.GLX_PBUFFER_HEIGHT, C.int(height), C.None}
	prev := C.glxTrapStart()
	pb := C.glXCreatePbuffer(dpy, cfg, &attrs[0])
	if code := C.glxTrapEnd(dpy, prev); code != 0 || pb == 0 {
		c.Destroy()
		return nil, fmt.Errorf("glx: can't make a pixel buffer (error %d)", int(code))
	}
	c.drawable = pb
	return c, nil
}

func (c *Context) create(cfg C.GLXFBConfig) error {
	prev := C.glxTrapStart()
	ctx := C.glXCreateNewContext(c.dpy, cfg, C.GLX_RGBA_TYPE, nil, C.True)
	if code := C.glxTrapEnd(c.dpy, prev); code != 0 || ctx == nil {
		C.XCloseDisplay(c.dpy)
		c.dpy = nil
		return fmt.Errorf("glx: can't make a context (error %d)", int(code))
	}
	c.ctx = ctx
	return nil
}

// MakeCurrent binds the context to the calling OS thread, which must
// stay locked to its goroutine while it draws.
func (c *Context) MakeCurrent() error {
	if c.dpy == nil {
		return errors.New("glx: context destroyed")
	}
	if C.glXGetCurrentContext() == c.ctx && C.glXGetCurrentDrawable() == c.drawable {
		return nil
	}
	if C.glXMakeContextCurrent(c.dpy, c.drawable, c.drawable, c.ctx) == 0 {
		return errors.New("glx: can't make the context current")
	}
	return nil
}

// DoubleBuffered reports whether frames are drawn into a back buffer
// that SwapBuffers shows.
func (c *Context) DoubleBuffered() bool { return c.double }

// SwapBuffers shows the back buffer, and waits until the server has
// it, so that what the toolkit draws over the window next lands on top.
func (c *Context) SwapBuffers() {
	if c.dpy == nil {
		return
	}
	if c.double {
		C.glXSwapBuffers(c.dpy, c.drawable)
	}
	C.XSync(c.dpy, C.False)
}

// Size is the drawable's size when the context was made.
func (c *Context) Size() (width, height int) { return c.width, c.height }

// Destroy frees the context, the pixel buffer if any, and the
// connection.
func (c *Context) Destroy() {
	if c.dpy == nil {
		return
	}
	if C.glXGetCurrentContext() == c.ctx {
		C.glXMakeContextCurrent(c.dpy, 0, 0, nil)
	}
	if c.pbuffer && c.drawable != 0 {
		C.glXDestroyPbuffer(c.dpy, c.drawable)
	}
	if c.ctx != nil {
		C.glXDestroyContext(c.dpy, c.ctx)
	}
	C.XCloseDisplay(c.dpy)
	c.dpy, c.ctx, c.drawable = nil, nil, 0
}
