package tkgui

import (
	"log"

	"github.com/takigo/takigo/window"

	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/gl"
	"github.com/telesapr/telesapr/pkg/render/gl/glx"
)

// The drawing area's OpenGL state. The context draws on the canvas's
// X window from a second Xlib connection, since takigo's own has no
// GLX; it is made once and kept while the window draws with GL. A
// build without GL gets glx's stubs, which fail, and so the software
// renderer.

// makeGL returns a new GL renderer on the canvas's window, making the
// context the first time.
func (c *canvas) makeGL() (render.ViewportCanvas, error) {
	if c.glCtx == nil {
		if c.Win.PlatformID == 0 {
			window.MakeWindowExist(c.Win)
		}
		// The window must exist on the server before the second
		// connection looks at it.
		c.w.p.app.Server().Flush()
		ctx, err := glx.NewForWindow(uint64(c.Win.PlatformID))
		if err != nil {
			return nil, err
		}
		c.glCtx = ctx
	}
	r, err := gl.New(c.glCtx)
	if err != nil {
		c.dropGL()
		return nil, err
	}
	c.glr = r
	// What the software path showed is stale now.
	c.frame, c.shown = nil, nil
	return r, nil
}

// dropGL frees the GL context, if any.
func (c *canvas) dropGL() {
	if c.glCtx != nil {
		c.glCtx.Destroy()
	}
	c.glCtx, c.glr = nil, nil
}

// presentGL shows the frame the GL renderer drew.
func (c *canvas) presentGL() {
	if c.Destroyed() || c.Win.PlatformID == 0 {
		return
	}
	c.glCtx.SwapBuffers()
}

// glFailed reports, once per run, why the windows draw in software.
func (p *Platform) glFailed(err error) {
	if p.glReported {
		return
	}
	p.glReported = true
	log.Printf("OpenGL is not available, drawing in software: %v", err)
}
