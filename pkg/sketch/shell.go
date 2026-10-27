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

// A shell hollows the solid so far to walls of its thickness, ValB,
// open where its faces, SolidFaces, are taken away (topo.ShellSolid):
// solids of flat faces, and prisms of lines and arcs open at their ends;
// and when the solid so far is one lathe or revolve, by its profile
// (topo.ShellRevolved).
// It works on a copy of the solid
// so far, so its running shell is that solid hollowed, with the Boolean
// done inside; the cavity's faces are named from the group
// (srf.OriginFor). Its own shell is that solid too, so the check and the
// display find a solid at it.

// NewShell starts a shell at the active group, hollowing the solid so
// far to walls thick mm, open at faces. It tries the shell on a copy of
// the solid, so a solid it can't hollow is an error here. It is what the
// interface and scripts add with AddGroupAfterActive.
func (sk *Sketch) NewShell(faces []srf.FaceID, thick float64, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupShell
	g.Name = name
	if name == "" {
		g.Name = "shell"
	}
	g.MeshCombine = CombineUnion
	g.ValB = thick
	if len(faces) == 0 {
		return g, errors.New("there are no faces to take away: a shell is open where faces are taken away")
	}
	if thick <= 0 || math.IsNaN(thick) || math.IsInf(thick, 0) {
		return g, fmt.Errorf("a shell's thickness must be more than zero, not %g", thick)
	}
	solid := sk.solidAt(sk.Active)
	if solid == nil || len(solid.RunningShell.Surface) == 0 {
		return g, errors.New("there is no solid with surfaces to hollow at the active group")
	}
	if solid.IsForcedToMesh(sk) {
		return g, errors.New("the solid at the active group is a triangle mesh, which has no faces to move")
	}
	if _, err := shellOf(&solid.RunningShell, faces, thick, 1<<30, sk.revolutionAt(solid)); err != nil {
		return g, err
	}
	g.SolidFaces = append([]srf.FaceID(nil), faces...)
	return g, nil
}

// SetShellFaces makes the shell hg open where faces, picked on the
// solid before it, are taken away, instead of the faces it takes away
// now: a shell's faces can be changed after it is made (ROADMAP F7).
// The shell is tried on a copy of the solid before it, so a face it
// can't open is an error here and nothing changes. The caller remembers
// the undo state.
func (sk *Sketch) SetShellFaces(hg core.HGroup, faces []srf.FaceID) error {
	g, err := sk.inputGroup(hg, GroupShell, "faces")
	if err != nil {
		return err
	}
	faces = dedup(faces)
	if len(faces) == 0 {
		return errors.New("there are no faces to take away: a shell is open where faces are taken away")
	}
	if slices.Equal(faces, g.SolidFaces) {
		return nil
	}
	for _, id := range faces {
		if err := sk.inputBefore(hg, faceGroup(id), sk.FaceName(id)); err != nil {
			return err
		}
	}
	solid := sk.solidBefore(g)
	if solid == nil || len(solid.RunningShell.Surface) == 0 {
		return errors.New("there is no solid with surfaces to hollow")
	}
	if solid.IsForcedToMesh(sk) {
		return errors.New("the solid is a triangle mesh, which has no faces to move")
	}
	if _, err := shellOf(&solid.RunningShell, faces, g.ValB, 1<<30, sk.revolutionAt(solid)); err != nil {
		return err
	}
	g.SolidFaces = faces
	sk.MarkGroupDirty(hg, false)
	return nil
}

// shellOf hollows sh, taking away the faces with identities faces. A
// solid of flat faces or a prism is hollowed as such; failing that, a
// solid that is rev (when not nil) by its profile.
func shellOf(sh *srf.SShell, faces []srf.FaceID, thick float64, origin uint32, rev *topo.Revolution) (*srf.SShell, error) {
	t, err := topo.Build(sh)
	if err != nil {
		return nil, err
	}
	var open []int
	for _, id := range faces {
		fs := t.FacesWithID(id)
		if len(fs) == 0 {
			return nil, fmt.Errorf("face %x is no longer in the solid", uint32(id))
		}
		open = append(open, fs...)
	}
	out, err := topo.ShellSolid(sh, open, thick, origin)
	if err == nil || rev == nil {
		return out, err
	}
	out, rerr := topo.ShellRevolved(sh, *rev, open, thick, origin)
	if errors.Is(rerr, topo.ErrNotRevolution) {
		return nil, err
	}
	return out, rerr
}

// revolutionAt is how the solid at group g was made when it is one
// lathe or revolve: the first group at or before g that makes or
// changes a solid is that lathe or revolve, and there is no solid
// before it. Otherwise nil.
func (sk *Sketch) revolutionAt(g *Group) *topo.Revolution {
	for ; g != nil; g = g.PreviousGroup(sk) {
		if g.Suppress || g.SuppressedBy.V != 0 {
			continue
		}
		if g.IsMeshGroup() || g.IsCopyGroup() || g.Type == GroupLinked {
			break
		}
	}
	if g == nil || (g.Type != GroupLathe && g.Type != GroupRevolve) || g.MeshCombine != CombineUnion {
		return nil
	}
	if before := g.PreviousGroup(sk); before != nil &&
		(len(before.RunningShell.Surface) > 0 || len(before.RunningMesh.Triangles) > 0) {
		return nil
	}
	src := sk.Group(g.OpA)
	if src == nil || len(src.BezierLoops.Sets) == 0 {
		return nil
	}
	pt, axis := g.revolveAxis(sk)
	angles, anglef, whole := g.revolveAngles(sk)
	if whole {
		angles, anglef = 0, 2*math.Pi
	}
	return &topo.Revolution{Profile: &src.BezierLoops, Pt: pt, Axis: axis, Start: angles, End: anglef}
}

// generateShellShell hollows the solid so far into the group's running
// shell; when it can't, the solid passes through and the group says
// why.
func (g *Group) generateShellShell(sk *Sketch) {
	prev := g.PreviousGroup(sk)
	if prev != nil {
		g.RunningShell.MakeFromCopyOf(&prev.RunningShell)
	}
	switch {
	case len(g.SolidFaces) == 0:
		g.setProblem(nil, "the shell takes no face away")
		return
	case g.ValB <= 0:
		g.setProblem(nil, "a shell's thickness must be more than zero")
		return
	case prev == nil || len(prev.RunningShell.Surface) == 0:
		g.setProblem(nil, "there is no solid with surfaces to hollow")
		return
	case prev.IsForcedToMesh(sk):
		g.setProblem(nil, "the solid so far is a triangle mesh, which has no faces to move")
		return
	}
	sh, err := shellOf(&prev.RunningShell, g.SolidFaces, g.ValB, srf.OriginFor(g.H.V, 0), sk.revolutionAt(prev))
	if err != nil {
		if se, ok := errors.AsType[*topo.ShellError](err); ok {
			at := se.At
			g.setProblem(&at, "%v", err)
		} else {
			g.setProblem(nil, "%v", err)
		}
		return
	}
	g.RunningShell = *sh
	g.ThisShell.MakeFromCopyOf(sh)
}
