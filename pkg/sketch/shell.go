package sketch

import (
	"errors"
	"fmt"
	"math"

	"github.com/spacesolve/spacesolve/pkg/srf"
	"github.com/spacesolve/spacesolve/pkg/topo"
)

// A shell hollows the solid so far to walls of its thickness, ValB,
// open where its faces, SolidFaces, are taken away (topo.ShellSolid):
// solids of flat faces in this version. It works on a copy of the solid
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
	if _, err := shellOf(&solid.RunningShell, faces, thick, 1<<30); err != nil {
		return g, err
	}
	g.SolidFaces = append([]srf.FaceID(nil), faces...)
	return g, nil
}

// shellOf hollows sh, taking away the faces with identities faces.
func shellOf(sh *srf.SShell, faces []srf.FaceID, thick float64, origin uint32) (*srf.SShell, error) {
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
	return topo.ShellSolid(sh, open, thick, origin)
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
	sh, err := shellOf(&prev.RunningShell, g.SolidFaces, g.ValB, srf.OriginFor(g.H.V, 0))
	if err != nil {
		var se *topo.ShellError
		if errors.As(err, &se) {
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
