package api

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
	"github.com/telesapr/telesapr/pkg/srf"
)

// EditMenu holds the calls of the Edit menu: undo, regeneration, the
// clipboard, the selection, commands and preferences. A script reaches
// it as doc.Edit.
type EditMenu struct{ d *Doc }

// Copy puts the entities on the clipboard, with the constraints between
// them, as Edit > Copy does with them selected. Workplanes are left out.
func (m EditMenu) Copy(entities ...Entity) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		activeWorkplane(sk)
		sel := make([]sketch.Selection, 0, len(entities))
		for _, e := range entities {
			e.get(sk)
			sel = append(sel, sketch.Selection{Entity: e.H})
		}
		sk.CopySelection(sel)
	})
}

// Paste adds what is on the clipboard to the active group, in the
// active workplane, turned by angle degrees about the workplane's
// origin, scaled by scale and then moved by move, as Edit > Paste
// Transformed does once; a scale of -1 mirrors it. It returns what
// was drawn by pasting, a line for a line, without their points.
func (m EditMenu) Paste(move Vec, angle, scale float64) (pasted []Entity) {
	d := m.d
	d.change(func(sk *sketch.Sketch) {
		wp := activeWorkplane(sk)
		if len(sk.Clipboard.Requests) == 0 {
			fail("the clipboard is empty; Copy first")
		}
		if scale == 0 {
			fail("a scale of 0 would paste everything onto one point")
		}
		origin := sk.MustEntity(wp.Point[0]).PointGetNum(sk)
		wn := sk.MustEntity(wp.Normal).NormalN(sk)
		theta := angle * math.Pi / 180
		// Turned about the origin: Q*(p - o) + o + t = Q*p + (t + o - Q*o).
		t := move.Plus(origin).Minus(origin.RotatedAroundAxis(wn, theta))
		for _, s := range sk.PasteClipboard(t, theta, scale) {
			// What was drawn, not its points: a line, not its ends.
			if s.Entity.V != 0 && s.Entity.IsFromRequest() && s.Entity == s.Entity.Request().Entity(0) {
				pasted = append(pasted, Entity{d, s.Entity})
			}
		}
	})
	return pasted
}

// Regen regenerates the whole sketch and solves it.
func (m EditMenu) Regen() {
	d := m.d
	d.h.Run(func() {
		for i := range d.h.Sketch().Groups {
			d.h.Sketch().Groups[i].Clean = false
		}
		d.h.Sketch().ScheduleGenerateAll()
		d.h.Regenerate()
	})
}

// SetChordTolerance sets how closely meshes follow curves, as a
// percentage of the model's size (0.1 unless set; smaller is finer and
// slower), and regenerates everything. In the application it is the
// setting Configuration shows; scripts that check volumes against a
// formula use a fine one.
func (m EditMenu) SetChordTolerance(pct float64) {
	d := m.d
	if !(pct > 0) || math.IsInf(pct, 0) {
		fail("a chord tolerance must be more than zero, not %g", pct)
	}
	d.h.Run(func() {
		d.h.SetChordTolerance(pct)
		for i := range d.h.Sketch().Groups {
			d.h.Sketch().Groups[i].Clean = false
		}
		d.h.Sketch().ScheduleGenerateAll()
		d.h.Regenerate()
	})
}

// SetMaxSegments sets the most straight pieces a curve is cut into
// when meshes are made, from 7 to 1000 (20 unless set), and
// regenerates everything. In the application it is the setting
// "max piecewise linear segments" of Configuration. The pieces
// exported files cut curves into are ExportOptions.MaxSegments.
func (m EditMenu) SetMaxSegments(n int) {
	d := m.d
	if n < MinSegments || n > MaxSegmentsLimit {
		fail("max segments must be %d to %d, not %d", MinSegments, MaxSegmentsLimit, n)
	}
	d.h.Run(func() {
		d.h.SetMaxSegments(n)
		for i := range d.h.Sketch().Groups {
			d.h.Sketch().Groups[i].Clean = false
		}
		d.h.Sketch().ScheduleGenerateAll()
		d.h.Regenerate()
	})
}

// SetCheckClosedContour sets whether the closed loops of a sketch are
// checked for crossing themselves (on unless set; the check is slow),
// and regenerates everything. A loop that crosses itself is a problem
// on the group that uses it only while this is on; a loop that is not
// closed always is. In the application it is the setting "check
// closed contour" of Configuration, kept with the application and not
// in the file.
func (m EditMenu) SetCheckClosedContour(on bool) {
	d := m.d
	d.h.Run(func() {
		d.h.SetCheckClosedContour(on)
		for i := range d.h.Sketch().Groups {
			d.h.Sketch().Groups[i].Clean = false
		}
		d.h.Sketch().ScheduleGenerateAll()
		d.h.Regenerate()
	})
}

// CheckClosedContour is whether the closed loops of a sketch are
// checked for crossing themselves.
func (m EditMenu) CheckClosedContour() (on bool) {
	d := m.d
	d.read(func(sk *sketch.Sketch) { on = sk.CheckClosedContour })
	return on
}

