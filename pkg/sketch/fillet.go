package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
)

// A fillet rounds its edges of the solid so far with its radius, ValB:
// straight edges between two planar faces that stick out, ending at
// corners of three planar faces (topo.FilletEdge). It edits a copy of
// the solid so far, so its running shell is that solid rounded, with no
// Boolean; the faces it puts in are named from the group (srf.OriginFor),
// so their edges can be named in turn. Its own shell is that solid too,
// so the check and the display find a solid at it.

// NewFillet starts a fillet at the active group, of edges of the solid
// so far, given as line segment entities or by name, with radius mm.
// It tries the fillet on a copy of the solid, so an edge it can't round
// is an error here. It is what the interface and scripts add with
// AddGroupAfterActive.
func (sk *Sketch) NewFillet(edges []core.HEntity, solidEdges []topo.Name, radius float64, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupFillet
	g.Name = name
	if name == "" {
		g.Name = "fillet"
	}
	g.MeshCombine = CombineUnion
	g.ValB = radius
	if len(edges) == 0 && len(solidEdges) == 0 {
		return g, errors.New("there are no edges to fillet")
	}
	if radius <= 0 || math.IsNaN(radius) || math.IsInf(radius, 0) {
		return g, fmt.Errorf("a fillet's radius must be more than zero, not %g", radius)
	}
	solid := sk.solidAt(sk.Active)
	if solid == nil || len(solid.RunningShell.Surface) == 0 {
		return g, errors.New("there is no solid with surfaces to fillet at the active group")
	}
	if solid.IsForcedToMesh(sk) {
		return g, errors.New("the solid at the active group is a triangle mesh, which has no edges to round")
	}
	segs, err := sk.chamferSegments(solid, edges, solidEdges)
	if err != nil {
		return g, err
	}
	var sh srf.SShell
	sh.MakeFromCopyOf(&solid.RunningShell)
	if err := topo.FilletEdges(&sh, segmentPairs(segs), radius, 1<<30); err != nil {
		return g, err
	}
	g.Edges = append([]core.HEntity(nil), edges...)
	g.SolidEdges = append([]topo.Name(nil), solidEdges...)
	return g, nil
}

func segmentPairs(segs []chamferSegment) [][2]geom.Vector {
	out := make([][2]geom.Vector, len(segs))
	for i, s := range segs {
		out[i] = [2]geom.Vector{s.a, s.b}
	}
	return out
}

// generateFilletShell rounds the solid so far into the group's running
// shell; when it can't, the solid passes through and the group says
// why.
func (g *Group) generateFilletShell(sk *Sketch) {
	prev := g.PreviousGroup(sk)
	if prev != nil {
		g.RunningShell.MakeFromCopyOf(&prev.RunningShell)
	}
	switch {
	case len(g.Edges) == 0 && len(g.SolidEdges) == 0:
		g.setProblem(nil, "the fillet has no edges")
		return
	case g.ValB <= 0:
		g.setProblem(nil, "a fillet's radius must be more than zero")
		return
	case prev == nil || len(prev.RunningShell.Surface) == 0:
		g.setProblem(nil, "there is no solid with surfaces to fillet")
		return
	case prev.IsForcedToMesh(sk):
		g.setProblem(nil, "the solid so far is a triangle mesh, which has no edges to round")
		return
	}
	segs, err := sk.chamferSegments(prev, g.Edges, g.SolidEdges)
	if err != nil {
		g.setProblem(nil, "%v", err)
		return
	}
	var sh srf.SShell
	sh.MakeFromCopyOf(&prev.RunningShell)
	if err := topo.FilletEdges(&sh, segmentPairs(segs), g.ValB, srf.OriginFor(g.H.V, 0)); err != nil {
		var fe *topo.FilletError
		if errors.As(err, &fe) {
			at := fe.At
			g.setProblem(&at, "%v", err)
		} else {
			g.setProblem(nil, "%v", err)
		}
		return
	}
	g.RunningShell = sh
	g.ThisShell.MakeFromCopyOf(&sh)
}
