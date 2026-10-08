package sketch

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// This file is the sketch editing the window asks for: adding a
// request, deleting the selection, and fixing up the constraints on
// what's deleted. The selection is passed in.

// Selection is a selected entity or constraint.
type Selection struct {
	Entity     core.HEntity
	Constraint core.HConstraint
	// Edge is an edge of the solid by name, when its A is not zero,
	// and Face a face no entity names: what the mouse can be over.
	Edge topo.Name
	Face srf.FaceID
}

// AddRequestOfType adds a request of the given type in the active group
// and workplane, and generates its entities and params, without
// solving.
func (sk *Sketch) AddRequestOfType(typ RequestType, rememberForUndo bool) core.HRequest {
	if rememberForUndo {
		sk.UndoRemember()
	}

	r := Request{}
	r.Group = sk.ActiveGroup()
	g := sk.Group(r.Group)
	if g.Type == GroupDrawing3D || g.Type == GroupDrawingWorkplane || typ == RequestDatumPoint {
		r.Construction = false
	} else {
		r.Construction = true
	}
	r.Workplane = sk.ActiveWorkplane()
	if r.Workplane == core.NoEntity {
		// A group with no workplane draws in 3d, which is what its file
		// reads back as, so a save and a reload agree.
		r.Workplane = core.FreeIn3D
	}
	r.Type = typ
	r.H = sk.AddRequest(r)

	// We must regenerate the parameters, so that the code that tries to
	// place this request's entities where the mouse is can do so. But
	// we mustn't try to solve until reasonable values have been supplied
	// for these new parameters, or else we'll get a numerical blowup.
	entities := core.EntityList(sk.Entities)
	params := core.ParamList(sk.Params)
	r.Generate(sk, &entities, &params)
	sk.Entities = []core.Entity(entities)
	sk.Params = []core.Param(params)
	sk.MarkGroupDirty(r.Group, false)
	return r.H
}

// DeleteSelection deletes the selected requests (not the references)
// and constraints.
func (sk *Sketch) DeleteSelection(selection []Selection) {
	for i := range sk.Requests {
		sk.Requests[i].Tag = 0
	}
	delC := make(map[core.HConstraint]bool)
	for _, s := range selection {
		var r core.HRequest
		if s.Entity.V != 0 && s.Entity.IsFromRequest() {
			r = s.Entity.Request()
		}
		if r.V != 0 && !r.IsFromReferences() {
			if req := sk.Request(r); req != nil {
				req.Tag = 1
			}
		}
		if s.Constraint.V != 0 {
			delC[s.Constraint] = true
		}
	}

	kept := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if !delC[c.H] {
			kept = append(kept, c)
		}
	}
	sk.Constraints = kept
	// This regenerates, to avoid lingering references to the
	// just-deleted items.
	sk.DeleteTaggedRequests()
}

// DeleteTaggedRequests deletes the tagged requests, fixing up the
// point-coincident constraints on their points, and regenerates.
func (sk *Sketch) DeleteTaggedRequests() {
	// Rewrite any point-coincident constraints that were affected by this
	// deletion.
	var tagged []core.HRequest
	for i := range sk.Requests {
		if sk.Requests[i].Tag != 0 {
			tagged = append(tagged, sk.Requests[i].H)
		}
	}
	for _, hr := range tagged {
		sk.FixConstraintsForRequestBeingDeleted(hr)
	}
	// and then delete the tagged requests.
	kept := sk.Requests[:0]
	for _, r := range sk.Requests {
		if r.Tag == 0 {
			kept = append(kept, r)
		}
	}
	sk.Requests = kept

	// The user interface's part: it clears what could refer to the
	// deleted requests, then regenerates.
	if sk.hooks().RegenerateAfterDelete() {
		return
	}
	// And regenerate to get rid of what it generates, plus anything
	// that references it (since the regen code checks for that).
	sk.GenerateAllTyped(GenerateAll)
}

// ReplacePointInConstraints moves the constraints on oldpt to newpt.
func (sk *Sketch) ReplacePointInConstraints(oldpt, newpt core.HEntity) {
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.PtA == oldpt {
			c.PtA = newpt
		}
		if c.PtB == oldpt {
			c.PtB = newpt
		}
	}
}

// RemoveConstraintsForPointBeingDeleted deletes the constraints on hpt.
func (sk *Sketch) RemoveConstraintsForPointBeingDeleted(hpt core.HEntity) {
	kept := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		if c.PtA == hpt || c.PtB == hpt {
			sk.Deleted.Constraints++
			if c.Type != core.ConstraintPointsCoincident &&
				c.Type != core.ConstraintHorizontal &&
				c.Type != core.ConstraintVertical {
				sk.Deleted.NonTrivialConstraints++
			}
			continue
		}
		kept = append(kept, c)
	}
	sk.Constraints = kept
}

// FixConstraintsForRequestBeingDeleted keeps the points made coincident
// through a point of request hr, which is being deleted, coincident.
func (sk *Sketch) FixConstraintsForRequestBeingDeleted(hr core.HRequest) {
	r := sk.Request(hr)
	if r.Group != sk.ActiveGroup() {
		return
	}

	var points []core.HEntity
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if !e.H.IsFromRequest() || e.H.Request() != hr {
			continue
		}
		if e.Type != core.EntityPointIn2D && e.Type != core.EntityPointIn3D {
			continue
		}
		points = append(points, e.H)
	}
	for _, h := range points {
		// This is a point generated by the request being deleted; so fix
		// the constraints for that.
		sk.FixConstraintsForPointBeingDeleted(h)
	}
}

// FixConstraintsForPointBeingDeleted deletes the point-coincident
// constraints on hpt, and makes the points they joined to it coincident
// with each other.
func (sk *Sketch) FixConstraintsForPointBeingDeleted(hpt core.HEntity) {
	var ld []core.HEntity

	kept := sk.Constraints[:0]
	for _, c := range sk.Constraints {
		tag := false
		if c.Type == core.ConstraintPointsCoincident && c.Group == sk.ActiveGroup() {
			if c.PtA == hpt {
				ld = append(ld, c.PtB)
				tag = true
			}
			if c.PtB == hpt {
				ld = append(ld, c.PtA)
				tag = true
			}
		}
		if !tag {
			kept = append(kept, c)
		}
	}
	// Remove constraints without waiting for regeneration; this way
	// if another point takes the place of the deleted one (e.g. when
	// removing control points of a bezier) the constraint doesn't
	// spuriously move. Similarly, subsequent calls of this function
	// (if multiple coincident points are getting deleted) will work
	// correctly.
	sk.Constraints = kept

	// If more than one point was constrained coincident with hpt, then
	// those two points were implicitly coincident with each other. By
	// deleting hpt (and all constraints that mention it), we will delete
	// that relationship. So put it back here now.
	for i := 1; i < len(ld); i++ {
		sk.ConstrainCoincident(ld[i-1], ld[i])
	}
}
