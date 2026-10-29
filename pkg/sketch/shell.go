package sketch

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
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
	if _, err := shellOf(&solid.RunningShell, faces, thick, 1<<30, sk.madeAt(solid)); err != nil {
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
	if _, err := shellOf(&solid.RunningShell, faces, g.ValB, 1<<30, sk.madeAt(solid)); err != nil {
		return err
	}
	g.SolidFaces = faces
	sk.MarkGroupDirty(hg, false)
	return nil
}

// solidMade is how a solid was made, when a shell can hollow it by
// that: a revolution, a drafted extrusion, or parts a shell hollows
// alone, joined. All nil when none.
type solidMade struct {
	rev   *topo.Revolution
	draft *topo.DraftedSolid
	parts []topo.ShellPart
}

// madeAt is how the solid at group g was made (revolutionAt,
// draftedAt, unionPartsAt).
func (sk *Sketch) madeAt(g *Group) solidMade {
	return solidMade{rev: sk.revolutionAt(g), draft: sk.draftedAt(g), parts: sk.unionPartsAt(g)}
}

// shellOf hollows sh, taking away the faces with identities faces. A
// solid of flat faces or a prism is hollowed as such; failing that, by
// how it was made.
func shellOf(sh *srf.SShell, faces []srf.FaceID, thick float64, origin uint32, made solidMade) (*srf.SShell, error) {
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
	if err == nil {
		return out, nil
	}
	if made.rev != nil {
		out, rerr := topo.ShellRevolved(sh, *made.rev, open, thick, origin)
		if !errors.Is(rerr, topo.ErrNotRevolution) {
			return out, rerr
		}
	}
	if made.draft != nil {
		out, derr := topo.ShellDrafted(sh, *made.draft, open, thick, origin)
		if !errors.Is(derr, topo.ErrNotDrafted) {
			return out, derr
		}
	}
	if made.rev == nil && made.draft == nil && len(made.parts) > 1 {
		return topo.ShellJoined(sh, made.parts, open, thick, origin)
	}
	return nil, err
}

// revolutionAt is how the solid at group g was made when it is one
// lathe or revolve: the first group at or before g that makes or
// changes a solid is that lathe or revolve, and there is no solid
// before it; or when it is lathes about one axis joined
// (joinedRevolutionAt). Otherwise nil.
func (sk *Sketch) revolutionAt(g *Group) *topo.Revolution {
	if rev := sk.oneRevolutionAt(g); rev != nil {
		return rev
	}
	return sk.joinedRevolutionAt(g)
}

// oneRevolutionAt is how the solid at group g was made when it is one
// lathe or revolve, or nil.
func (sk *Sketch) oneRevolutionAt(g *Group) *topo.Revolution {
	g = sk.soleSolidAt(g)
	if g == nil || (g.Type != GroupLathe && g.Type != GroupRevolve) {
		return nil
	}
	return sk.revolutionOf(g)
}

// revolutionOf is how the lathe or revolve g turns its sketch, or nil
// when the sketch has no loops.
func (sk *Sketch) revolutionOf(g *Group) *topo.Revolution {
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

// soleSolidAt is the group that made the solid at group g when that
// group alone made it: the first group at or before g that makes or
// changes a solid, a union with no solid before it. Otherwise nil.
func (sk *Sketch) soleSolidAt(g *Group) *Group {
	for ; g != nil; g = g.PreviousGroup(sk) {
		if g.Suppress || g.SuppressedBy.V != 0 {
			continue
		}
		if g.IsMeshGroup() || g.IsCopyGroup() || g.Type == GroupLinked {
			break
		}
	}
	if g == nil || g.MeshCombine != CombineUnion {
		return nil
	}
	if before := g.PreviousGroup(sk); before != nil &&
		(len(before.RunningShell.Surface) > 0 || len(before.RunningMesh.Triangles) > 0) {
		return nil
	}
	return g
}

// joinedRevolutionAt is how the solid at group g was made when every
// group up to it that makes a solid is a lathe, or a revolve of a
// whole turn, about one axis, joined to the ones before by a union, a
// difference or an intersection: a solid of revolution whose profile
// is theirs joined (topo.JoinedRevolution). Otherwise nil.
func (sk *Sketch) joinedRevolutionAt(g *Group) *topo.Revolution {
	if g == nil {
		return nil
	}
	var parts []topo.RevolvedPart
	var pt, axis geom.Vector
	for _, gh := range sk.GroupOrder {
		cur := sk.Group(gh)
		if cur == nil {
			continue
		}
		if cur.Suppress || cur.SuppressedBy.V != 0 {
			if cur.H == g.H {
				break
			}
			continue
		}
		turn := cur.Type == GroupLathe || cur.Type == GroupRevolve
		if !turn {
			if cur.IsMeshGroup() || cur.IsCopyGroup() || cur.Type == GroupLinked ||
				len(cur.ThisShell.Surface) > 0 || len(cur.ThisMesh.Triangles) > 0 {
				return nil
			}
			if cur.H == g.H {
				break
			}
			continue
		}
		if _, _, whole := cur.revolveAngles(sk); !whole {
			return nil
		}
		src := sk.Group(cur.OpA)
		if src == nil || len(src.BezierLoops.Sets) == 0 {
			return nil
		}
		p, a := cur.revolveAxis(sk)
		var op srf.SurfaceCombineAs
		switch cur.MeshCombine {
		case CombineUnion:
			op = srf.SurfaceUnion
		case CombineDifference:
			op = srf.SurfaceDiff
		case CombineIntersection:
			op = srf.SurfaceInter
		default:
			return nil
		}
		if len(parts) == 0 {
			if op != srf.SurfaceUnion {
				return nil
			}
			pt, axis = p, a
		} else if axis.Cross(a).Magnitude() > 1e-9 || p.DistanceToLine(pt, axis) > geom.LengthEps {
			return nil
		}
		parts = append(parts, topo.RevolvedPart{Profile: &src.BezierLoops, Combine: op})
		if cur.H == g.H {
			break
		}
	}
	if len(parts) < 2 {
		return nil
	}
	rev, err := topo.JoinedRevolution(pt, axis, parts, sk.tol)
	if err != nil {
		return nil
	}
	return &rev
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
	sh, err := shellOf(&prev.RunningShell, g.SolidFaces, g.ValB, srf.OriginFor(g.H.V, 0), sk.madeAt(prev))
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
