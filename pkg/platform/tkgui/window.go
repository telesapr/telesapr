package tkgui

import (
	"encoding/binary"
	"strconv"
	"time"
	"unicode"

	"github.com/takigo/takigo/cursor"
	"github.com/takigo/takigo/event"
	tkplatform "github.com/takigo/takigo/platform"
	"github.com/takigo/takigo/screenunit"
	"github.com/takigo/takigo/widget"
	"github.com/takigo/takigo/widget/toplevel"
	"github.com/takigo/takigo/window"
	"github.com/takigo/takigo/wm"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/platform"
	"github.com/telesapr/telesapr/pkg/render"
	"github.com/telesapr/telesapr/pkg/render/soft"
	"github.com/telesapr/telesapr/pkg/res"
)

// Window is a toplevel with a menu bar, a drawing area, an editor over
// it and a scrollbar at its right.
type Window struct {
	p      *Platform
	events platform.WindowEvents

	top     widget.Caregiver
	topWin  *window.Window
	wmInfo  *wm.WmInfo
	toolTop *toplevel.Toplevel // nil for the main window, which is the root

	canvas    *canvas
	scrollbar *scrollbarAdapter
	editor    *editor
	menuBar   *MenuBar

	visible    bool
	fullScreen bool
	// Whether X has mapped the window, and whether Focus waits for it.
	mapped       bool
	focusPending bool

	tooltip tooltip

	// Double click detection (GDK's double-click-time and -distance).
	lastPress struct {
		button uint
		time   time.Time
		x, y   int
	}
}

var _ platform.Window = (*Window)(nil)

const (
	doubleClickTime     = 400 * time.Millisecond
	doubleClickDistance = 5
)

// CreateWindow makes a window. The first toplevel is the application's
// root window; a tool window stays over its parent.
func (p *Platform) CreateWindow(kind platform.WindowKind, parent platform.Window) platform.Window {
	w := &Window{p: p}
	if kind == platform.WindowToplevel && !p.rootUsed {
		p.rootUsed = true
		w.top = p.app
		w.topWin = p.app.Window()
		w.wmInfo = p.app.WmInfo()
	} else {
		opts := []toplevel.ToplevelOption{toplevel.Title("TeleSAPR")}
		if pw, ok := parent.(*Window); ok && pw != nil {
			opts = append(opts, toplevel.TransientFor(pw.topWin))
		}
		t := toplevel.New(p.app, p.newName("win"), opts...)
		w.toolTop = t
		w.top = t
		w.topWin = t.Window()
		w.wmInfo = t.WmInfo
		t.Hide()
	}
	setWindowIcon(w.topWin)
	// The drawing area has no background, so what shows in the part a
	// resize uncovers, until the next frame, is the toplevel's; make it
	// the default scene background rather than white.
	w.topWin.SetBackgroundPixel(0)
	p.app.RegisterCloseHandler(w.topWin.PlatformID, func() {
		if w.events.OnClose != nil {
			w.events.OnClose()
		}
	})
	p.app.Dispatcher().Bind(w.topWin.PlatformID, event.StructureNotifyMask, func(ev *event.Event) {
		switch ev.Type {
		case event.MapType:
			w.mapped = true
			if w.focusPending {
				w.focusPending = false
				w.present()
			}
		case event.UnmapType:
			w.mapped = false
		}
	})
	w.canvas = newCanvas(w)
	w.scrollbar = newScrollbar(w)
	w.layout()
	return w
}

// Events returns the window's callbacks.
func (w *Window) Events() *platform.WindowEvents { return &w.events }

// PixelDensity is the screen's DPI.
func (w *Window) PixelDensity() float64 { return screenunit.DPI() }

// DevicePixelRatio is 1: takigo draws in device pixels.
func (w *Window) DevicePixelRatio() float64 { return 1 }

// DeviceFontScale is the DPI relative to 96.
func (w *Window) DeviceFontScale() float64 {
	return w.PixelDensity() / w.DevicePixelRatio() / 96.0
}

// IsVisible reports whether the window is shown.
func (w *Window) IsVisible() bool { return w.visible }

