package ui

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/thread"
)

// MenuMate makes a mate of the selection (roadmap 16.9): the
// constraints api.Doc.Concentric, Flush, Hinge or Screw make, as one
// step of undo, in the active group. Concentric and Flush take datums
// on a solid selected as their workplanes or, through a link, their
// normals, as the datums screen's [mate] does. A screw's lead is the
// lead of the screw thread a circle carries, and the part is moved to
// where the threads fit (api.AssemblyMenu.Screw); without threads it is
// the coarse pitch of the ISO metric thread of the first circle's
// diameter, or 1 mm. It is the parameter the screw makes, to be changed
// in the table.
func (ss *Application) MenuMate(id Command) {
	gw := ss.GW
	gw.GroupSelection()
	gs := &gw.gs
	var what, need string
	var run func(d *api.Doc, a, b api.Entity)
	var picked []uint32
	switch id {
	case cmdMateConcentric:
		what, need = "concentric", "two circles, arcs or datum axes"
		run = func(d *api.Doc, a, b api.Entity) { d.Assembly.Concentric(a, b) }
		picked = gw.pickedForMate(func(e *core.Entity) bool {
			return e.Type == core.EntityCircle || e.Type == core.EntityArcOfCircle || isDatumFrame(e)
		})
	case cmdMateFlush:
		what, need = "flush", "two faces or datum planes"
		run = func(d *api.Doc, a, b api.Entity) { d.Assembly.Flush(a, b) }
		picked = gw.pickedForMate(func(e *core.Entity) bool { return e.IsFace() || isDatumFrame(e) })
	case cmdMateScrew:
		what, need = "screw", "two circles or arcs"
		run = func(d *api.Doc, a, b api.Entity) {
			// A lead of 0 is the threads', which the mate also fits.
			lead := 0.0
			_, onA := ss.SK.ThreadOnCircle(a.H)
			_, onB := ss.SK.ThreadOnCircle(b.H)
			if !onA && !onB {
				var ok bool
				if lead, ok = thread.CoarsePitch(2 * a.Radius()); !ok {
					lead = 1
				}
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

// pickedForMate is the two entities selected, in the order they were
// selected, when there are two and ok takes both; none otherwise.
func (gw *graphicsWindow) pickedForMate(ok func(e *core.Entity) bool) []uint32 {
	if gw.gs.n != 2 || len(gw.gs.solidEdge)+len(gw.gs.solidFace)+gw.gs.constraints != 0 {
		return nil
	}
	var picked []uint32
	for _, s := range gw.selection {
		e := gw.sk().Entity(s.Entity)
		if e == nil || !ok(e) {
			return nil
		}
		picked = append(picked, s.Entity.V)
	}
	return picked
}

// isDatumFrame reports whether e is a workplane, or a normal through a
// point, as a datum on a solid is, here or through a link: a mate takes
// it as a plane or an axis, as its kind allows.
func isDatumFrame(e *core.Entity) bool {
	return e.IsWorkplane() || e.IsNormal() && e.Point[0].V != 0
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
