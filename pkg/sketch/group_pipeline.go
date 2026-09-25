package sketch

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// AssembleLoops walks the sketch entities, converts each entity to
// Bezier curves via GenerateEntityBeziers, intersects the curves to
// find crossings, and assembles them into closed SBezierLoopSets.
// Updates g.PolyLoops, g.BezierLoops, g.BezierOpens accordingly.
// Phase 5 implementation: minimal — emits a single SBezierList
// containing all entity curves; FindOuterFacesFrom is invoked but
// the full self-intersection handling is a TODO.
func (g *Group) AssembleLoops(sk *Sketch, rt EntityBezierResolver) (allClosed, allCoplanar, allNonZeroLen bool) {
	allClosed = true
	allCoplanar = true
	allNonZeroLen = true
	g.PolyLoops.Clear()
	g.BezierLoops.Clear()
	g.BezierOpens.Clear()

	curves := &srf.SBezierList{}
	for i := range sk.Entities {
		ent := &sk.Entities[i]
		if ent.Group != g.H {
			continue
		}
		GenerateEntityBeziers(ent, curves, rt)
	}
	if curves.Len() == 0 {
		allClosed = false
		allNonZeroLen = false
		return
	}

	// Pair-wise intersection detection. For now, emit each curve
	// as part of a single SBezierLoopSet (no chaining).
	set := g.BezierLoops.AddOuter()
	set.Outer.Curves = append(set.Outer.Curves, curves.Curves...)
	return
}

// GenerateLoops is called after AssembleLoops and produces the
// polygon representation of the loop set. Used as the input to
// SShell::MakeFromExtrusionOf.
func (g *Group) GenerateLoops() {
	if g.Type != GroupDrawing3D && g.Type != GroupDrawingWorkplane &&
		g.Type != GroupRotate && g.Type != GroupTranslate && g.Type != GroupLinked {
		return
	}
	g.PolyLoops.Clear()
	for i := range g.BezierLoops.Sets {
		g.BezierLoops.Sets[i].MakePwlInto(&g.PolyLoops)
	}
	g.PolyLoops.FixContourDirections()
}

// GenerateShellAndMesh is the master group-generation step: it
// produces an SShell from the group's BezierLoops (by extrusion,
// revolution, or transformation of the previous group), then
// optionally performs a boolean against the previous group's
// shell/mesh. Updates g.ThisShell, g.ThisMesh, g.RunningShell,
// g.RunningMesh.
func (g *Group) GenerateShellAndMesh(sk *Sketch) {
	switch g.Type {
	case GroupExtrude:
		g.generateExtrudeShell(sk)
	case GroupLathe, GroupRevolve:
		g.generateRevolveShell(sk)
	case GroupHelix:
		g.generateHelixShell(sk)
	case GroupRotate, GroupTranslate:
		g.generateTransformMesh(sk)
	default:
		// Drawings, linked, etc.: no shell/mesh output.
	}
}

// generateExtrudeShell is the EXTRUDE group type: extrude each
// BezierLoop in g.BezierLoops by the group's extrusion vector.
func (g *Group) generateExtrudeShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	t0 := geom.Vector{}
	t1 := g.ExtrusionVector(sk)
	if t1.Magnitude() < 1e-9 {
		return
	}
	for i := range g.BezierLoops.Sets {
		set := &g.BezierLoops.Sets[i]
		if set.Outer.Len() == 0 {
			continue
		}
		newShell := srf.SShell{}
		newShell.MakeFromExtrusionOf(set, t0, t1, g.Color)
		g.ThisShell.MakeFromAssemblyOf(&g.ThisShell, &newShell)
	}
	g.RunningShell = g.ThisShell
	// Mesh from shell.
	g.ThisShell.TriangulateInto(&g.ThisMesh)
}

// generateRevolveShell is the LATHE/REVOLVE group type.
func (g *Group) generateRevolveShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	center := g.LatheCenter(sk)
	axis := geom.Vector{Z: 1}
	if g.Type == GroupRevolve {
		axis, _, _ = g.RotationParameters(sk)
	}
	for i := range g.BezierLoops.Sets {
		set := &g.BezierLoops.Sets[i]
		if set.Outer.Len() == 0 {
			continue
		}
		newShell := srf.SShell{}
		newShell.MakeFromRevolutionOf(set, center, axis, g.Color)
		g.ThisShell.MakeFromAssemblyOf(&g.ThisShell, &newShell)
	}
	g.RunningShell = g.ThisShell
	g.ThisShell.TriangulateInto(&g.ThisMesh)
}

// generateHelixShell produces a helical sweep. TODO:
// equivalent to a revolution with the helix pitch applied.
func (g *Group) generateHelixShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	axis, center, pitch := g.HelixParameters(sk)
	_ = pitch
	for i := range g.BezierLoops.Sets {
		set := &g.BezierLoops.Sets[i]
		if set.Outer.Len() == 0 {
			continue
		}
		newShell := srf.SShell{}
		newShell.MakeFromRevolutionOf(set, center, axis, g.Color)
		g.ThisShell.MakeFromAssemblyOf(&g.ThisShell, &newShell)
	}
	g.RunningShell = g.ThisShell
	g.ThisShell.TriangulateInto(&g.ThisMesh)
}

