package ui

import (
	"github.com/telesapr/telesapr/pkg/script/api"
)

// MenuMate makes a mate of the selection (roadmap 16.9): the
// constraints api.Doc.Concentric, Flush or Hinge make, as one step of
// undo, in the active group.
func (ss *SolveSpaceUI) MenuMate(id Command) {
	gw := ss.GW
	gw.GroupSelection()
	gs := &gw.gs
	var what, need string
	var run func(d *api.Doc, a, b api.Entity)
	var picked []uint32
	switch id {
	case CmdMateConcentric:
		what, need = "concentric", "two circles or arcs"
		run = func(d *api.Doc, a, b api.Entity) { d.Concentric(a, b) }
		if gs.circlesOrArcs == 2 && gs.n == 2 {
			for _, h := range gs.entity {
				picked = append(picked, h.V)
			}
		}
	case CmdMateFlush:
		what, need = "flush", "two faces"
		run = func(d *api.Doc, a, b api.Entity) { d.Flush(a, b) }
		if gs.faces == 2 && gs.n == 2 {
			for _, h := range gs.face {
				picked = append(picked, h.V)
			}
		}
	case CmdMateHinge:
		what, need = "hinge", "two line segments"
		run = func(d *api.Doc, a, b api.Entity) { d.Hinge(a, b) }
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
	host := ss.ScriptHost()
	host.BeginEval()
	d := api.NewDoc(host)
	a, b := d.Entity(picked[0]), d.Entity(picked[1])
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
		run(d, a, b)
		return nil
	}()
	if err != nil {
		host.Rollback()
		ss.Error("Can't make the %s mate: %s.", what, err.Error())
	}
	host.EndEval()
	gw.ClearSelection()
}
