package sketch

import (
	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// An assembly's part with no shell, made of triangles (a group forced to
// a mesh, a linked mesh, or the solid before the assembly made so), is
// cut by the groups that cut the assembly after it as a part with a
// shell is (cutParts), by the triangles' Boolean the model's own mesh
// is combined by: a hole cut through a linked mesh after it was placed
// is a hole in that part for Clearances, Wall Thickness and Mass. A
// cutting group made of triangles cuts the parts with shells it reaches
// so too, which are then parts of triangles.

// meshPart is a part of an assembly with no shell: its group and its
// triangles.
type meshPart struct {
	g    core.HGroup
	mesh srf.SMesh
}

// newMeshPart is the part of group hg made of a copy of m's triangles
// and those of sh, where a group forced to a mesh still holds its own.
func newMeshPart(hg core.HGroup, m *srf.SMesh, sh *srf.SShell) *meshPart {
	mp := &meshPart{g: hg}
	mp.mesh.MakeFromCopyOf(m)
	if sh != nil {
		sh.TriangulateInto(&mp.mesh)
	}
	return mp
}

// cutAssembly cuts the parts and the parts of triangles by g, a group
// cutting the assembly after them, and is them after.
func cutAssembly(parts []*part, meshes []*meshPart, g *Group) ([]*part, []*meshPart) {
	if len(g.ThisShell.Surface) > 0 {
		cutParts(parts, g)
	} else if !g.ThisMesh.IsEmpty() {
		// The tool has no shell to cut a shell by: the parts it changes
		// are cut as triangles.
		var kept []*part
		lo, hi := meshBox(&g.ThisMesh)
		for _, p := range parts {
			if g.MeshCombine != CombineDifference || boxesMeet(p.lo, p.hi, lo, hi) {
				meshes = append(meshes, newMeshPart(p.g, &srf.SMesh{}, p.shell))
			} else {
				kept = append(kept, p)
			}
		}
		parts = kept
	}
	var tool srf.SMesh
	tool.MakeFromCopyOf(&g.ThisMesh)
	g.ThisShell.TriangulateInto(&tool)
	if tool.IsEmpty() {
		return parts, meshes
	}
	lo, hi := meshBox(&tool)
	for _, mp := range meshes {
		plo, phi := meshBox(&mp.mesh)
		meets := boxesMeet(plo, phi, lo, hi)
		var out srf.SMesh
		switch {
		case g.MeshCombine == CombineDifference && !meets:
			continue
		case g.MeshCombine == CombineDifference:
			out.MakeFromDifferenceOf(&mp.mesh, &tool)
		case meets:
			out.MakeFromIntersectionOf(&mp.mesh, &tool)
		}
		mp.mesh = out
	}
	return parts, meshes
}

// meshBox is the box about m's triangles.
func meshBox(m *srf.SMesh) (lo, hi geom.Vector) {
	hi, lo = m.Bounding()
	return lo, hi
}
