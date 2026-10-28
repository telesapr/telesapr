package ui

import (
	"slices"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// The datums screen (Edit > Datums) lists the names given to entities
// the model offers others, and those of the models it links; it names
// the selected entity, or the selected face or edge of the solid,
// renames and deletes, selects what a name names, and mates two names.
// The window's side of doc.Datums.

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

// selectedSolidForDatum is the one face or edge of the solid selected
// that no entity makes, if one is: a datum names it by a workplane
// that follows it (Datums.NameFace, Datums.NameEdge).
func (ss *Application) selectedSolidForDatum() (face srf.FaceID, edge *topo.Name, ok bool) {
	ss.GW.GroupSelection()
	gs := &ss.GW.gs
	if gs.n != 0 || gs.constraints != 0 {
		return 0, nil, false
	}
	switch {
	case len(gs.solidFace) == 1 && len(gs.solidEdge) == 0:
		return gs.solidFace[0], nil, true
	case len(gs.solidFace) == 0 && len(gs.solidEdge) == 1:
		n := gs.solidEdge[0]
		return 0, &n, true
	}
	return 0, nil, false
}

// selectedSolidAt is where on the one face or edge of the solid
// selected the mouse picked it, or nil: picked from a list, as a
// feature's screen's, it was picked nowhere in particular.
func (ss *Application) selectedSolidAt() *geom.Vector {
	for _, s := range ss.GW.selection {
		if s.Face != 0 || s.Edge.A != 0 {
			return s.At
		}
	}
	return nil
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
		if d.Kind != topo.DatumUnknown {
			what += " " + d.Kind.String()
		}
		tw.Printf(false, "%Bp   %s %Fd%s%E %Fl%Ls%D%f[select]%E %Fl%Ln%D%f[rename]%E %Fl%Lx%D%f[delete]%E %Fl%Lm%D%f[mate]%E",
			bg, pad(d.Name, 13), what,
			i, lf(tw.ScreenChangeDatum), i, lf(tw.ScreenChangeDatum), i, lf(tw.ScreenChangeDatum),
			i, lf(tw.ScreenPickDatumForMate))
		if datumAfterActive(sk, d) {
			tw.Printf(false, "%Bp       %Fx(after the active group: it follows its face when that group is active, and on saving)%E", bg)
		}
	}
	if _, ok := ss.selectedForDatum(); ok {
		tw.Printf(true, "   %Fl%La%f[name the selected entity]%E", lf(tw.ScreenChangeDatum))
	} else if _, edge, ok := ss.selectedSolidForDatum(); ok {
		what := "face"
		if edge != nil {
			what = "edge"
		}
		tw.Printf(true, "   %Fl%Lf%f[name the selected %s of the solid]%E", lf(tw.ScreenChangeDatum), what)
	} else {
		tw.Printf(true, "   %Fdselect a point, workplane, face, edge or axis to name it%E")
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
			if d.Kind != topo.DatumUnknown {
				name += " " + d.Kind.String()
			}
			tw.Printf(false, "%Bd   %s %Fl%Ll%D%f[select]%E %Fl%Lm%D%f[mate]%E", pad(name, 24), k, lf(tw.ScreenSelectLinkedDatum),
				len(sk.Datums)+k, lf(tw.ScreenPickDatumForMate))
			k++
		}
	}
	tw.showDatumMate()
}

// datumAfterActive reports whether datum d names an entity of a group
// after the active one, which is not made again until it is active or
// the model is saved: a datum on the solid there stays where it was
// last found while groups before it change.
func datumAfterActive(sk *sketch.Sketch, d sketch.Datum) bool {
	e := sk.Entity(d.Entity)
	if e == nil {
		return false
	}
	g, active := sk.Group(e.Group), sk.Group(sk.Active)
	return g != nil && active != nil && g.Order > active.Order
}

// showDatumMate shows the names picked for a mate, and the mates that
// can be made of two.
func (tw *textWindow) showDatumMate() {
	switch len(tw.datumMate) {
	case 0:
		tw.Printf(true, "   %Fd[mate] two names to make a mate of them%E")
	case 1:
		tw.Printf(true, "%Ftmate%E %s and   %Fd([mate] another)%E   %Fl%Lx%f[clear]%E", tw.datumMate[0], lf(tw.ScreenMateDatums))
	default:
		tw.Printf(true, "%Ftmate%E %s and %s", tw.datumMate[0], tw.datumMate[1])
		tw.Printf(false, "   %Fl%Lf%f[flush]%E %Fl%Lc%f[concentric]%E %Fl%Lh%f[hinge]%E %Fl%Lx%f[clear]%E",
			lf(tw.ScreenMateDatums), lf(tw.ScreenMateDatums), lf(tw.ScreenMateDatums), lf(tw.ScreenMateDatums))
	}
}

