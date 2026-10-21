package ui

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// The datums screen (Edit > Datums) lists the names given to entities
// the model offers others, and those of the models it links; it names
// the selected entity, renames and deletes, and selects what a name
// names, for a mate. The window's side of doc.Datums.

// MenuDatums shows the datums, from the menu. The selection is kept:
// the screen names it.
func (ss *Application) MenuDatums() {
	ss.TW.GoToScreen(screenDatums)
	ss.GW.ForceTextWindowShown()
	ss.ScheduleShowTW()
}

// selectedForDatum is the one selected entity a datum can name, if one
// is selected.
func (ss *Application) selectedForDatum() (core.HEntity, bool) {
	ss.GW.GroupSelection()
	gs := &ss.GW.gs
	if gs.n != 1 {
		return core.HEntity{}, false
	}
	he := slices.Concat(gs.point, gs.entity)[0]
	e := ss.SK.Entity(he)
	return he, e != nil && sketch.CanBeDatum(e)
}

// ShowDatums shows the datums screen.
func (tw *textWindow) ShowDatums() {
	ss := tw.ss
	sk := ss.SK
	tw.Printf(true, "%FtDATUMS%E   (names of what the model offers a model linking it)")
	if len(sk.Datums) == 0 {
		tw.Printf(false, "%Ba   (none)")
	}
	for i, d := range sk.Datums {
		bg := byte('d')
		if i%2 == 0 {
			bg = 'a'
		}
		what := "gone"
		if sk.Entity(d.Entity) != nil {
			what = sk.EntityName(d.Entity)
		}
		tw.Printf(false, "%Bp   %s %Fd%s%E %Fl%Ls%D%f[select]%E %Fl%Ln%D%f[rename]%E %Fl%Lx%D%f[delete]%E",
			bg, pad(d.Name, 13), what,
			i, lf(tw.ScreenChangeDatum), i, lf(tw.ScreenChangeDatum), i, lf(tw.ScreenChangeDatum))
	}
	if _, ok := ss.selectedForDatum(); ok {
		tw.Printf(true, "   %Fl%La%f[name the selected entity]%E", lf(tw.ScreenChangeDatum))
	} else {
		tw.Printf(true, "   %Fdselect a point, workplane, face or axis to name it%E")
	}

	k := 0
	for _, hg := range sk.GroupOrder {
		g := sk.Group(hg)
		if g.Type != sketch.GroupLinked || len(g.LinkDatums) == 0 {
			continue
		}
		label := sk.LinkLabel(g)
		tw.Printf(true, "%Ftlinked by %s%E", label)
		for _, d := range g.LinkDatums {
			name := label + "." + d.Name
			tw.Printf(false, "%Bd   %s %Fl%Ll%D%f[select]%E", pad(name, 24), k, lf(tw.ScreenSelectLinkedDatum))
			k++
		}
	}
}

// ScreenChangeDatum selects ('s'), renames ('n') or deletes ('x') datum
// v, or ('a') names the selected entity.
func (tw *textWindow) ScreenChangeDatum(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	if link == 'a' {
		if _, ok := ss.selectedForDatum(); !ok {
			return
		}
		tw.ShowEditControl(3, "")
		tw.edit.meaning = editDatumAdd
		return
	}
	i := int(v)
	if i < 0 || i >= len(sk.Datums) {
		return
	}
	d := sk.Datums[i]
	tw.edit.i = i
	switch link {
	case 's':
		ss.selectDatum(d.Entity)
	case 'n':
		tw.ShowEditControl(3, d.Name)
		tw.edit.meaning = editDatumName
	case 'x':
		ss.parameterEdit(func() error { return sk.DeleteDatum(d.Name) })
	}
}

// ScreenSelectLinkedDatum selects what the v-th datum of a linked model
// names, as the link places it.
func (tw *textWindow) ScreenSelectLinkedDatum(link int, v uint32) {
	ss := tw.ss
	names := ss.SK.DatumNames()[len(ss.SK.Datums):]
	if int(v) >= len(names) {
		return
	}
	he, err := ss.SK.ResolveDatum(names[v])
	if err != nil {
		ss.Report(err)
		return
	}
	ss.selectDatum(he)
}

// selectDatum adds the entity a datum names to the selection.
func (ss *Application) selectDatum(he core.HEntity) {
	if ss.SK.Entity(he) == nil {
		return
	}
	ss.GW.MakeSelectedEntity(he)
	ss.GW.Invalidate(false)
	ss.ScheduleShowTW()
}

// EditControlDoneForDatums takes an edit of the datums, and reports
// whether the edit was one of those.
func (tw *textWindow) EditControlDoneForDatums(s string) bool {
	ss := tw.ss
	sk := ss.SK
	switch tw.edit.meaning {
	case editDatumAdd:
		he, ok := ss.selectedForDatum()
		if !ok {
			return true
		}
		ss.parameterEdit(func() error { return sk.SetDatum(s, he) })
	case editDatumName:
		if tw.edit.i >= len(sk.Datums) {
			return true
		}
		d := sk.Datums[tw.edit.i]
		ss.parameterEdit(func() error { return sk.SetDatum(s, d.Entity) })
	default:
		return false
	}
	return true
}
