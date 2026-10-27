package sketch

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A fillet rounds its edges of the solid so far with its radius, ValB,
// or an edge's own (EdgeRadii):
// straight edges between two planar faces, or circles where a planar
// face meets a cylinder or a cone, ending at corners of three faces
// (topo.FilletPicks); those that go in are filled. It edits a copy of
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
	if err := sk.filletTrial(sk.solidAt(sk.Active), edges, solidEdges, radius, nil); err != nil {
		return g, err
	}
	g.Edges = append([]core.HEntity(nil), edges...)
	g.SolidEdges = append([]topo.Name(nil), solidEdges...)
	return g, nil
}

// SetFilletEdges makes the fillet hg round edges and solidEdges, picked
// on the solid before it, instead of the edges it rounds now: a
// fillet's edges can be changed after it is made (ROADMAP F7). An
// edge's own radius stays with it; an edge taken away takes its
// radius with it, and an edge added takes the fillet's. The fillet is
// tried on a copy of the solid before it, so an edge it can't round is
// an error here and nothing changes. The caller remembers the undo
// state.
func (sk *Sketch) SetFilletEdges(hg core.HGroup, edges []core.HEntity, solidEdges []topo.Name) error {
	g, err := sk.inputGroup(hg, GroupFillet, "edges")
	if err != nil {
		return err
	}
	edges, solidEdges = dedup(edges), dedup(solidEdges)
	if len(edges) == 0 && len(solidEdges) == 0 {
		return errors.New("there are no edges to fillet")
	}
	if slices.Equal(edges, g.Edges) && slices.Equal(solidEdges, g.SolidEdges) {
		return nil
	}
	if err := sk.edgesBefore(hg, edges, solidEdges); err != nil {
		return err
	}
	radii := carryEdgeRadii(g, edges, solidEdges)
	if err := sk.filletTrial(sk.solidBefore(g), edges, solidEdges, g.ValB, radii); err != nil {
		return err
	}
	g.Edges, g.SolidEdges, g.EdgeRadii = edges, solidEdges, radii
	sk.MarkGroupDirty(hg, false)
	return nil
}

// filletTrial rounds edges and solidEdges of solid's running shell on a
// copy, with radius and the edges' own radii, or says why it can't:
// what NewFillet and SetFilletEdges check before the fillet is made.
func (sk *Sketch) filletTrial(solid *Group, edges []core.HEntity, solidEdges []topo.Name, radius float64, radii []EdgeRadius) error {
	if solid == nil || len(solid.RunningShell.Surface) == 0 {
		return errors.New("there is no solid with surfaces to fillet")
	}
	if solid.IsForcedToMesh(sk) {
		return errors.New("the solid is a triangle mesh, which has no edges to round")
	}
	segs, err := sk.chamferSegments(solid, edges, solidEdges)
	if err != nil {
		return err
	}
	var sh srf.SShell
	sh.MakeFromCopyOf(&solid.RunningShell)
	return topo.FilletPicks(&sh, filletPicks(segs, radii), radius, 1<<30, false)
}

// filletPicks are the segments as topo picks them, each with its own
// radius where radii gives it one.
func filletPicks(segs []chamferSegment, radii []EdgeRadius) []topo.FilletPick {
	out := make([]topo.FilletPick, len(segs))
	for i, s := range segs {
		out[i] = s.pick(0)
	}
	for _, er := range radii {
		if er.Edge >= 0 && er.Edge < len(out) {
			out[er.Edge].R = er.Val
		}
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
	picks := filletPicks(segs, g.EdgeRadii)
	var sh srf.SShell
	sh.MakeFromCopyOf(&prev.RunningShell)
	if err := topo.FilletPicks(&sh, picks, g.ValB, srf.OriginFor(g.H.V, 0), false); err != nil {
		if fe, ok := errors.AsType[*topo.FilletError](err); ok {
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
