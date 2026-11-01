//go:build js && wasm

package web

import (
	"image"
	"sync"
	"syscall/js"
	"time"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/soft"
)

// canvasKind is which kind of context a window's canvas has: it has
// one for good, so changing it means a new canvas.
type canvasKind int

const (
	canvasNone canvasKind = iota
	canvas2D
	canvasGL
)

// Window is a window in the page: the main window fills its left, a tool
// window is a panel in the column at its right. Its drawing area is a
// canvas.
type Window struct {
	p      *Platform
	kind   platform.WindowKind
	events platform.WindowEvents
	js     js.Value

	visible    bool
	fullScreen bool
	minW, minH float64

	canvasKind canvasKind
	gl         render.ViewportCanvas

	renderPending bool
	renderFn      js.Func

	scroll struct{ lo, hi, page, pos float64 }
	clicks clickTracker

	// motion is the latest pointer motion not yet delivered: motion
	// events come faster than frames are drawn, and the queue holds
	// only the newest.
	mu      sync.Mutex
	motion  *platform.MouseEvent
	resized bool
}

var _ platform.Window = (*Window)(nil)

// CreateWindow makes a window. The first toplevel is the main window; a
// tool window, or any other toplevel, is a panel beside it, hidden until
// shown.
func (p *Platform) CreateWindow(kind platform.WindowKind, _ platform.Window) platform.Window {
	w := &Window{p: p, kind: kind, visible: kind == platform.WindowToplevel && p.mainWin == nil}
	isTool := !(kind == platform.WindowToplevel && p.mainWin == nil)
	cb := js.Global().Get("Object").New()
	cb.Set("mouse", p.fn(w.onMouse))
	cb.Set("resize", p.fn(w.onResize))
	cb.Set("scroll", p.fn(w.onScroll))
	cb.Set("editDone", p.fn(w.onEditDone))
	cb.Set("editEscape", p.fn(w.onEditEscape))
	cb.Set("close", p.fn(func([]js.Value) any {
		p.q.post(func() {
			if w.events.OnClose != nil {
				w.events.OnClose()
			}
		})
		return nil
	}))
	cb.Set("focus", p.fn(func([]js.Value) any { p.focused = w; return nil }))
	w.js = p.w.Call("createWindow", isTool, cb)
	w.renderFn = js.FuncOf(func(js.Value, []js.Value) any {
		p.q.post(w.render)
		return nil
	})
	p.w.Call("listenFullscreen", p.fn(func(a []js.Value) any {
		on := a[0].Bool()
		p.q.post(func() {
			if w.fullScreen != on {
				w.fullScreen = on
				if w.events.OnFullScreen != nil {
					w.events.OnFullScreen(on)
				}
			}
		})
		return nil
	}))
	if p.mainWin == nil && !isTool {
		p.mainWin = w
	}
	p.windows = append(p.windows, w)
	return w
}

// Events returns the window's callbacks.
func (w *Window) Events() *platform.WindowEvents { return &w.events }

// PixelDensity is 96 dots per CSS inch times the device pixel ratio.
func (w *Window) PixelDensity() float64 { return 96 * w.DevicePixelRatio() }

// DevicePixelRatio is the page's: 2 on a high-density screen or when
// zoomed in.
func (w *Window) DevicePixelRatio() float64 {
	r := w.p.w.Call("devicePixelRatio").Float()
	if r <= 0 {
		return 1
	}
	return r
}

// DeviceFontScale is 1: CSS pixels are already scaled to the display.
func (w *Window) DeviceFontScale() float64 {
	return w.PixelDensity() / w.DevicePixelRatio() / 96.0
}

// IsVisible reports whether the window is shown.
func (w *Window) IsVisible() bool { return w.visible }

// SetVisible shows or hides the window.
func (w *Window) SetVisible(v bool) {
	if v == w.visible {
		return
	}
	w.visible = v
	w.js.Call("setVisible", v)
	if v {
		w.Invalidate()
	}
}

// Focus gives the window the keyboard.
func (w *Window) Focus() {
	w.p.focused = w
	w.js.Call("focus")
}

// IsFullScreen reports whether the page was made full screen.
func (w *Window) IsFullScreen() bool { return w.fullScreen }

