package tkgui

import (
	"image"
	"unicode"

	"github.com/takigo/takigo/event"
	tkplatform "github.com/takigo/takigo/platform"
)

// What the user's guide needs to picture the real window: menus, popup
// menus and dialogs are drawn by the X server's toplevels, which the
// headless platform has none of. cmd/telesapr's TestDocWindowShots
// drives the window with these under Xvfb and reads the screen back.

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

// Click moves the pointer to x, y in the drawing area, so that what is
// under it is hovered, then presses and releases button there, with the
// pointer's place on the screen set as the X server would, so that a
// popup menu opens there.
func (w *Window) Click(x, y int, button uint) {
	win := w.canvas.Window()
	d := win.Display
	rx, ry := d.Server.TranslateCoordinates(win.PlatformID, d.RootWindow, x, y)
	disp := w.p.app.Dispatcher()
	disp.Dispatch(&event.Event{Type: event.MotionType, Window: win.PlatformID,
		X: x, Y: y, RootX: rx, RootY: ry})
	for _, typ := range []event.Type{event.ButtonPressType, event.ButtonReleaseType} {
		disp.Dispatch(&event.Event{Type: typ, Window: win.PlatformID,
			X: x, Y: y, RootX: rx, RootY: ry, Button: button})
	}
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
