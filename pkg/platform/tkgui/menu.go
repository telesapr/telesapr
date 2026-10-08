package tkgui

import (
	"strings"
	"time"
	"unicode"

	"github.com/takigo/takigo/bind"
	"github.com/takigo/takigo/event"
	tkplatform "github.com/takigo/takigo/platform"
	"github.com/takigo/takigo/widget"
	"github.com/takigo/takigo/widget/menu"
	"github.com/takigo/takigo/window"

	"github.com/telesapr/telesapr/pkg/platform"
)

// prepareMnemonics drops the '&' that marks a label's mnemonic and
// returns where it was, for Tk's -underline.
func prepareMnemonics(label string) (string, int) {
	before, after, ok := strings.Cut(label, "&")
	if !ok {
		return label, -1
	}
	return before + after, len([]rune(before))
}

// Menu is a Tk menu. Tk menus can't lose entries, so Clear replaces the
// menu in the entry it hangs from.
type Menu struct {
	p     *Platform
	m     *menu.Menu
	items []*MenuItem
	// The menu entry this menu is the cascade of, if any.
	parent      *Menu
	parentIndex int
}

// MenuItem is an entry of a Menu, addressed by its index there.
type MenuItem struct {
	menu      *Menu
	index     int
	onTrigger func()
}

func (p *Platform) newMenu() *Menu {
	m := &Menu{p: p}
	m.m = menu.New(p.app, p.newName("menu"), menu.TearOffOpt(false))
	return m
}

// CreateMenu makes a popup menu.
func (p *Platform) CreateMenu() platform.Menu { return p.newMenu() }

func (it *MenuItem) entry() *menu.MenuEntry {
	es := it.menu.m.Entries()
	if it.index >= len(es) {
		return nil
	}
	return &es[it.index]
}

// SetAccelerator shows the accelerator; the application handles the
// key itself (GraphicsWindow::KeyboardEvent).
func (it *MenuItem) SetAccelerator(accel platform.KeyboardEvent) {
	if e := it.entry(); e != nil {
		e.AccelStr = platform.AcceleratorDescription(accel)
	}
}

// SetIndicator makes the entry a check or radio button, or neither.
func (it *MenuItem) SetIndicator(ind platform.Indicator) {
	e := it.entry()
	if e == nil {
		return
	}
	switch ind {
	case platform.IndicatorCheckMark:
		e.Type = menu.Checkbutton
	case platform.IndicatorRadioMark:
		e.Type = menu.Radiobutton
	default:
		e.Type = menu.Command
	}
}

// SetEnabled enables or greys out the entry.
func (it *MenuItem) SetEnabled(enabled bool) {
	if e := it.entry(); e != nil {
		if enabled {
			e.State = widget.StateNormal
		} else {
			e.State = widget.StateDisabled
		}
	}
}

// SetActive checks or unchecks the entry's indicator, without
// triggering it.
func (it *MenuItem) SetActive(active bool) {
	if e := it.entry(); e != nil {
		e.Checked = active
	}
}

// AddItem adds a command.
func (m *Menu) AddItem(label string, onTrigger func(), mnemonics bool) platform.MenuItem {
	it := &MenuItem{menu: m, index: len(m.m.Entries()), onTrigger: onTrigger}
	underline := -1
	if mnemonics {
		label, underline = prepareMnemonics(label)
	}
	m.m.AddCommandAccelUL(label, "", underline, func() {
		if it.onTrigger != nil {
			it.onTrigger()
		}
	})
	m.items = append(m.items, it)
	return it
}

// AddSubMenu adds a cascade.
func (m *Menu) AddSubMenu(label string) platform.Menu {
	sub := m.p.newMenu()
	sub.parent, sub.parentIndex = m, len(m.m.Entries())
	label, underline := prepareMnemonics(label)
	m.m.AddCascadeUL(label, underline, sub.m)
	m.items = append(m.items, nil)
	return sub
}

// AddSeparator adds a separator.
func (m *Menu) AddSeparator() {
	m.m.AddSeparator()
	m.items = append(m.items, nil)
}

