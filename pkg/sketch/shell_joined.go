package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
	"github.com/telesapr/telesapr/pkg/topo"
)

// A solid joined from lathes, revolves and extrusions by unions, each of
// which a shell can hollow alone, is hollowed as their union
// (topo.ShellJoined): a turned body with an extruded boss, a cup with a
// revolved lug, a block with a round boss. Lathes of whole turns about
// the first one's axis are joined into one revolution by their profiles
// first (topo.JoinedRevolution), as their faces touch where they meet.

// unionPartsAt is the parts the solid at group g is the union of, when
// every group up to it that makes a solid is a lathe, a revolve or an
// extrusion with no draft, joined to the solid before by a union:
// revolutions for the lathes and revolves, the extrusions' own solids.
// Otherwise nil.
func (sk *Sketch) unionPartsAt(g *Group) []topo.ShellPart {
	if g == nil {
		return nil
	}
	var parts []topo.ShellPart
	var turned []topo.RevolvedPart // whole turns about the first one's axis
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
		switch {
		case cur.Type == GroupLathe || cur.Type == GroupRevolve:
			if cur.MeshCombine != CombineUnion {
				return nil
			}
			rev := sk.revolutionOf(cur)
			if rev == nil {
				return nil
			}
			if rev.Whole() {
				if len(turned) == 0 {
					pt, axis = rev.Pt, rev.Axis.WithMagnitude(1)
				}
				if axis.Cross(rev.Axis.WithMagnitude(1)).Magnitude() < 1e-9 && rev.Pt.DistanceToLine(pt, axis) < geom.LengthEps {
					turned = append(turned, topo.RevolvedPart{Profile: rev.Profile, Combine: srf.SurfaceUnion})
					break
				}
			}
			parts = append(parts, topo.ShellPart{Rev: rev})
		case cur.Type == GroupExtrude && !cur.HasDraft():
			if cur.MeshCombine != CombineUnion || len(cur.ThisShell.Surface) == 0 {
				return nil
			}
			parts = append(parts, topo.ShellPart{Shell: &cur.ThisShell})
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
		parts = append(parts, topo.ShellPart{Rev: &topo.Revolution{Profile: turned[0].Profile, Pt: pt, Axis: axis, Start: 0, End: 2 * math.Pi}})
	default:
		rev, err := topo.JoinedRevolution(pt, axis, turned, sk.tol)
		if err != nil {
			return nil
		}
		parts = append(parts, topo.ShellPart{Rev: &rev})
	}
	if len(parts) < 2 {
		return nil
	}
	return parts
}
