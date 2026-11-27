package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// Whether what is picked can be a fillet's or a chamfer's edges at all
// is asked before one is offered, as the context menu does, with what
// NewFillet and NewChamfer themselves ask of each edge. A line need
// not be an edge of the solid: an extrusion's keeps its length where a
// later group has cut the solid's edge shorter. No solid is made here,
// so what only making it tells, a size too large or a corner the kernel
// can't cut, is left to the operation to refuse.

// CheckFilletEdges says why NewFillet would refuse edges and solidEdges
// whatever the radius, or nil: each must be a whole edge of the solid
// so far at the active group, or a circle through one.
func (sk *Sketch) CheckFilletEdges(edges []core.HEntity, solidEdges []topo.Name) error {
	if len(edges) == 0 && len(solidEdges) == 0 {
		return errors.New("there are no edges to fillet")
	}
	_, err := sk.filletSegments(sk.solidAt(sk.Active), edges, solidEdges)
	return err
}

// filletSegments are the segments of a fillet of edges and solidEdges
// on solid, each an edge of it.
func (sk *Sketch) filletSegments(solid *Group, edges []core.HEntity, solidEdges []topo.Name) ([]chamferSegment, error) {
	if solid == nil || len(solid.RunningShell.Surface) == 0 {
		return nil, errors.New("there is no solid with surfaces to fillet")
	}
	if solid.IsForcedToMesh(sk) {
		return nil, errors.New("the solid is a triangle mesh, which has no edges to round")
	}
	segs, err := sk.chamferSegments(solid, edges, solidEdges)
	if err != nil {
		return nil, err
	}
	if err := picksOnSolid(solid, segs); err != nil {
		return nil, err
	}
	return segs, nil
}

// picksOnSolid says why one of segs is no edge of solid's faces
// (topo.Topology.PickEdge), which a fillet, and a chamfer's edit of the
// faces, need.
func picksOnSolid(solid *Group, segs []chamferSegment) error {
	tp, err := solid.Topology()
	if err != nil {
		return fmt.Errorf("the solid's topology is broken: %w", err)
	}
	for _, seg := range segs {
		if _, err := tp.PickEdge(seg.pick(0)); err != nil {
			return err
		}
	}
	return nil
}

// CheckChamferEdges says why NewChamfer would refuse edges and
// solidEdges whatever the distance, or nil. The edit of the solid's
// faces takes them when each is an edge of it, as a fillet's are;
// failing that a wedge must cut each straight one, which takes a line
// along part of an edge between two flat faces too. The solid is
// triangulated only in that second case.
func (sk *Sketch) CheckChamferEdges(edges []core.HEntity, solidEdges []topo.Name) error {
	if len(edges) == 0 && len(solidEdges) == 0 {
		return errors.New("there are no edges to chamfer")
	}
	solid := sk.solidAt(sk.Active)
	if solid == nil {
		return errors.New("there is no solid to chamfer")
	}
	segs, err := sk.chamferSegments(solid, edges, solidEdges)
	if err != nil {
		return err
	}
	surfaces := !solid.IsForcedToMesh(sk) && len(solid.RunningShell.Surface) > 0
	var editErr error
	if surfaces {
		if editErr = picksOnSolid(solid, segs); editErr == nil {
			return nil
		}
	}
	var mesh srf.SMesh
	solidMesh(solid, &mesh)
	for _, seg := range segs {
		switch {
		case !seg.arc:
			// The probe of the menu's distance, 1 mm.
			if _, err := cornerForWedge(&mesh, seg, 1); err != nil {
				return editReason(err, editErr)
			}
		case !surfaces:
			return errNoCircularEdges
		default:
			if err := picksOnSolid(solid, []chamferSegment{seg}); err != nil {
				return err
			}
		}
	}
	return nil
}

// errNoCircularEdges is why a circle of a mesh can't be chamfered.
var errNoCircularEdges = errors.New("the solid is a triangle mesh, which has no circular edges to cut")