// PopUp posts the menu at the pointer and returns once it is
// dismissed, having run the chosen entry's trigger.
func (m *Menu) PopUp() {
	app := m.p.app
	m.m.PostFromButton(m.p.pointerX, m.p.pointerY)
	done := make(chan struct{})
	var check func()
	check = func() {
		if !m.m.IsPosted() {
			close(done)
			return
		}
		app.After(20*time.Millisecond, check)
	}
	app.After(20*time.Millisecond, check)
	app.RunNestedLoop(done)
}

// Clear empties the menu by replacing it with a new one in the entry
// it hangs from.
func (m *Menu) Clear() {
	old := m.m
	m.m = menu.New(m.p.app, m.p.newName("menu"), menu.TearOffOpt(false))
	m.items = nil
	if m.parent != nil {
		if es := m.parent.m.Entries(); m.parentIndex < len(es) {
			es[m.parentIndex].SubMenu = m.m
		}
	}
	old.Destroy()
}

// MenuBar is a window's menu bar. Until it is given to a window it only
// records its menus, since a Tk menubar belongs to its toplevel.
type MenuBar struct {
	p     *Platform
	menus []barMenu
	bar   *menu.Menubar
	w     *Window
	// keyPosted is the menu postByKey opened, until its first entry is
	// made active.
	keyPosted *menu.Menu
}

type barMenu struct {
	label     string
	underline int
	menu      *Menu
}

// MainMenu returns the main window's menu bar; each window
// has its own, so unique is false.
func (p *Platform) MainMenu() (platform.MenuBar, bool) {
	if p.mainBar == nil {
		p.mainBar = &MenuBar{p: p}
	}
	return p.mainBar, false
}

// AddSubMenu adds a menu to the bar.
func (b *MenuBar) AddSubMenu(label string) platform.Menu {
	sub := b.p.newMenu()
	label, underline := prepareMnemonics(label)
	b.menus = append(b.menus, barMenu{label, underline, sub})
	if b.bar != nil {
		b.bar.AddCascade(label, underline, sub.m)
	}
	b.bindTraversal(sub)
	return sub
}

// keyPress opens a menu from the keyboard as GTK's menu bar does: Alt
// and a menu's mnemonic opens it, F10 the first one (Tk's
// tk::TraverseToMenu and tk::FirstMenu, which takigo's menubar lacks).
func (b *MenuBar) keyPress(ev *event.Event) bool {
	if b.bar == nil || len(b.menus) == 0 {
		return false
	}
	mods := ev.State & (tkplatform.ShiftMask | tkplatform.ControlMask |
		tkplatform.Mod1Mask | tkplatform.Mod4Mask)
	if ev.KeySym == xkF10 && mods == 0 {
		b.postByKey(0)
		return true
	}
	if mods&^tkplatform.ShiftMask != tkplatform.Mod1Mask {
		return false
	}
	r := unicode.ToLower(tkplatform.KeySymToRune(ev.KeySym))
	if r <= 0 {
		return false
	}
	for i, m := range b.menus {
		if runes := []rune(m.label); m.underline >= 0 && m.underline < len(runes) &&
			unicode.ToLower(runes[m.underline]) == r {
			b.postByKey(i)
			return true
		}
	}
	return false
}

// bindTraversal makes Left and Right in a menu of the bar open the
// menu next to it, as Tk's tk::MenuLeftArrow and tk::MenuRightArrow
// do. Left is bound on the menu's path, which comes before takigo's
// class binding, whose closing of the menu it replaces: the next menu
// is then posted while this one still holds the focus, as a click on
// the menubar does. Right runs after the class binding, and does
// nothing when that opened a cascade of the menu.
func (b *MenuBar) bindTraversal(m *Menu) {
	next := func(dir int) {
		for i, bm := range b.menus {
			if bm.menu == m {
				n := len(b.menus)
				b.postByKey((i + dir + n) % n)
				return
			}
		}
	}
	win := m.m.Window()
	_ = b.p.app.Bind().Bind(win.PathName, "<KeyPress-Left>", func(*bind.EventData) bool {
		if b.bar == nil {
			return false
		}
		next(-1)
		return true
	})
	// After takigo's own Enter, which activates the entry under the
	// pointer, or none.
	b.p.app.Dispatcher().Bind(win.PlatformID, event.EnterMask, func(ev *event.Event) {
		if ev.Type == event.EnterType && b.keyPosted == m.m {
			b.activateFirst()
		}
	})
	b.p.app.Dispatcher().Bind(win.PlatformID, event.KeyPressMask, func(ev *event.Event) {
		if ev.Type != event.KeyPressType || ev.KeySym != tkplatform.XK_Right || b.bar == nil {
			return
		}
		for _, e := range m.m.Entries() {
			if e.SubMenu != nil && e.SubMenu.IsPosted() {
				return
			}
		}
		next(1)
	})
}

