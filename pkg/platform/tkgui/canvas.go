package tkgui

import (
	"bytes"
	"image"
	"time"

	"github.com/msorc/takigo/event"
	"github.com/msorc/takigo/font"
	"github.com/msorc/takigo/geometry/pack"
	tkplatform "github.com/msorc/takigo/platform"
	"github.com/msorc/takigo/widget"
	"github.com/msorc/takigo/window"

	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/gl/glx"
)

// canvas is the drawing area: a widget showing the last frame the
// application rendered (the C++ GtkGLWidget).
type canvas struct {
	widget.Base
	w *Window

	frame         *image.RGBA
	renderPending bool

	// shown is a copy of the frame last copied to the window, shownW by
	// shownH pixels, so that present can send only what changed.
	shown          []byte
	shownW, shownH int

	// With OpenGL, the context and the renderer drawing with it.
	glCtx *glx.Context
	glr   render.ViewportCanvas
}

func newCanvas(w *Window) *canvas {
	app := w.p.app
	win := window.NewChildWindow(w.top.Window(), w.p.newName("canvas"), 0, 0, 1, 1)
	makeCanvasWindow(win)
	c := &canvas{w: w}
	widget.InitBase(&c.Base, win, app)
	win.Class = "TeleSAPRCanvas"
	win.Flags |= window.FlagFocusable
	win.ReqWidth, win.ReqHeight = 300, 200

	disp := app.Dispatcher()
	disp.Bind(win.PlatformID, event.ExposureMask, func(ev *event.Event) {
		if ev.ExposeCount == 0 {
			c.blit()
		}
	})
	disp.Bind(win.PlatformID, event.StructureNotifyMask, func(ev *event.Event) {
		if ev.Type == event.ConfigureType {
			win.Width, win.Height = ev.ConfigWidth, ev.ConfigHeight
			c.invalidate()
		}
	})
	disp.Bind(win.PlatformID, event.ButtonPressMask, func(ev *event.Event) {
		if ev.Type != event.ButtonPressType {
			return
		}
		if !w.editorVisible() {
			app.FocusManager().SetFocus(win)
		}
		w.tooltip.hide(c)
		w.buttonPress(ev)
	})
	disp.Bind(win.PlatformID, event.ButtonReleaseMask, func(ev *event.Event) {
		if ev.Type != event.ButtonReleaseType || ev.Button == 4 || ev.Button == 5 {
			return
		}
		w.mouse(platform.MouseRelease, ev, ev.Button, 0)
	})
	disp.Bind(win.PlatformID, event.MotionMask, func(ev *event.Event) {
		if ev.Type != event.MotionType {
			return
		}
		w.tooltip.motion(c, ev.X, ev.Y)
		w.mouse(platform.MouseMotion, ev, 0, 0)
	})
	disp.Bind(win.PlatformID, event.LeaveMask, func(ev *event.Event) {
		if ev.Type != event.LeaveType {
			return
		}
		w.tooltip.hide(c)
		w.mouse(platform.MouseLeave, ev, 0, 0)
	})
	disp.Bind(win.PlatformID, event.KeyPressMask|event.KeyReleaseMask, func(ev *event.Event) {
		switch ev.Type {
		case event.KeyPressType:
			w.key(platform.KeyPress, ev)
		case event.KeyReleaseType:
			w.key(platform.KeyRelease, ev)
		}
	})
	return c
}