// MaxSegments is the most straight pieces a curve is cut into when
// meshes are made.
func (m EditMenu) MaxSegments() (n int) {
	d := m.d
	d.read(func(sk *sketch.Sketch) { n = sk.SegmentLimit() })
	return n
}

// ChordTolerance is how closely meshes follow curves, as a percentage
// of the model's size.
func (m EditMenu) ChordTolerance() (pct float64) {
	d := m.d
	d.read(func(sk *sketch.Sketch) { pct = sk.ChordTolPercent() })
	return pct
}

// Undo takes back the last step: in a console, what the last line
// that changed the sketch did.
func (m EditMenu) Undo() (ok bool) {
	d := m.d
	d.h.Run(func() { ok = d.h.Undo() })
	return ok
}

// Redo does again what Undo took back.
func (m EditMenu) Redo() (ok bool) {
	d := m.d
	d.h.Run(func() { ok = d.h.Redo() })
	return ok
}

// Selection is what is selected on the drawing, but for the
// constraints.
func (m EditMenu) Selection() (entities []Entity) {
	d := m.d
	d.read(func(*sketch.Sketch) {
		for _, s := range d.h.Selection() {
			if s.Entity.V != 0 {
				entities = append(entities, Entity{d, s.Entity})
			}
		}
	})
	return entities
}

// SelectedConstraints are the constraints selected on the drawing.
func (m EditMenu) SelectedConstraints() (constraints []Constraint) {
	d := m.d
	d.read(func(*sketch.Sketch) {
		for _, s := range d.h.Selection() {
			if s.Constraint.V != 0 {
				constraints = append(constraints, Constraint{d, s.Constraint})
			}
		}
	})
	return constraints
}

// Select selects entities on the drawing, and nothing else.
func (m EditMenu) Select(entities ...Entity) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		sel := make([]sketch.Selection, 0, len(entities))
		for _, e := range entities {
			e.get(sk)
			sel = append(sel, sketch.Selection{Entity: e.H})
		}
		d.h.Select(sel)
	})
}

// SelectEdges selects edges of the solid on the drawing, and nothing
// else, as picking them on the model does: for a fillet or a chamfer
// made from the menu. The edges must be of the solid at the active
// group, which such a group is made after.
func (m EditMenu) SelectEdges(edges ...SolidEdge) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		sel := make([]sketch.Selection, 0, len(edges))
		for _, e := range edges {
			e.resolve(sk)
			sel = append(sel, sketch.Selection{Edge: e.name})
		}
		d.h.Select(sel)
	})
}

// SelectFaces selects faces of the solid on the drawing, and nothing
// else, as picking them on the model does: for a shell made from the
// menu, or a face to draw on. A face an entity made is selected by its
// entity, one that no entity names by its identity.
func (m EditMenu) SelectFaces(faces ...SolidFace) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		sel := make([]sketch.Selection, 0, len(faces))
		for _, f := range faces {
			h := core.HEntity{V: uint32(f.id)}
			if sk.Entity(h) != nil {
				sel = append(sel, sketch.Selection{Entity: h})
			} else {
				sel = append(sel, sketch.Selection{Face: f.id})
			}
		}
		d.h.Select(sel)
	})
}

// SelectedEdges are the edges of the solid selected on the drawing,
// picked on the model or by SelectEdges, as edges of the solid at the
// active group.
func (m EditMenu) SelectedEdges() (edges []SolidEdge) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		for _, s := range d.h.Selection() {
			if s.Edge.A != 0 {
				edges = append(edges, SolidEdge{s: Solid{d: d, g: sk.ActiveGroup()}, name: s.Edge})
			}
		}
	})
	return edges
}

// SelectedFaces are the faces of the solid selected on the drawing,
// picked on the model or by SelectFaces, as faces of the solid at the
// active group: a face entity's, or a face no entity names by its
// identity. A face is given as the first with its identity.
func (m EditMenu) SelectedFaces() (faces []SolidFace) {
	d := m.d
	d.read(func(sk *sketch.Sketch) {
		s := Solid{d: d, g: sk.ActiveGroup()}
		for _, x := range d.h.Selection() {
			switch {
			case x.Face != 0:
				faces = append(faces, SolidFace{s: s, id: x.Face})
			case x.Entity.V != 0:
				if e := sk.Entity(x.Entity); e != nil && e.IsFace() {
					faces = append(faces, SolidFace{s: s, id: srf.FaceID(x.Entity.V)})
				}
			}
		}
	})
	return faces
}

// RunCommand runs a command of the main menu, as choosing it does:
// "View > Zoom To Fit", or the entry's own name, "Zoom To Fit", when
// only one entry has it. telesapr-cli runs scripts on the interface
// without a window, so commands run there too; a document of a Go
// program's own, from New, has no menu to run them from, and it is an
// error. What the command says went wrong is the error too: a command
// that would replace a drawing with unsaved changes, as File > New,
// refuses; see ForgetChanges. What the command changes is part of the
// script's step of undo.
func (m EditMenu) RunCommand(label string) (err error) {
	d := m.d
	d.h.Run(func() {
		sk := d.h.Sketch()
		undo := sk.Undo.Write
		err = d.h.RunCommand(label)
		if d.h.Sketch() == sk {
			d.h.Remembered(sk.UndoPushedSince(undo))
		}
	})
	return err
}