// ScreenPickDatumForMate picks the v-th datum name (DatumNames) for a
// mate: the first is of the part that stays. A third starts again.
func (tw *textWindow) ScreenPickDatumForMate(link int, v uint32) {
	names := tw.ss.SK.DatumNames()
	if int(v) >= len(names) {
		return
	}
	if len(tw.datumMate) == 2 {
		tw.datumMate = nil
	}
	tw.datumMate = append(tw.datumMate, names[v])
	tw.ss.ScheduleShowTW()
}

// ScreenMateDatums makes the two names picked flush ('f'), concentric
// ('c') or a hinge ('h'), as the Assembly menu's mates do of a
// selection, or clears them ('x').
func (tw *textWindow) ScreenMateDatums(link int, v uint32) {
	ss := tw.ss
	picked := tw.datumMate
	if link == 'x' || len(picked) != 2 {
		tw.datumMate = nil
		ss.ScheduleShowTW()
		return
	}
	mates := map[int]struct {
		what string
		run  func(d *api.Doc, a, b api.Entity)
	}{
		'f': {"flush", func(d *api.Doc, a, b api.Entity) { d.Assembly.Flush(a, b) }},
		'c': {"concentric", func(d *api.Doc, a, b api.Entity) { d.Assembly.Concentric(a, b) }},
		'h': {"hinge", func(d *api.Doc, a, b api.Entity) { d.Assembly.Hinge(a, b) }},
	}
	m, ok := mates[link]
	if !ok {
		return
	}
	ss.makeMate(m.what, func(d *api.Doc) { m.run(d, d.Datums.Get(picked[0]), d.Datums.Get(picked[1])) })
	tw.datumMate = nil
	ss.ScheduleShowTW()
}

// nameSolidDatum names the selected face or edge of the solid as the
// datum name, by a workplane that follows it, as one step of undo.
func (ss *Application) nameSolidDatum(name string) {
	face, edge, ok := ss.selectedSolidForDatum()
	if !ok {
		return
	}
	at := ss.selectedSolidAt()
	host := ss.ScriptHost()
	host.BeginEval()
	d := api.NewDoc(host)
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				e, isErr := r.(error)
				if !isErr {
					panic(r)
				}
				err = e
			}
		}()
		solid := d.Active().Solid()
		// Where it was picked, which a face or an edge neither flat nor
		// round keeps (Datums.NameFaceAt).
		switch {
		case edge != nil && at != nil:
			d.Datums.NameEdgeAt(name, solid.Edge(edge.String()), *at)
		case edge != nil:
			d.Datums.NameEdge(name, solid.Edge(edge.String()))
		case at != nil:
			d.Datums.NameFaceAt(name, solid.Face(uint32(face)), *at)
		default:
			d.Datums.NameFace(name, solid.Face(uint32(face)))
		}
		return nil
	}()
	if err != nil {
		host.Rollback()
		ss.Error("Can't name it %s: %s.", name, err.Error())
	}
	host.EndEval()
	ss.GW.ClearSelection()
	ss.ScheduleShowTW()
}

// ScreenChangeDatum selects ('s'), renames ('n') or deletes ('x') datum
// v, or names the selected entity ('a') or face or edge of the solid
// ('f').
func (tw *textWindow) ScreenChangeDatum(link int, v uint32) {
	ss := tw.ss
	sk := ss.SK
	switch link {
	case 'a':
		if _, ok := ss.selectedForDatum(); !ok {
			return
		}
		tw.ShowEditControl(3, "")
		tw.edit.meaning = editDatumAdd
		return
	case 'f':
		if _, _, ok := ss.selectedSolidForDatum(); !ok {
			return
		}
		tw.ShowEditControl(3, "")
		tw.edit.meaning = editDatumSolid
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
	case editDatumSolid:
		ss.nameSolidDatum(strings.TrimSpace(s))
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
