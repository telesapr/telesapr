package tkgui

import (
	"image"
	"slices"
	"unicode"

	"github.com/takigo/takigo/event"
	tkplatform "github.com/takigo/takigo/platform"
	"github.com/takigo/takigo/widget/menu"
	"github.com/takigo/takigo/window"
)

// What the user's guide needs to picture the real window: menus, popup
// menus and dialogs are drawn by the X server's toplevels, which the
// headless platform has none of. pkg/ui's TestDocWindowShots and
// TestDocWindowVideos drive the window with these under Xvfb and read
// the screen back.

// ScreenBounds is the part of the screen the mapped toplevels cover:
// the windows, and the menus and dialogs open over them.
func (p *Platform) ScreenBounds() image.Rectangle {
	d := p.app.Display()
	var r image.Rectangle
	for _, w := range d.Windows {
		if !w.IsTopLevel() || !w.IsMapped() || w.IsDestroyed() || w.PlatformID == 0 {
			continue
		}
		x, y := d.Server.TranslateCoordinates(w.PlatformID, d.RootWindow, 0, 0)
		r = r.Union(image.Rect(x, y, x+w.Width, y+w.Height))
	}
	sw, sh := d.Server.ScreenWidth(d.Screen), d.Server.ScreenHeight(d.Screen)
	return r.Intersect(image.Rect(0, 0, sw, sh))
}

// ReadScreen is the screen's pixels in r, opaque, or nil where the
// display can't read them back.
func (p *Platform) ReadScreen(r image.Rectangle) *image.RGBA {
	d := p.app.Display()
	if r.Empty() {
		return nil
	}
	data := d.Server.GetImageRGBA(tkplatform.DrawableID(d.RootWindow), r.Min.X, r.Min.Y, r.Dx(), r.Dy())
	if len(data) < 4*r.Dx()*r.Dy() {
		return nil
	}
	return &image.RGBA{Pix: data, Stride: 4 * r.Dx(), Rect: image.Rect(0, 0, r.Dx(), r.Dy())}
}

// OpenMenu opens menu i of the bar as Alt and its mnemonic does.
func (b *MenuBar) OpenMenu(i int) { b.postByKey(i) }

// OpenSubMenu opens the cascade labelled label (its mnemonic's & left
// out) of menu i of the bar, once OpenMenu(i) has opened that menu: the
// entry's mnemonic is typed in the menu, which posts the cascade as
// Alt, the menu's mnemonic and the entry's do. It does nothing when the
// menu is not open or has no such cascade with a mnemonic.
func (b *MenuBar) OpenSubMenu(i int, label string) {
	if i < 0 || i >= len(b.menus) {
		return
	}
	m := b.menus[i].menu.m
	if !m.IsPosted() {
		return
	}
	for _, e := range m.Entries() {
		runes := []rune(e.Label)
		if e.SubMenu == nil || e.Label != label || e.Underline < 0 || e.Underline >= len(runes) {
			continue
		}
		b.p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType, Window: m.Window().PlatformID,
			KeySym: tkplatform.RuneToKeySym(unicode.ToLower(runes[e.Underline]))})
		return
	}
}

// ChooseInSubMenu types the mnemonic r in the cascade labelled label of
// menu i, once OpenSubMenu has opened it, which chooses the entry with
// that mnemonic as the key does. It does nothing when the cascade is
// not open.
func (b *MenuBar) ChooseInSubMenu(i int, label string, r rune) {
	if i < 0 || i >= len(b.menus) {
		return
	}
	for _, e := range b.menus[i].menu.m.Entries() {
		if e.SubMenu == nil || e.Label != label || !e.SubMenu.IsPosted() {
			continue
		}
		b.p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType, Window: e.SubMenu.Window().PlatformID,
			KeySym: tkplatform.RuneToKeySym(unicode.ToLower(r))})
		return
	}
}

// Spot is a place on one of the application's windows that the pointer
// can be put on and pressed at: a point of the drawing area, an entry
// of a posted menu or a widget of a form. The events go to the window's
// own handlers, as the X server's do; the X pointer stays where it is.
type Spot struct {
	p    *Platform
	win  *window.Window
	x, y int
}

// Screen is where the spot lies on the screen.
func (s Spot) Screen() image.Point {
	d := s.win.Display
	x, y := d.Server.TranslateCoordinates(s.win.PlatformID, d.RootWindow, s.x, s.y)
	return image.Pt(x, y)
}

func (s Spot) send(typ event.Type, button uint) {
	at := s.Screen()
	s.p.app.Dispatcher().Dispatch(&event.Event{Type: typ, Window: s.win.PlatformID,
		X: s.x, Y: s.y, RootX: at.X, RootY: at.Y, Button: button})
}

