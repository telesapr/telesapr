package tkgui

import (
	"strings"
	"time"

	"github.com/msorc/takigo/widget"
	"github.com/msorc/takigo/widget/menu"
	"github.com/msorc/takigo/window"

	"github.com/spacesolve/spacesolve/pkg/platform"
)

// prepareMnemonics drops the '&' that marks a label's mnemonic and
// returns where it was, for Tk's -underline.
func prepareMnemonics(label string) (string, int) {
	i := strings.IndexByte(label, '&')
	if i < 0 {
		return label, -1
	}
	return label[:i] + label[i+1:], len([]rune(label[:i]))
}

// Menu is a Tk menu (C++ MenuImplGtk). Tk menus can't lose entries, so
// Clear replaces the menu in the entry it hangs from.
type Menu struct {
	p     *Platform
	m     *menu.Menu
	items []*MenuItem
	// The menu entry this menu is the cascade of, if any.
	parent      *Menu
	parentIndex int
}

// MenuItem is an entry of a Menu, addressed by its index there (C++
// MenuItemImplGtk).
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

// MenuBar is a window's menu bar (C++ MenuBarImplGtk). Until it is
// given to a window it only records its menus, since a Tk menubar
// belongs to its toplevel.
type MenuBar struct {
	p     *Platform
	menus []barMenu
	bar   *menu.Menubar
	w     *Window
}

type barMenu struct {
	label     string
	underline int
	menu      *Menu
}

// GetOrCreateMainMenu returns the main window's menu bar; each window
// has its own, so unique is false.
func (p *Platform) GetOrCreateMainMenu() (platform.MenuBar, bool) {
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
	return sub
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