// SetFullScreen asks the browser to show the page full screen, which it
// does only to a page the user has just acted on.
func (w *Window) SetFullScreen(full bool) {
	if full == w.fullScreen {
		return
	}
	w.js.Call("setFullScreen", full)
}

// SetTitle sets the title of a tool window's panel; the main window's
// title is the page's, with the application's name after it.
func (w *Window) SetTitle(title string) {
	w.js.Call("setTitle", title)
	if w == w.p.mainWin {
		w.p.w.Call("setTitle", title+" — TeleSAPR")
	}
}

// SetTitleForFilename returns false: the title names the file.
func (w *Window) SetTitleForFilename(string) bool { return false }

// SetMenuBar puts the menu bar across the top of the page.
func (w *Window) SetMenuBar(platform.MenuBar) {}

// ContentSize is the drawing area's size in CSS pixels. A window that
// is not shown has none, and reports the size it asks for.
func (w *Window) ContentSize() (width, height float64) {
	s := w.js.Call("size")
	width, height = s.Index(0).Float(), s.Index(1).Float()
	if width < 1 || height < 1 {
		return max(w.minW, 300), max(w.minH, 200)
	}
	return width, height
}

// SetMinContentSize keeps the drawing area at least this big.
func (w *Window) SetMinContentSize(width, height float64) {
	w.minW, w.minH = width, height
	w.js.Call("setMinSize", width, height)
}

// FreezePosition saves the size of a panel; a page has no positions.
func (w *Window) FreezePosition(s core.Settings, key string) {
	if !w.visible || w.kind != platform.WindowTool {
		return
	}
	width, height := w.ContentSize()
	s.FreezeInt(key+"_Width", uint32(width))
	s.FreezeInt(key+"_Height", uint32(height))
}

// ThawPosition restores the size of a panel FreezePosition saved: its
// width, which the column of panels takes.
func (w *Window) ThawPosition(s core.Settings, key string) {
	if w.kind != platform.WindowTool {
		return
	}
	if width := s.ThawInt(key+"_Width", 0); width >= 200 {
		w.js.Call("setColumnWidth", float64(width))
	}
}

// SetCursor sets the drawing area's cursor.
func (w *Window) SetCursor(c platform.Cursor) {
	if c == platform.CursorHand {
		w.js.Call("setCursor", "pointer")
		return
	}
	w.js.Call("setCursor", "default")
}

// SetTooltip shows text while the pointer rests in the rectangle.
func (w *Window) SetTooltip(text string, x, y, width, height float64) {
	w.js.Call("setTooltip", text, x, y, width, height)
}

// IsEditorVisible reports whether the editor is shown.
func (w *Window) IsEditorVisible() bool { return w.js.Call("editorVisible").Bool() }

// ShowEditor shows a text box over the drawing area, with its text's
// baseline at x, y.
func (w *Window) ShowEditor(x, y, fontHeight, minWidth float64, isMonospace bool, text string) {
	w.js.Call("showEditor", x, y, fontHeight, minWidth, isMonospace, text)
}

// HideEditor hides the editor and gives the keyboard back to the
// drawing area.
func (w *Window) HideEditor() { w.js.Call("hideEditor") }

// SetScrollbarVisible shows or hides the scrollbar.
func (w *Window) SetScrollbarVisible(v bool) {
	w.js.Call("setScrollbarVisible", v)
	if v {
		w.js.Call("configureScrollbar", w.scroll.lo, w.scroll.hi, w.scroll.page, w.scroll.pos)
	}
}

// ConfigureScrollbar sets the range and page size, keeping the value.
func (w *Window) ConfigureScrollbar(lo, hi, pageSize float64) {
	s := &w.scroll
	s.lo, s.hi, s.page = lo, hi, pageSize
	s.pos = clampScroll(s.pos, lo, hi, pageSize)
	w.js.Call("configureScrollbar", lo, hi, pageSize, s.pos)
}

// ScrollbarPosition is the scrollbar's value.
func (w *Window) ScrollbarPosition() float64 { return w.scroll.pos }