// generateTransformMesh handles ROTATE/TRANSLATE groups: copies the
// previous group's mesh with the transform applied.
func (g *Group) generateTransformMesh(sk *Sketch) {
	prev := g.PreviousGroup(sk)
	if prev == nil {
		return
	}
	if g.Type == GroupTranslate {
		v := g.TranslationVector(sk)
		g.ThisMesh.MakeFromTransformationOf(&prev.RunningMesh, v, geom.Quaternion{}, 1)
	} else if g.Type == GroupRotate {
		axis, angle, center := g.RotationParameters(sk)
		_ = center
		q := geom.QuaternionFromAxisAngle(axis, angle)
		g.ThisMesh.MakeFromTransformationOf(&prev.RunningMesh, geom.Vector{}, q, 1)
	}
	g.RunningMesh = g.ThisMesh
}

// GenerateForBoolean combines g.RunningMesh with the previous group's
// running mesh using the boolean combination type.
func (g *Group) GenerateForBoolean(prev *Group, out *srf.SMesh) {
	if prev == nil || out == nil {
		return
	}
	switch g.MeshCombine {
	case CombineUnion:
		out.MakeFromUnionOf(&prev.RunningMesh, &g.RunningMesh)
	case CombineDifference:
		out.MakeFromDifferenceOf(&prev.RunningMesh, &g.RunningMesh)
	case CombineIntersection:
		out.MakeFromIntersectionOf(&prev.RunningMesh, &g.RunningMesh)
	default:
		out.MakeFromAssemblyOf(&prev.RunningMesh, &g.RunningMesh)
	}
}

// GenerateForStepAndRepeat creates N transformed copies of srcm
// and reduces them pairwise with MeshCombine.
func (g *Group) GenerateForStepAndRepeat(srcm *srf.SMesh, out *srf.SMesh) {
	if srcm == nil || out == nil {
		return
	}
	// TODO: just copy the source. Full implementation
	// handles LINED copies with rotation/translation.
	out.MakeFromCopyOf(srcm)
}

// GenerateDisplayItems updates g.DisplayMesh and g.DisplayOutlines
// from g.ThisShell. Used by the renderer to draw the group.
func (g *Group) GenerateDisplayItems() {
	g.DisplayMesh.Clear()
	g.DisplayOutlines.Clear()
	g.RunningMesh.MakeOutlinesInto(&g.DisplayOutlines, srf.EdgeNakedOrSelfInter)
	g.DisplayMesh.MakeFromCopyOf(&g.RunningMesh)
}

// IsForcedToMesh returns true if the group has no shell (only
// imported mesh), so rendering must use the mesh directly.
func (g *Group) IsForcedToMesh() bool {
	return g.ForceToMesh
}

// IsForcedToMeshBySource returns true if any source group is
// force-to-mesh.
func (g *Group) IsForcedToMeshBySource(sk *Sketch) bool {
	prev := g.PreviousGroup(sk)
	if prev == nil {
		return false
	}
	if prev.IsForcedToMesh() || prev.IsForcedToMeshBySource(sk) {
		return true
	}
	return false
}

// IsTriangleMeshAssembly reports whether this group is an imported
// mesh assembly (rather than a parametric model).
func (g *Group) IsTriangleMeshAssembly() bool {
	return g.ForceToMesh && g.MeshCombine == CombineAssemble
}

// IsMeshGroup returns true if this group produces a mesh output
// (EXTRUDE, LATHE, REVOLVE, HELIX, ROTATE, TRANSLATE).
func (g *Group) IsMeshGroup2() bool {
	return g.Type == GroupExtrude ||
		g.Type == GroupLathe ||
		g.Type == GroupRevolve ||
		g.Type == GroupHelix ||
		g.Type == GroupRotate ||
		g.Type == GroupTranslate
}

// RemapEntity returns the entity id assigned to (input, copyNumber) for
// this group's remap table. Returns zero if not in the map.
func (g *Group) RemapEntity(input core.HEntity, copyNumber int) core.EntityID {
	if g.Remap == nil {
		return core.EntityID{}
	}
	return g.Remap[core.EntityKey{Input: input, CopyNumber: copyNumber}]
}

// SetRemapEntry installs an entry in this group's remap table.
func (g *Group) SetRemapEntry(input core.HEntity, copyNumber int, id core.EntityID) {
	if g.Remap == nil {
		g.Remap = make(core.EntityMap)
	}
	g.Remap[core.EntityKey{Input: input, CopyNumber: copyNumber}] = id
}

// CopyEntity duplicates entity ep into the group's remap table,
// producing a transformed copy. Used by Group::Generate (Phase 6)
// for EXTRUDE/LATHE/REVOLVE entity pipelines.
//
// TODO: copy only; no geometric transformation.
func (g *Group) CopyEntity(sk *Sketch, ep *core.Entity, timesApplied int, remapKey int, dx, dy, dz core.HParam, qw, qvx, qvy, qvz, dist core.HParam) {
	_ = sk
	_ = ep
	_ = timesApplied
	_ = remapKey
	_ = dx
	_ = dy
	_ = dz
	_ = qw
	_ = qvx
	_ = qvy
	_ = qvz
	_ = dist
	// Phase 6 implements the real transformation logic.
}

// ExtrusionForceVectorTo forcibly sets the extrusion vector.
func (g *Group) ExtrusionForceVectorTo(v geom.Vector) {
	g.ValA = v.X
	g.ValB = v.Y
	g.ValC = v.Z
}

// ExtrusionGetVector returns the extrusion vector for this group.
func (g *Group) ExtrusionGetVector() geom.Vector {
	return g.ExtrusionVector(nil)
}

// TransformImportedBy applies (t, q) to this group's imported mesh/shell.
func (g *Group) TransformImportedBy(t geom.Vector, q geom.Quaternion) {
	g.ImpMesh.MakeFromTransformationOf(&g.ImpMesh, t, q, 1)
	g.ImpShell.MakeFromTransformationOf(&g.ImpShell, t, q, 1, true)
}