// SetVisible shows or hides the window.
func (w *Window) SetVisible(v bool) {
	// As GTK's show, nothing for a window already shown: takigo would
	// map it raised again, over the window the user is working in.
	if v && w.visible {
		return
	}
	w.visible = v
	if !v {
		w.tooltip.hide(w.canvas)
	}
	if w.toolTop != nil {
		if v {
			w.toolTop.Show()
		} else {
			w.toolTop.Hide()
		}
		return
	}
	if v {
		w.wmInfo.Deiconify()
	} else {
		w.wmInfo.Withdraw()
	}
}

// Focus raises the window and gives its drawing area the keyboard, as
// GTK's present does. takigo gives a toplevel the X focus only once it
// has had it, so ask for it; a window not mapped yet (the main window
// at startup, shown after the property browser) is asked for once it
// is, or the window manager would focus the one mapped first.
func (w *Window) Focus() {
	w.p.app.FocusManager().SetFocus(w.canvas.Win)
	if !w.mapped {
		w.focusPending = true
		return
	}
	w.present()
}

// present raises the window and asks for the X focus: from the window
// manager through _NET_ACTIVE_WINDOW when there is one (as
// gdk_window_focus does), else directly.
func (w *Window) present() {
	d := w.p.app.Server()
	id := w.topWin.PlatformID
	d.RaiseWindow(id)
	if hasWindowManager(w.topWin) {
		// Source 1: an application's request; timestamp 0: none.
		d.SendClientMessage(id, d.DefaultRootWindow(),
			d.InternAtom("_NET_ACTIVE_WINDOW", false), 1, 0, 0, 0, 0)
	} else {
		d.SetInputFocus(id, tkplatform.RevertToParent, tkplatform.CurrentTime)
	}
	d.Flush()
}

// hasWindowManager reports whether an EWMH window manager runs: the
// root window names its check window.
func hasWindowManager(win *window.Window) bool {
	d := win.Display.Server
	prop := d.InternAtom("_NET_SUPPORTING_WM_CHECK", false)
	data, _, format := d.GetWindowProperty(d.DefaultRootWindow(), prop, 0, 1, false)
	return format == 32 && len(data) >= 4
}

// IsFullScreen reports whether the window was made full screen.
func (w *Window) IsFullScreen() bool { return w.fullScreen }

// SetFullScreen asks the window manager for full screen through
// _NET_WM_STATE, which takigo doesn't wrap.
func (w *Window) SetFullScreen(full bool) {
	if full == w.fullScreen {
		return
	}
	d := w.p.app.Server()
	state := d.InternAtom("_NET_WM_STATE", false)
	fs := d.InternAtom("_NET_WM_STATE_FULLSCREEN", false)
	action := int64(0)
	if full {
		action = 1
	}
	d.SendClientMessage(w.topWin.PlatformID, d.DefaultRootWindow(), state, action, int64(fs), 0, 1, 0)
	d.Flush()
	w.fullScreen = full
	if w.events.OnFullScreen != nil {
		w.events.OnFullScreen(full)
	}
}

// SetTitle sets the window title, with the application's name after
// it as GTK's does.
func (w *Window) SetTitle(title string) { w.wmInfo.SetTitle(prepareTitle(title)) }

// SetTitleForFilename returns false: the title names the file.
func (w *Window) SetTitleForFilename(string) bool { return false }

// SetMenuBar puts a menu bar across the top of the window.
func (w *Window) SetMenuBar(mb platform.MenuBar) {
	bar, _ := mb.(*MenuBar)
	if w.menuBar != nil && w.menuBar != bar {
		w.menuBar.detach()
	}
	w.menuBar = bar
	if bar != nil {
		bar.attach(w)
	}
	w.layout()
}

// ContentSize is the drawing area's size.
func (w *Window) ContentSize() (float64, float64) {
	win := w.canvas.Win
	if win.Width <= 1 || win.Height <= 1 {
		// Not laid out yet: the size it asks for, as GTK reports the
		// default size of a window that isn't mapped yet.
		return float64(win.ReqWidth), float64(win.ReqHeight)
	}
	return float64(win.Width), float64(win.Height)
}

// SetMinContentSize keeps the drawing area at least this big.
func (w *Window) SetMinContentSize(width, height float64) {
	w.canvas.Win.ReqWidth = int(width)
	w.canvas.Win.ReqHeight = int(height)
	w.layout()
}

