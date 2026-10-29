package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A solid joined from lathes, revolves and extrusions, each of which a
// shell can hollow alone, is hollowed as they were joined
// (topo.ShellJoined): a turned body with an extruded boss, a cup with a
// revolved lug, a block with a round boss, a block with a round pocket
// cut into it or a lathed groove round it, a ball cut flat by a block it
// is intersected with. Lathes of whole turns about the first one's axis
// joined by unions before any part is taken away or intersected with are
// joined into one revolution by their profiles first
// (topo.JoinedRevolution), as their faces touch where they meet.

// unionPartsAt is the parts the solid at group g is made of, when every
// group up to it that makes a solid is a lathe, a revolve or an
// extrusion with no draft, the first joined by a union and the rest by a
// union, a difference or an intersection: revolutions for the lathes and
// revolves, the extrusions' own solids. Otherwise nil.
func (sk *Sketch) unionPartsAt(g *Group) []topo.ShellPart {
	if g == nil {
		return nil
	}
	var parts []topo.ShellPart
	var turned []topo.RevolvedPart // whole turns about the first one's axis
	turnedAt := -1                 // where in parts they go
	cut := false                   // whether a part so far is taken away or intersected with
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
		var op srf.SurfaceCombineAs
		switch cur.MeshCombine {
		case CombineUnion:
			op = srf.SurfaceUnion
		case CombineDifference:
			op = srf.SurfaceDiff
		case CombineIntersection:
			op = srf.SurfaceInter
		}
		switch {
		case cur.Type == GroupLathe || cur.Type == GroupRevolve:
			rev := sk.revolutionOf(cur)
			if op == 0 || rev == nil || (len(parts) == 0 && len(turned) == 0 && op != srf.SurfaceUnion) {
				return nil
			}
			if rev.Whole() && op == srf.SurfaceUnion && !cut {
				if len(turned) == 0 {
					pt, axis = rev.Pt, rev.Axis.WithMagnitude(1)
				}
				if axis.Cross(rev.Axis.WithMagnitude(1)).Magnitude() < 1e-9 && rev.Pt.DistanceToLine(pt, axis) < geom.LengthEps {
					if turnedAt < 0 {
						turnedAt = len(parts)
						parts = append(parts, topo.ShellPart{})
					}
					turned = append(turned, topo.RevolvedPart{Profile: rev.Profile, Combine: srf.SurfaceUnion})
					break
				}
			}
			parts = append(parts, topo.ShellPart{Rev: rev, Combine: op})
			cut = cut || op != srf.SurfaceUnion
		case cur.Type == GroupExtrude && !cur.HasDraft():
			if op == 0 || len(cur.ThisShell.Surface) == 0 || (len(parts) == 0 && op != srf.SurfaceUnion) {
				return nil
			}
			parts = append(parts, topo.ShellPart{Shell: &cur.ThisShell, Combine: op})
			cut = cut || op != srf.SurfaceUnion
		case cur.IsMeshGroup() || cur.IsCopyGroup() || cur.Type == GroupLinked ||
			len(cur.ThisShell.Surface) > 0 || len(cur.ThisMesh.Triangles) > 0:
			return nil
		}
		if cur.H == g.H {
			break
		}
	}
	switch len(turned) {
	case 0:
	case 1:
		parts[turnedAt] = topo.ShellPart{Rev: &topo.Revolution{Profile: turned[0].Profile, Pt: pt, Axis: axis, Start: 0, End: 2 * math.Pi}}
	default:
		rev, err := topo.JoinedRevolution(pt, axis, turned, sk.tol)
		if err != nil {
			return nil
		}
		parts[turnedAt] = topo.ShellPart{Rev: &rev}
	}
	if len(parts) < 2 {
		return nil
	}
	return parts
}