// makeCanvasWindow creates the drawing area's X window as
// window.MakeWindowExist does, but with no background and north-west
// bit gravity: the server then keeps the last frame in place when the
// window is resized, where a background would clear it and show a
// flash of that colour on every step of a resize until the next frame
// (GTK's GL area never clears either).
func makeCanvasWindow(win *window.Window) {
	parent := win.Parent
	if win.PlatformID != 0 || parent == nil || parent.PlatformID == 0 {
		window.MakeWindowExist(win)
		return
	}
	d := win.Display
	attrs := &tkplatform.WindowAttrs{
		BorderPixel: d.BlackPixel,
		BitGravity:  tkplatform.NorthWestGravity,
		EventMask: int64(tkplatform.KeyPressMask | tkplatform.KeyReleaseMask |
			tkplatform.ButtonPressMask | tkplatform.ButtonReleaseMask |
			tkplatform.PointerMotionMask | tkplatform.EnterWindowMask |
			tkplatform.LeaveWindowMask | tkplatform.ExposureMask |
			tkplatform.StructureNotifyMask | tkplatform.PropertyChangeMask |
			tkplatform.FocusChangeMask),
	}
	win.PlatformID = d.Server.CreateWindow(parent.PlatformID,
		win.X, win.Y, uint(win.Width), uint(win.Height), uint(win.BorderWidth),
		win.Depth, tkplatform.InputOutput,
		tkplatform.CWBorderPixel|tkplatform.CWBitGravity|tkplatform.CWEventMask, attrs)
	d.RegisterWindow(win.PlatformID, win)
	win.GC = d.Server.CreateGC(win.Drawable(), tkplatform.GCForeground|tkplatform.GCBackground,
		&tkplatform.GCValues{Foreground: d.BlackPixel, Background: 0})
}

func (c *canvas) pack() {
	_ = pack.Pack(c, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true))
}

// invalidate schedules one render at idle time.
func (c *canvas) invalidate() {
	if c.renderPending || c.Destroyed {
		return
	}
	c.renderPending = true
	c.w.p.app.DoWhenIdle(func() {
		c.renderPending = false
		if c.Destroyed || c.Win.Width <= 1 || c.Win.Height <= 1 {
			return
		}
		if c.w.events.OnRender != nil {
			c.w.events.OnRender()
		}
	})
}

// present shows a frame and keeps it for exposures. Only the rectangle
// that changed since the last frame is sent: a repaint for a hover or a
// drag changes a small part of the window, and sending the whole frame
// is most of such a repaint's cost.
func (c *canvas) present(frame *image.RGBA) {
	c.frame = frame
	b := frame.Bounds()
	if c.shown == nil || c.shownW != b.Dx() || c.shownH != b.Dy() || frame.Stride != 4*b.Dx() {
		c.blit()
		return
	}
	x0, y0, x1, y1, ok := changedRect(c.shown, frame.Pix, b.Dx(), b.Dy())
	if !ok {
		return
	}
	win := c.Win
	if c.Destroyed || win.PlatformID == 0 {
		return
	}
	// Keep the copy in step with what the window will show.
	for y := y0; y < y1; y++ {
		row := y * frame.Stride
		copy(c.shown[row+4*x0:row+4*x1], frame.Pix[row+4*x0:row+4*x1])
	}
	x1, y1 = min(x1, win.Width), min(y1, win.Height)
	if x1 > x0 && y1 > y0 {
		d := c.w.p.app.Server()
		d.PutImageRGBA(win.Drawable(), win.GC, win.Depth, frame.Pix, frame.Stride, b.Dx(), b.Dy(),
			x0, y0, x0, y0, x1-x0, y1-y0, 0)
		c.w.tooltip.draw(c)
		d.Flush()
	}
}

// changedRect is the bounding box, x0 to x1 and y0 to y1 exclusive, of
// the pixels that differ between two frames of width by height packed
// RGBA pixels; ok is false when none do.
func changedRect(prev, next []byte, width, height int) (x0, y0, x1, y1 int, ok bool) {
	stride := 4 * width
	y0, y1 = -1, -1
	for y := 0; y < height; y++ {
		row := y * stride
		if !bytes.Equal(prev[row:row+stride], next[row:row+stride]) {
			if y0 < 0 {
				y0 = y
			}
			y1 = y + 1
		}
	}
	if y0 < 0 {
		return 0, 0, 0, 0, false
	}
	x0, x1 = width, 0
	for y := y0; y < y1; y++ {
		p, n := prev[y*stride:(y+1)*stride], next[y*stride:(y+1)*stride]
		for x := 0; x < x0; x++ {
			if !bytes.Equal(p[4*x:4*x+4], n[4*x:4*x+4]) {
				x0 = x
				break
			}
		}
		for x := width - 1; x >= x1; x-- {
			if !bytes.Equal(p[4*x:4*x+4], n[4*x:4*x+4]) {
				x1 = x + 1
				break
			}
		}
	}
	if x0 >= x1 {
		// Rows differed only in bytes a row compare saw; send them whole.
		x0, x1 = 0, width
	}
	return x0, y0, x1, y1, true
}