// Hover moves the pointer onto the spot, so that what is under it
// highlights.
func (s Spot) Hover() {
	s.send(event.EnterType, 0)
	s.send(event.MotionType, 0)
}

// Leave takes the pointer off the spot's widget, which stops
// highlighting. It does nothing to a menu, which keeps its entry active
// while the pointer is in its cascade, nor to a widget that is gone or
// the zero Spot.
func (s Spot) Leave() {
	if s.win == nil || s.win.IsDestroyed() || s.win.Class == "Menu" {
		return
	}
	s.send(event.LeaveType, 0)
}

// Press presses and releases button at the spot. What that opens, a
// popup menu or a modal dialog, has a loop of its own, so Press returns
// only once it is closed.
func (s Spot) Press(button uint) {
	s.send(event.ButtonPressType, button)
	// A press that closed the widget's window leaves nothing to release
	// on.
	if !s.win.IsDestroyed() {
		s.send(event.ButtonReleaseType, button)
	}
}

// Spot is the point x, y of the drawing area.
func (w *Window) Spot(x, y int) Spot {
	return Spot{p: w.p, win: w.canvas.Window(), x: x, y: y}
}

// Click moves the pointer to x, y in the drawing area, so that what is
// under it is hovered, then presses and releases button there, with the
// pointer's place on the screen set as the X server would, so that a
// popup menu opens there.
func (w *Window) Click(x, y int, button uint) {
	s := w.Spot(x, y)
	s.send(event.MotionType, 0)
	s.Press(button)
}

// EntrySpot is the entry labelled label (its mnemonic's & left out) of
// the menu; false while the menu is not posted or has no such entry.
// The entries are laid out as takigo's Menu does in one column with no
// tearoff, which it keeps to itself.
func (m *Menu) EntrySpot(label string) (Spot, bool) {
	tm := m.m
	if !tm.IsPosted() || tm.Font == nil {
		return Spot{}, false
	}
	const sepHeight = 6
	entryHeight := tm.Font.Metrics().Linespace() + 6
	y := tm.BorderWidth
	for _, e := range tm.Entries() {
		if e.Type == menu.Separator {
			y += sepHeight
			continue
		}
		if e.Label == label {
			// Right of most labels and low in the entry, so that the
			// pointer drawn there hides little of the text.
			win := tm.Window()
			return Spot{p: m.p, win: win, x: win.Width * 3 / 4, y: y + entryHeight*2/3}, true
		}
		y += entryHeight
	}
	return Spot{}, false
}

// FormSpot is a widget of the open form dialog (FormDialog), named by
// the path of window names to it: "row0", "c2" is the third choice of
// the first field, "row4", "check" the box of the fifth, "b0" the
// accepting button and "b1" the other. False while no form is open or
// it has no such widget.
func (p *Platform) FormSpot(names ...string) (Spot, bool) {
	for _, top := range p.app.Display().Windows {
		if !top.IsTopLevel() || !top.IsMapped() || top.IsDestroyed() || descendant(top, "form") == nil {
			continue
		}
		w := top
		for _, name := range names {
			if w = descendant(w, name); w == nil {
				return Spot{}, false
			}
		}
		return Spot{p: p, win: w, x: w.Width * 3 / 4, y: w.Height * 2 / 3}, true
	}
	return Spot{}, false
}

// descendant is the window named name nearest below w, or nil. Other
// toplevels, whose parent w may be, are not looked in.
func descendant(w *window.Window, name string) *window.Window {
	for queue := slices.Clone(w.Children); len(queue) > 0; queue = queue[1:] {
		c := queue[0]
		if c.IsTopLevel() {
			continue
		}
		if c.Name == name {
			return c
		}
		queue = append(queue, c.Children...)
	}
	return nil
}

// CloseMenus closes whatever menus are open, as Escape in one does.
func (p *Platform) CloseMenus() {
	for _, w := range p.app.Display().Windows {
		if w.IsTopLevel() && w.IsMapped() && w.Class == "Menu" {
			p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType, Window: w.PlatformID,
				KeySym: tkplatform.XK_Escape})
		}
	}
}

// CloseDialogs cancels whatever dialogs are open, as Escape in one
// does. The escape goes to every toplevel but the menus and the main
// window, so the application's other windows get one too.
func (p *Platform) CloseDialogs() {
	root := p.app.Window()
	for _, w := range p.app.Display().Windows {
		if w.IsTopLevel() && w.IsMapped() && w != root && w.Class != "Menu" {
			p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType, Window: w.PlatformID,
				KeySym: tkplatform.XK_Escape})
		}
	}
}
