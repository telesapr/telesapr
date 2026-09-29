package tkgui

import (
	"image"
	"time"

	"github.com/msorc/takigo/event"
	"github.com/msorc/takigo/font"
	"github.com/msorc/takigo/geometry/pack"
	tkplatform "github.com/msorc/takigo/platform"
	"github.com/msorc/takigo/widget"
	"github.com/msorc/takigo/window"

	"github.com/spacesolve/spacesolve/pkg/platform"
)

// canvas is the drawing area: a widget showing the last frame the
// application rendered (the C++ GtkGLWidget).
type canvas struct {
	widget.Base
	w *Window

	frame         *image.RGBA
	renderPending bool
}

func newCanvas(w *Window) *canvas {
	app := w.p.app
	win := window.NewChildWindow(w.top.Window(), w.p.newName("canvas"), 0, 0, 1, 1)
	window.MakeWindowExist(win)
	c := &canvas{w: w}
	widget.InitBase(&c.Base, win, app)
	win.Class = "SolveSpaceCanvas"
	win.Flags |= window.FlagFocusable
	win.ReqWidth, win.ReqHeight = 300, 200
	win.BackgroundPixel = 0

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

func (c *canvas) pack() {
	pack.Pack(c, pack.SideOpt(pack.Left), pack.FillOpt(pack.FillBoth), pack.Expand(true))
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

// present shows a frame and keeps it for exposures.
func (c *canvas) present(frame *image.RGBA) {
	c.frame = frame
	c.blit()
}

// blit copies the frame to the window, and the tooltip over it.
func (c *canvas) blit() {
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
