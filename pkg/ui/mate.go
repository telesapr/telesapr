package ui

import (
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/thread"
)

// MenuMate makes a mate of the selection (roadmap 16.9): the
// constraints api.Doc.Concentric, Flush, Hinge or Screw make, as one
// step of undo, in the active group. A screw's lead is the coarse pitch
// of the ISO metric thread of the first circle's diameter, or 1 mm; it
// is the parameter the screw makes, to be changed in the table.
func (ss *Application) MenuMate(id Command) {
	gw := ss.GW
	gw.GroupSelection()
	gs := &gw.gs
	var what, need string
	var run func(d *api.Doc, a, b api.Entity)
	var picked []uint32
	switch id {
	case cmdMateConcentric:
		what, need = "concentric", "two circles or arcs"
		run = func(d *api.Doc, a, b api.Entity) { d.Assembly.Concentric(a, b) }
		if gs.circlesOrArcs == 2 && gs.n == 2 {
			for _, h := range gs.entity {
				picked = append(picked, h.V)
			}
		}
	case cmdMateFlush:
		what, need = "flush", "two faces"
		run = func(d *api.Doc, a, b api.Entity) { d.Assembly.Flush(a, b) }
		if gs.faces == 2 && gs.n == 2 {
			for _, h := range gs.face {
				picked = append(picked, h.V)
			}
		}
	case cmdMateScrew:
		what, need = "screw", "two circles or arcs"
		run = func(d *api.Doc, a, b api.Entity) {
			lead, ok := thread.CoarsePitch(2 * a.Radius())
			if !ok {
				lead = 1
			}
			d.Assembly.Screw(a, b, lead)
		}
		if gs.circlesOrArcs == 2 && gs.n == 2 {
			for _, h := range gs.entity {
				picked = append(picked, h.V)
			}
		}
	case cmdMateHinge:
		what, need = "hinge", "two line segments"
		run = func(d *api.Doc, a, b api.Entity) { d.Assembly.Hinge(a, b) }
		if gs.lineSegments == 2 && gs.n == 2 {
			for _, h := range gs.entity {
				picked = append(picked, h.V)
			}
		}
	}
	if len(picked) != 2 {
		ss.Error("Bad selection for a %s mate. Select %s: the first of the part that stays, "+
			"the second of the part the active group places.", what, need)
		return
	}
	ss.makeMate(what, func(d *api.Doc) { run(d, d.Entity(picked[0]), d.Entity(picked[1])) })
	gw.ClearSelection()
}

// makeMate runs mate, which makes the constraints of a mate by what, as
// one step of undo, or says why it can't.
func (ss *Application) makeMate(what string, mate func(d *api.Doc)) {
	host := ss.ScriptHost()
	host.BeginEval()
	d := api.NewDoc(host)
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				e, ok := r.(error)
				if !ok {
					panic(r)
				}
				err = e
			}
		}()
		mate(d)
		return nil
	}()
	if err != nil {
		host.Rollback()
		ss.Error("Can't make the %s mate: %s.", what, err.Error())
	}
	host.EndEval()
}