// blit copies the whole frame to the window, and the tooltip over it.
// With GL, whose back buffer is gone after a swap, it renders anew.
func (c *canvas) blit() {
	if c.glr != nil {
		c.invalidate()
		return
	}
	win := c.Win
	if c.Destroyed || win.PlatformID == 0 {
		return
	}
	d := c.w.p.app.Server()
	if f := c.frame; f != nil {
		b := f.Bounds()
		width := min(b.Dx(), win.Width)
		height := min(b.Dy(), win.Height)
		if width > 0 && height > 0 {
			d.PutImageRGBA(win.Drawable(), win.GC, win.Depth, f.Pix, f.Stride, b.Dx(), b.Dy(),
				0, 0, 0, 0, width, height, 0)
		}
		c.shown = append(c.shown[:0], f.Pix...)
		c.shownW, c.shownH = b.Dx(), b.Dy()
	}
	c.w.tooltip.draw(c)
	d.Flush()
}

// tooltip is drawn over the frame after the pointer rests in its area,
// since takigo has no tooltips (the C++ backends use the toolkit's).
type tooltip struct {
	text       string
	x, y, w, h float64
	shown      bool
	cancel     func() bool
	px, py     int
}

// SetTooltip sets the text shown while the pointer rests in the area.
func (w *Window) SetTooltip(text string, x, y, width, height float64) {
	t := &w.tooltip
	if t.text == text && t.x == x && t.y == y && t.w == width && t.h == height {
		return
	}
	t.hide(w.canvas)
	t.text, t.x, t.y, t.w, t.h = text, x, y, width, height
	if text != "" {
		t.arm(w.canvas)
	}
}

func (t *tooltip) inside(x, y int) bool {
	fx, fy := float64(x), float64(y)
	return fx >= t.x && fx < t.x+t.w && fy >= t.y && fy < t.y+t.h
}

func (t *tooltip) arm(c *canvas) {
	if t.cancel != nil {
		t.cancel()
	}
	t.cancel = c.w.p.app.After(500*time.Millisecond, func() {
		t.cancel = nil
		if t.text != "" && t.inside(t.px, t.py) {
			t.shown = true
			c.blit()
		}
	})
}

func (t *tooltip) motion(c *canvas, x, y int) {
	t.px, t.py = x, y
	if t.text == "" {
		return
	}
	if !t.inside(x, y) {
		t.hide(c)
		return
	}
	if !t.shown && t.cancel == nil {
		t.arm(c)
	}
}

func (t *tooltip) hide(c *canvas) {
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
	if t.shown {
		t.shown = false
		c.blit()
	}
}

func (t *tooltip) draw(c *canvas) {
	if !t.shown || t.text == "" {
		return
	}
	app := c.w.p.app
	f, err := app.FontRegistry().Get(font.TkTooltipFont)
	if err != nil {
		return
	}
	df, ok := f.(tkplatform.DrawableFont)
	if !ok {
		return
	}
	bg, err1 := app.ColorCache().Get("#ffffe0")
	fg, err2 := app.ColorCache().Get("#000000")
	if err1 != nil || err2 != nil {
		return
	}
	m := f.Metrics()
	tw := f.MeasureString(t.text) + 8
	th := m.Linespace() + 4
	x := min(t.px+12, c.Win.Width-tw-1)
	y := min(t.py+20, c.Win.Height-th-1)
	x, y = max(x, 0), max(y, 0)
	d := app.Server()
	gc := c.Win.GC
	d.SetForeground(gc, bg.Pixel)
	d.FillRectangle(c.Win.Drawable(), gc, x, y, uint(tw), uint(th))
	d.SetForeground(gc, fg.Pixel)
	d.DrawRectangle(c.Win.Drawable(), gc, x, y, uint(tw-1), uint(th-1))
	df.DrawString(c.Win.Drawable(), x+4, y+2+m.Ascent, t.text, fg.Pixel, fg.Red, fg.Green, fg.Blue)
}