// SetScrollbarPosition sets the value, and reports it as the other
// backends do.
func (w *Window) SetScrollbarPosition(pos float64) {
	s := &w.scroll
	pos = clampScroll(pos, s.lo, s.hi, s.page)
	if pos == s.pos {
		return
	}
	s.pos = pos
	w.js.Call("setScrollbarPosition", pos)
	if w.events.OnScrollbarAdjusted != nil {
		w.events.OnScrollbarAdjusted(pos)
	}
}

// onScroll is the user scrolling the box: top is its scrollTop.
func (w *Window) onScroll(a []js.Value) any {
	top, spacer, lo, hi := a[0].Float(), a[1].Float(), a[2].Float(), a[3].Float()
	w.p.q.post(func() {
		s := &w.scroll
		pos := clampScroll(valueForScrollTop(top, lo, hi, spacer), s.lo, s.hi, s.page)
		if pos == s.pos {
			return
		}
		s.pos = pos
		if w.events.OnScrollbarAdjusted != nil {
			w.events.OnScrollbarAdjusted(pos)
		}
	})
	return nil
}

// Invalidate asks for OnRender to run on the next frame.
func (w *Window) Invalidate() {
	if w.renderPending {
		return
	}
	w.renderPending = true
	js.Global().Call("requestAnimationFrame", w.renderFn)
}

// render runs OnRender, for the frame Invalidate asked for.
func (w *Window) render() {
	w.renderPending = false
	if !w.visible || w.events.OnRender == nil {
		return
	}
	start := time.Now()
	w.events.OnRender()
	d := time.Since(start)
	st := &w.p.stats
	st.Frames++
	st.Last, st.Total, st.Max = d, st.Total+d, max(st.Max, d)
}

// onResize is the drawing area changing size: the frame is drawn anew,
// and a WebGL canvas takes its new size.
func (w *Window) onResize([]js.Value) any {
	w.mu.Lock()
	pending := w.resized
	w.resized = true
	w.mu.Unlock()
	if pending {
		return nil
	}
	w.p.q.post(func() {
		w.mu.Lock()
		w.resized = false
		w.mu.Unlock()
		if w.gl != nil {
			pw, ph := w.physicalSize()
			w.js.Call("setBacking", pw, ph)
			if r, ok := w.gl.(interface{ Resize(width, height int) }); ok {
				r.Resize(pw, ph)
			}
		}
		if w.visible {
			w.Invalidate()
		}
	})
	return nil
}

// physicalSize is the drawing area in device pixels.
func (w *Window) physicalSize() (int, int) {
	cw, ch := w.ContentSize()
	r := w.DevicePixelRatio()
	return max(int(cw*r+0.5), 1), max(int(ch*r+0.5), 1)
}

// CreateRenderer returns, with useGL for the graphics window and a
// WebGL renderer made available, that renderer, else the software
// renderer. The canvas of the old kind is replaced: it can have only one.
func (w *Window) CreateRenderer(useGL bool) render.ViewportCanvas {
	if useGL && w.p.NewGLRenderer != nil && w.kind == platform.WindowToplevel {
		w.closeGL()
		canvas := w.js.Call("newCanvas")
		pw, ph := w.physicalSize()
		canvas.Set("width", pw)
		canvas.Set("height", ph)
		r, err := w.p.NewGLRenderer(canvas, pw, ph)
		if err == nil {
			w.gl, w.canvasKind = r, canvasGL
			return r
		}
		w.p.glFailed(err)
		w.canvasKind = canvasNone
	}
	w.closeGL()
	if w.canvasKind != canvas2D {
		if w.canvasKind != canvasNone {
			w.js.Call("newCanvas")
		}
		w.canvasKind = canvas2D
	}
	return soft.New()
}

func (w *Window) closeGL() {
	if c, ok := w.gl.(interface{ Close() }); ok {
		c.Close()
	}
	w.gl = nil
}

// PresentFrame shows the frame canvas has just finished. A WebGL
// renderer has drawn into the page's canvas already.
func (w *Window) PresentFrame(canvas render.ViewportCanvas) {
	if w.gl != nil && canvas == w.gl {
		return
	}
	if w.canvasKind != canvas2D {
		return // a frame of a renderer that was replaced
	}
	img := canvas.ReadFrame()
	if img == nil {
		return
	}
	b := img.Bounds()
	wd, ht := b.Dx(), b.Dy()
	if wd == 0 || ht == 0 {
		return
	}
	buf := w.js.Call("frameBuffer", wd, ht)
	copyPixels(buf, img, wd, ht)
	w.js.Call("putFrame")
}

