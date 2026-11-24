package step

import (
	"slices"

	"github.com/telesapr/telesapr/pkg/srf"
)

// The faces of a solid are tessellated one by one, so their meshes can
// fail to meet where the file's edges or trims are off, as a sphere's
// pole on a box's edge did (fixed 2026-11-23) with no word said. Each
// solid's mesh is checked as Check Solid checks a group's: edges with no
// mate, which leave it open, and edges through a triangle, where it
// crosses itself.

// solidKinds are the shape roots that are closed: a shell or a surface
// set may be open, and is not checked.
var solidKinds = []string{"MANIFOLD_SOLID_BREP", "BREP_WITH_VOIDS", "FACETED_BREP", "TESSELLATED_SOLID"}

// checkMesh counts into rep the naked and self-intersecting edges of a
// solid's mesh, in its own space, so that an assembly's parts touching
// one another don't count. A solid with faces left out is not checked:
// its mesh is open, and the report already says why.
func checkMesh(sh *Shape, sm shapeMesh, skipped bool, rep *Report) {
	if skipped || !slices.Contains(solidKinds, sh.Kind) {
		return
	}
	var m srf.SMesh
	for _, tris := range sm.faces {
		for _, t := range tris {
			m.AddTriangle(srf.STriangle{A: t.p[0], B: t.p[1], C: t.p[2]})
		}
	}
	if len(m.Triangles) == 0 {
		return
	}
	// As FromModel snaps the placed mesh.
	m.SnapTJunctions()
	root := srf.SKdNodeFrom(&m)
	var bad, inter srf.SEdgeList
	var crossed, leaky bool
	root.MakeCertainEdgesInto(&bad, srf.EdgeNakedOrSelfInter, false, &crossed, &leaky, 0)
	if !crossed && !leaky {
		return
	}
	n := len(bad.Edges)
	if crossed {
		root.MakeCertainEdgesInto(&inter, srf.EdgeSelfInter, false, nil, nil, 0)
		n -= len(inter.Edges)
		rep.SelfIntersecting += len(inter.Edges)
	}
	rep.NakedEdges += n
	rep.BrokenSolids++
}