// FreezePosition saves the window's geometry.
func (w *Window) FreezePosition(s core.Settings, key string) {
	if !w.visible {
		return
	}
	width, height, _, _, _, _, _, _, err := wm.ParseGeometry(w.wmInfo.Geometry())
	if err != nil {
		return
	}
	// takigo does not follow a toplevel's moves, so ask the server where
	// it is, and save the frame's corner as GTK's get_position does.
	d := w.topWin.Display.Server
	x, y := d.TranslateCoordinates(w.topWin.PlatformID, d.DefaultRootWindow(), 0, 0)
	fl, ft := frameOffset(w.topWin)
	x, y = x-fl, y-ft
	s.FreezeInt(key+"_Left", uint32(int32(x)))
	s.FreezeInt(key+"_Top", uint32(int32(y)))
	s.FreezeInt(key+"_Width", uint32(width))
	s.FreezeInt(key+"_Height", uint32(height))
	core.FreezeBool(s, key+"_Maximized", false)
}

// ThawPosition restores the geometry FreezePosition saved. Without a
// saved size the window keeps its natural one, the content it asked
// for, as GTK's get_size reports for a window not yet shown; without a
// saved position it goes to the corner, as GTK's move(0, 0) puts it.
func (w *Window) ThawPosition(s core.Settings, key string) {
	has := func(name string) bool {
		if h, ok := s.(interface{ Has(string) bool }); ok {
			return h.Has(key + name)
		}
		return true
	}
	x := int(int32(s.ThawInt(key+"_Left", 0)))
	y := int(int32(s.ThawInt(key+"_Top", 0)))
	geom := signed(x) + signed(y)
	if has("_Width") && has("_Height") {
		width := int(s.ThawInt(key+"_Width", 0))
		height := int(s.ThawInt(key+"_Height", 0))
		if width <= 0 || height <= 0 {
			return
		}
		geom = strconv.Itoa(width) + "x" + strconv.Itoa(height) + geom
	}
	_ = w.wmInfo.SetGeometry(geom)
	// takigo's packer resizes a toplevel to its content whenever the
	// requested size changes, ignoring a user size as Tk would not, and
	// the first layout runs later; so set the size again once it has.
	w.p.app.DoWhenIdle(func() { _ = w.wmInfo.SetGeometry(geom) })
}

func signed(v int) string {
	if v < 0 {
		return strconv.Itoa(v)
	}
	return "+" + strconv.Itoa(v)
}

// SetCursor sets the drawing area's cursor.
func (w *Window) SetCursor(c platform.Cursor) {
	switch c {
	case platform.CursorHand:
		w.canvas.Win.SetCursor(cursor.Hand2)
	default:
		w.canvas.Win.ResetCursor()
	}
}

// Invalidate schedules OnRender.
func (w *Window) Invalidate() { w.canvas.invalidate() }

// CreateRenderer returns, with useGL, an OpenGL renderer drawing on
// the window, else or when GL fails, the software renderer.
func (w *Window) CreateRenderer(useGL bool) render.ViewportCanvas {
	if useGL {
		r, err := w.canvas.makeGL()
		if err == nil {
			return r
		}
		w.p.glFailed(err)
	}
	w.canvas.dropGL()
	return soft.New()
}

// PresentFrame puts the frame canvas rendered on screen.
func (w *Window) PresentFrame(canvas render.ViewportCanvas) {
	if c := w.canvas; c.glr != nil && canvas == c.glr {
		c.presentGL()
		return
	}
	w.canvas.present(canvas.ReadFrame())
}

// layout packs the drawing area and the scrollbar below the menu bar.
func (w *Window) layout() {
	w.scrollbar.pack()
	w.canvas.pack()
}

// translateButton is process_pointer_event's choice of button.
func translateButton(button, state uint) platform.MouseButton {
	switch {
	case button == 1 || state&tkplatform.Button1Mask != 0:
		return platform.ButtonLeft
	case button == 2 || state&tkplatform.Button2Mask != 0:
		return platform.ButtonMiddle
	case button == 3 || state&tkplatform.Button3Mask != 0:
		return platform.ButtonRight
	}
	return platform.ButtonNone
}