// postByKey opens menu i of the bar with its first entry active. The
// menubar posts its menus only for a click, so it is given one on the
// menu's label; Down then activates the first entry (Tk's
// tk::MenuFirst).
//
// Down waits for the Enter that the menu's pointer grab sends it, since
// takigo's Enter activates the entry under the pointer, or none. The
// events are read on another goroutine, so on a busy machine that Enter
// can come after the loop has gone idle. When the pointer is already in
// the menu no Enter comes, and a timer sends Down instead.
func (b *MenuBar) postByKey(i int) {
	x, y, ok := b.entryCenter(i)
	if !ok {
		return
	}
	win := b.bar.Window()
	rx, ry := win.Display.Server.TranslateCoordinates(win.PlatformID, win.Display.RootWindow, x, y)
	b.p.app.Dispatcher().Dispatch(&event.Event{Type: event.ButtonPressType, Window: win.PlatformID,
		X: x, Y: y, RootX: rx, RootY: ry, Button: 1})
	sub := b.menus[i].menu.m
	b.keyPosted = sub
	b.p.app.After(keyPostTimeout, func() {
		if b.keyPosted == sub {
			b.activateFirst()
		}
	})
}

// keyPostTimeout is how long postByKey waits for the grab's Enter.
const keyPostTimeout = 300 * time.Millisecond

// activateFirst activates the first entry of the menu postByKey opened.
func (b *MenuBar) activateFirst() {
	sub := b.keyPosted
	b.keyPosted = nil
	if sub.IsPosted() {
		b.p.app.Dispatcher().Dispatch(&event.Event{Type: event.KeyPressType, Window: sub.Window().PlatformID,
			KeySym: tkplatform.XK_Down})
	}
}

// entryCenter is the middle of the label of menu i in the menubar,
// laid out as takigo's Menubar.geometry (TkpComputeMenubarGeometry)
// does, which it keeps to itself: entries of equal height in rows as
// wide as the window.
func (b *MenuBar) entryCenter(i int) (x, y int, ok bool) {
	mb := b.bar
	if mb == nil || mb.Font == nil || i < 0 || i >= len(b.menus) {
		return 0, 0, false
	}
	const abw = 1 // the menubar's active border width
	bw := mb.BorderWidth
	maxWidth := b.w.topWin.Width
	if maxWidth <= 1 {
		maxWidth = 0x7FFFFFF
	}
	height := mb.Font.Metrics().Linespace() + 1 + 2*abw + 10
	ex, ey := bw, bw
	for j, m := range b.menus {
		width := mb.Font.MeasureString(m.label) + bw + 2*abw + 10
		if ex+width+bw > maxWidth && ex > bw {
			ex, ey = bw, ey+height
		}
		if j == i {
			return ex + width/2, ey + height/2, true
		}
		ex += width
	}
	return 0, 0, false
}

// Clear removes the menus: the Tk menubar is made again.
func (b *MenuBar) Clear() {
	for _, m := range b.menus {
		m.menu.m.Destroy()
	}
	b.menus = nil
	if b.w != nil {
		w := b.w
		b.detach()
		b.attach(w)
	}
}

func (b *MenuBar) attach(w *Window) {
	b.w = w
	b.bar = menu.NewMenubar(w.top, b.p.newName("menubar"))
	for _, m := range b.menus {
		b.bar.AddCascade(m.label, m.underline, m.menu.m)
	}
}

func (b *MenuBar) detach() {
	if b.bar != nil {
		b.w.topWin.Menubar = nil
		window.DestroyWindow(b.bar.Window())
		b.bar = nil
	}
	b.w = nil
}