// copyPixels copies the frame into an ImageData's bytes.
func copyPixels(dst js.Value, img *image.RGBA, wd, ht int) {
	if img.Stride == 4*wd {
		js.CopyBytesToJS(dst, img.Pix[:4*wd*ht])
		return
	}
	packed := make([]byte, 4*wd*ht)
	for y := range ht {
		copy(packed[4*wd*y:4*wd*(y+1)], img.Pix[y*img.Stride:])
	}
	js.CopyBytesToJS(dst, packed)
}

// --- input

// onMouse is the page's pointer, wheel and leave events for the window:
// type, x, y, shift, control, button, delta, page x, page y, time (ms),
// delta mode.
func (w *Window) onMouse(a []js.Value) any {
	typ := platform.MouseEventType(a[0].Int())
	ev := platform.MouseEvent{
		Type:        typ,
		X:           a[1].Float(),
		Y:           a[2].Float(),
		ShiftDown:   a[3].Bool(),
		ControlDown: a[4].Bool(),
	}
	raw := a[5].Int()
	switch typ {
	case platform.MouseMotion:
		ev.Button = buttonFromMask(raw)
	case platform.MousePress, platform.MouseRelease:
		ev.Button = buttonFromBrowser(raw)
	case platform.MouseScrollVert:
		ev.ScrollDelta = wheelDelta(a[6].Float(), a[10].Int())
	}
	w.p.pointerX, w.p.pointerY = a[7].Float(), a[8].Float()
	ms := a[9].Float()
	w.p.focused = w

	if typ == platform.MouseMotion {
		w.mu.Lock()
		first := w.motion == nil
		w.motion = &ev
		w.mu.Unlock()
		if first {
			w.p.q.post(w.flushMotion)
		}
		return nil
	}
	// Whatever else comes keeps its place after the motion before it.
	w.mu.Lock()
	m := w.motion
	w.motion = nil
	w.mu.Unlock()
	if m != nil {
		w.p.q.post(func() { w.deliverMouse(*m) })
	}
	w.p.q.post(func() {
		if typ == platform.MousePress {
			ev.Type = w.clicks.press(ms, ev.Button, ev.X, ev.Y)
		}
		w.deliverMouse(ev)
	})
	return nil
}

func (w *Window) flushMotion() {
	w.mu.Lock()
	m := w.motion
	w.motion = nil
	w.mu.Unlock()
	if m != nil {
		w.deliverMouse(*m)
	}
}

func (w *Window) deliverMouse(ev platform.MouseEvent) {
	if w.p.modal > 0 || w.events.OnMouseEvent == nil {
		return
	}
	w.events.OnMouseEvent(ev)
}

// deliverKey gives the application the key as the other backends do:
// the first reading, and while it is not taken, the others.
func (w *Window) deliverKey(typ platform.KeyboardEventType, cands []platform.KeyboardEvent) {
	if w.events.OnKeyboardEvent == nil {
		return
	}
	for _, k := range cands {
		k.Type = typ
		if w.events.OnKeyboardEvent(k) || typ == platform.KeyRelease {
			return
		}
	}
}

// onEditDone is Enter in the editor.
func (w *Window) onEditDone(a []js.Value) any {
	text := a[0].String()
	w.p.q.post(func() {
		if w.events.OnEditingDone != nil {
			w.events.OnEditingDone(text)
		}
	})
	return nil
}

// onEditEscape is Escape in the editor, which goes to the drawing area's
// handler, which cancels the edit.
func (w *Window) onEditEscape([]js.Value) any {
	w.p.q.post(func() {
		if w.events.OnKeyboardEvent != nil {
			w.events.OnKeyboardEvent(platform.KeyboardEvent{Type: platform.KeyPress, Key: platform.KeyCharacter, Chr: '\x1b'})
		}
	})
	return nil
}