func (w *Window) mouse(typ platform.MouseEventType, ev *event.Event, button uint, delta float64) {
	w.p.pointerX, w.p.pointerY = ev.RootX, ev.RootY
	if w.events.OnMouseEvent == nil {
		return
	}
	w.events.OnMouseEvent(platform.MouseEvent{
		Type:        typ,
		X:           float64(ev.X),
		Y:           float64(ev.Y),
		ShiftDown:   ev.State&tkplatform.ShiftMask != 0,
		ControlDown: ev.State&tkplatform.ControlMask != 0,
		Button:      translateButton(button, ev.State),
		ScrollDelta: delta,
	})
}

// wheel delivers a wheel turn as a vertical scroll of one per notch,
// positive up; a high-resolution wheel's smaller steps give fractions,
// as GTK's smooth scroll deltas do. takigo reports the wheel as its own
// event, never as presses of buttons 4 and 5.
func (w *Window) wheel(ev *event.Event) {
	w.mouse(platform.MouseScrollVert, ev, 0, float64(ev.Delta)/120)
}

// buttonPress delivers a press, or for the second of two quick clicks
// a double press instead, as the GTK backend does.
func (w *Window) buttonPress(ev *event.Event) {
	now := time.Now()
	lp := &w.lastPress
	if lp.button == ev.Button && now.Sub(lp.time) < doubleClickTime &&
		abs(ev.X-lp.x) <= doubleClickDistance && abs(ev.Y-lp.y) <= doubleClickDistance {
		// Reset, so a third click is a press again.
		*lp = struct {
			button uint
			time   time.Time
			x, y   int
		}{}
		w.mouse(platform.MouseDblPress, ev, ev.Button, 0)
		return
	}
	lp.button, lp.time, lp.x, lp.y = ev.Button, now, ev.X, ev.Y
	w.mouse(platform.MousePress, ev, ev.Button, 0)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// key delivers a key as the GTK backend does: a character (lowercase,
// with Shift reported separately) or a function key; keys with other
// modifiers than Shift and Control are left alone.
func (w *Window) key(typ platform.KeyboardEventType, ev *event.Event) bool {
	// GTK's window takes its menu bar's keys before its widgets see them.
	if typ == platform.KeyPress && w.menuBar != nil && w.menuBar.keyPress(ev) {
		return true
	}
	if ev.State&(tkplatform.Mod1Mask|tkplatform.Mod4Mask) != 0 {
		return false
	}
	k := platform.KeyboardEvent{
		Type:        typ,
		ShiftDown:   ev.State&tkplatform.ShiftMask != 0,
		ControlDown: ev.State&tkplatform.ControlMask != 0,
	}
	if chr := keySymToUnicode(ev.KeySym); chr != 0 {
		k.Key = platform.KeyCharacter
		k.Chr = chr
	} else if ev.KeySym >= xkF1 && ev.KeySym <= xkF12 {
		k.Key = platform.KeyFunction
		k.Num = int(ev.KeySym-xkF1) + 1
	} else {
		return false
	}
	if w.events.OnKeyboardEvent == nil {
		return false
	}
	if w.events.OnKeyboardEvent(k) {
		return true
	}
	// GTK matches a menu accelerator on the key pressed, with Shift
	// left out when it made the symbol: Shift and '=' is the '+' of
	// Zoom In, Shift and '.' the '>' of Show Snap Grid, and Shift and
	// '3' is Sketch In 3d's Shift+3 although X names it '#'. The
	// application sees only the symbol, so offer it those readings too.
	if k.Key == platform.KeyCharacter && k.ShiftDown && typ == platform.KeyPress &&
		!unicode.IsLetter(k.Chr) && !unicode.IsDigit(k.Chr) && k.Chr > ' ' {
		unshifted := k
		unshifted.ShiftDown = false
		if w.events.OnKeyboardEvent(unshifted) {
			return true
		}
		if digit, ok := digitRowKey(ev.KeyCode); ok {
			asDigit := k
			asDigit.Chr = digit
			return w.events.OnKeyboardEvent(asDigit)
		}
	}
	return false
}

// digitRowKey is the digit of a key of the keyboard's top row by its X
// keycode, which on Linux (XKB over evdev) is 10 for '1' through 19
// for '0' whatever the layout's symbols.
func digitRowKey(keycode uint) (rune, bool) {
	if keycode < 10 || keycode > 19 {
		return 0, false
	}
	if keycode == 19 {
		return '0', true
	}
	return rune('1' + keycode - 10), true
}

const (
	xkF1  = tkplatform.KeySym(0xffbe)
	xkF10 = tkplatform.KeySym(0xffc7)
	xkF12 = tkplatform.KeySym(0xffc9)
)

// keySymToUnicode is gdk_keyval_to_unicode(gdk_keyval_to_lower(k)).
func keySymToUnicode(k tkplatform.KeySym) rune {
	switch {
	case k >= 'A' && k <= 'Z':
		k += 'a' - 'A'
	case k >= 0xc0 && k <= 0xde && k != 0xd7:
		// Latin-1 capitals.
		k += 0x20
	}
	switch k {
	case 0xff08: // BackSpace
		return '\b'
	case 0xff09, 0xfe20: // Tab, ISO_Left_Tab
		return '\t'
	case 0xff0a: // Linefeed
		return '\n'
	case 0xff0d, 0xff8d: // Return, KP_Enter
		return '\r'
	case 0xff1b: // Escape
		return '\x1b'
	case 0xffff, 0xff9f: // Delete, KP_Delete
		return '\x7f'
	case 0xff80: // KP_Space
		return ' '
	case 0xffaa:
		return '*'
	case 0xffab:
		return '+'
	case 0xffac:
		return ','
	case 0xffad:
		return '-'
	case 0xffae:
		return '.'
	case 0xffaf:
		return '/'
	case 0xffbd:
		return '='
	}
	if k >= 0xffb0 && k <= 0xffb9 {
		return rune('0' + k - 0xffb0)
	}
	if k >= 0xff00 && k < 0x1000000 && k>>8 == 0xff {
		// Other function and modifier keys.
		return 0
	}
	r := tkplatform.KeySymToRune(k)
	if r < 0 {
		return 0
	}
	return r
}

// setWindowIcon gives a window the SolveSpace icon; takigo has no
// iconphoto, so this sets _NET_WM_ICON (cardinals: width, height, then
// ARGB pixels) directly.
func setWindowIcon(win *window.Window) {
	icon := res.LoadPng("freedesktop/telesapr-48x48.png")
	bpp := icon.BytesPerPixel()
	vals := make([]uint32, 0, 2+icon.Width*icon.Height)
	vals = append(vals, uint32(icon.Width), uint32(icon.Height))
	for y := range icon.Height {
		for x := range icon.Width {
			px := icon.Data[y*icon.Stride+x*bpp:]
			a := uint32(0xFF)
			if icon.Format == res.FormatRGBA {
				a = uint32(px[3])
			}
			vals = append(vals, a<<24|uint32(px[0])<<16|uint32(px[1])<<8|uint32(px[2]))
		}
	}
	d := win.Display.Server
	d.ChangeProperty(win.PlatformID, d.InternAtom("_NET_WM_ICON", false),
		d.Atoms().Cardinal, 32, tkplatform.PropModeReplace, cardinals(vals), len(vals))
}

// cardinals packs format-32 property data as takigo takes it: four
// bytes per item in native byte order.
func cardinals(vals []uint32) []byte {
	data := make([]byte, len(vals)*4)
	for i, v := range vals {
		binary.NativeEndian.PutUint32(data[i*4:], v)
	}
	return data
}

// frameOffset is how far the window manager's frame reaches left of
// and above the window (_NET_FRAME_EXTENTS: left, right, top, bottom),
// zero without one.
func frameOffset(win *window.Window) (left, top int) {
	d := win.Display.Server
	prop := d.InternAtom("_NET_FRAME_EXTENTS", false)
	data, _, format := d.GetWindowProperty(win.PlatformID, prop, 0, 4, false)
	if format != 32 || len(data) < 16 {
		return 0, 0
	}
	left = int(int32(binary.NativeEndian.Uint32(data[0:])))
	top = int(int32(binary.NativeEndian.Uint32(data[8:])))
	return left, top
}
