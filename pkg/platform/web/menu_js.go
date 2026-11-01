//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/telesapr/telesapr/pkg/platform"
)

// Menu is a menu in the page: a list of items that a bar shows under a
// label, an item shows beside it, or PopUp shows at the pointer. The
// page draws it; the menu keeps the application's side.
type Menu struct {
	p  *Platform
	js js.Value
	// funcs are the page's triggers of the menu's items, released with
	// the items; subs are the submenus, whose triggers go with them.
	funcs []js.Func
	subs  []*Menu
}

var _ platform.Menu = (*Menu)(nil)

// MenuItem is a menu entry.
type MenuItem struct {
	js        js.Value
	accel     string
	indicator platform.Indicator
	enabled   bool
	active    bool
}

var _ platform.MenuItem = (*MenuItem)(nil)

// CreateMenu makes a popup menu.
func (p *Platform) CreateMenu() platform.Menu {
	return &Menu{p: p, js: p.w.Call("menu")}
}

// AddItem adds a command. A '&' in the label marks its mnemonic when
// mnemonics is set.
func (m *Menu) AddItem(label string, onTrigger func(), mnemonics bool) platform.MenuItem {
	trigger := js.FuncOf(func(js.Value, []js.Value) any {
		m.p.q.post(func() {
			if onTrigger != nil {
				onTrigger()
			}
		})
		return nil
	})
	m.funcs = append(m.funcs, trigger)
	return &MenuItem{js: m.js.Call("addItem", label, mnemonics, trigger), enabled: true}
}

// AddSubMenu adds a cascade.
func (m *Menu) AddSubMenu(label string) platform.Menu {
	sub := &Menu{p: m.p, js: m.js.Call("addSub", label)}
	m.subs = append(m.subs, sub)
	return sub
}

// AddSeparator adds a separator.
func (m *Menu) AddSeparator() { m.js.Call("addSeparator") }

// Clear drops the menu's items.
func (m *Menu) Clear() {
	m.js.Call("clear")
	m.release()
}

// release frees the functions given to the page for the items.
func (m *Menu) release() {
	for _, f := range m.funcs {
		f.Release()
	}
	m.funcs = nil
	for _, s := range m.subs {
		s.release()
	}
	m.subs = nil
}

// PopUp shows the menu at the pointer and runs the event queue until it
// is dismissed, having queued the chosen item's trigger first, so that
// it runs before PopUp returns.
func (m *Menu) PopUp() {
	done := js.Func{}
	m.p.runModal(func(end func(func())) {
		done = js.FuncOf(func(js.Value, []js.Value) any {
			end(nil)
			return nil
		})
		m.js.Call("popup", m.p.pointerX, m.p.pointerY, done)
	})
	done.Release()
}

// SetAccelerator shows the accelerator; the application takes the key
// itself.
func (it *MenuItem) SetAccelerator(accel platform.KeyboardEvent) {
	s := platform.AcceleratorDescription(accel)
	if s != it.accel {
		it.accel = s
		it.js.Call("setAccel", s)
	}
}

// SetIndicator makes the entry a check or radio item, or neither.
func (it *MenuItem) SetIndicator(ind platform.Indicator) {
	if ind != it.indicator {
		it.indicator = ind
		it.js.Call("setIndicator", int(ind))
	}
}

// SetEnabled enables or greys out the entry.
func (it *MenuItem) SetEnabled(enabled bool) {
	if enabled != it.enabled {
		it.enabled = enabled
		it.js.Call("setEnabled", enabled)
	}
}

// SetActive marks or unmarks the entry's indicator, without triggering
// it.
func (it *MenuItem) SetActive(active bool) {
	if active != it.active {
		it.active = active
		it.js.Call("setActive", active)
	}
}

// MenuBar is the page's menu bar.
type MenuBar struct {
	p     *Platform
	js    js.Value
	menus []*Menu
}

var _ platform.MenuBar = (*MenuBar)(nil)

// MainMenu returns the menu bar, which is across the top of the page.
func (p *Platform) MainMenu() (platform.MenuBar, bool) {
	if p.bar == nil {
		p.bar = &MenuBar{p: p, js: p.w.Call("menuBar")}
	}
	return p.bar, true
}

// AddSubMenu adds a menu to the bar.
func (b *MenuBar) AddSubMenu(label string) platform.Menu {
	m := &Menu{p: b.p, js: b.js.Call("addSub", label)}
	b.menus = append(b.menus, m)
	return m
}

// Clear removes the menus.
func (b *MenuBar) Clear() {
	b.js.Call("clear")
	for _, m := range b.menus {
		m.release()
	}
	b.menus = nil
}
