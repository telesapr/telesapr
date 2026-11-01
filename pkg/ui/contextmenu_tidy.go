package ui

import (
	"github.com/telesapr/telesapr/pkg/platform"
)

// tidyMenu sets the context menu's groups of entries apart without
// doubling the line between them: the groups are added by different
// code, each starting with a separator, and some come out empty. A
// separator is kept until an entry follows it, and only between
// entries.
type tidyMenu struct {
	platform.Menu
	entries    int
	sepPending bool
}

func (m *tidyMenu) flush() {
	if m.sepPending && m.entries > 0 {
		m.Menu.AddSeparator()
	}
	m.sepPending = false
	m.entries++
}

// AddItem adds an entry, after the separator asked for before it.
func (m *tidyMenu) AddItem(label string, onTrigger func(), mnemonics bool) platform.MenuItem {
	m.flush()
	return m.Menu.AddItem(label, onTrigger, mnemonics)
}

// AddSubMenu adds a submenu, after the separator asked for before it.
func (m *tidyMenu) AddSubMenu(label string) platform.Menu {
	m.flush()
	return m.Menu.AddSubMenu(label)
}

// AddSeparator asks for a separator before the next entry.
func (m *tidyMenu) AddSeparator() { m.sepPending = true }

// sketchEntitiesSelected counts the selected entities a request draws,
// which a style and construction apply to; a face or an edge a group
// generates has neither.
func (gw *graphicsWindow) sketchEntitiesSelected(gs *groupedSelection) (stylable, all int) {
	sk := gw.sk()
	for _, he := range gs.entity {
		if !he.IsFromRequest() {
			continue
		}
		all++
		if e := sk.Entity(he); e != nil && e.IsStylable() {
			stylable++
		}
	}
	for _, hp := range gs.point {
		if hp.IsFromRequest() {
			all++
		}
	}
	return stylable, all
}

// constraintsStylable counts the selected constraints a style applies
// to.
func (gw *graphicsWindow) constraintsStylable(gs *groupedSelection) int {
	n := 0
	for _, hc := range gs.constraint {
		if c := gw.sk().Constraint(hc); c != nil && c.IsStylable() {
			n++
		}
	}
	return n
}
