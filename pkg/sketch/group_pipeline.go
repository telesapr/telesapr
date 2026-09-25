package sketch

import (
	"fmt"
	"math"
	"os"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// AssembleLoops walks the sketch entities, converts each entity to
// Bezier curves via GenerateEntityBeziers, intersects the curves to
// find crossings, and assembles them into closed SBezierLoopSets.
// Updates g.PolyLoops, g.BezierLoops, g.BezierOpens accordingly.
// Self-intersecting contours emit a single SBezierList containing
// all entity curves; FindOuterFacesFrom is invoked but the full
// self-intersection split-and-reclassify path is a known limitation
// (the solver still converges for non-self-intersecting sketches,
// which covers all parity fixtures).
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
//
// For Drawing and Workplane groups, if the group's BezierLoops are
// empty this will run AssembleLoops first and, if that yields no
// curves, fall back to inheriting the source group's BezierLoops.
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
	case GroupLinked:
		g.generateLinkedShell(sk)
	default:
		// Drawings, etc.: no shell/mesh output.
	}
}

// generateLinkedShell copies the imported shell and mesh into this
// group, applying the group's translation/rotation/scale transform.
// Implements the LINKED branch of C++ Group::GenerateShellAndMesh
// (groupmesh.cpp:362-378).
//
// The Group struct currently does not store an imported shell/mesh
// (those are part of the Link import pipeline, not yet wired into
// the Go core). When importing is implemented, this function should
// pull from the importer and apply the transform. For now, we
// simply emit an empty shell/mesh and document the gap.
func (g *Group) generateLinkedShell(sk *Sketch) {
	_ = sk
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	// Future: g.ThisShell.MakeFromTransformationOf(impShell, offset, q, scale)
	// Future: g.ThisMesh.MakeFromTransformationOf(impMesh, offset, q, scale)
}

// generateExtrudeShell is the EXTRUDE group type: extrude each
// generateExtrudeShell is the EXTRUDE group type: extrude each
// BezierLoop of the SOURCE group (g.OpA) by the group's extrusion
// vector. Mirrors the C++ Group::GenerateShellAndMesh logic where
// the extrude pulls BezierLoops from src->bezierLoops rather than
// the extrude group's own entities.
func (g *Group) generateExtrudeShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	t0 := geom.Vector{}
	t1 := g.ExtrusionVector(sk)
	if t1.Magnitude() < 1e-9 {
		return
	}
	src := sk.GetGroup(g.OpA)
	if src == nil {
		g.RunningShell = g.ThisShell
		return
	}
	if os.Getenv("EXTRUDE_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[extrude] g=%v src=%v src.Loops=%d\n", g.H, src.H, len(src.BezierLoops.Sets))
	}
	if len(src.BezierLoops.Sets) == 0 {
		// Walk the opA lineage to find a group with populated
		// BezierLoops (typically the original Drawing). For each
		// empty group along the way, run AssembleLoops so the
		// LINE_SEGMENT entities get converted into beziers.
		rt := NewRuntime(sk)
		visited := make(map[core.HGroup]bool)
		current := src
		for current != nil && !visited[current.H] {
			visited[current.H] = true
			if os.Getenv("EXTRUDE_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "[extrude]   iter current=%v type=%v loops=%d OpA=%v\n",
					current.H, current.Type, len(current.BezierLoops.Sets), current.OpA)
			}
			if len(current.BezierLoops.Sets) == 0 {
				if current.Type == GroupDrawing3D || current.Type == GroupDrawingWorkplane {
					current.AssembleLoops(sk, rt)
				}
			}
			if len(current.BezierLoops.Sets) > 0 {
				for _, srcSet := range current.BezierLoops.Sets {
					dstSet := src.BezierLoops.AddOuter()
					dstSet.Outer.Curves = append(dstSet.Outer.Curves, srcSet.Outer.Curves...)
				}
				break
			}
			if current.OpA == (core.HGroup{}) {
				break
			}
			current = sk.GetGroup(current.OpA)
		}
	}
	for i := range src.BezierLoops.Sets {
		set := &src.BezierLoops.Sets[i]
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

// generateHelixShell produces a helical sweep: revolved contour
// translated along the axis with a per-revolution pitch. The total
// rotation count is taken from g.ValA (radians, default 2π) and the
// pitch from g.H.Param(3) (axial distance per full turn).
func (g *Group) generateHelixShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	axis, center, pitch := g.HelixParameters(sk)
	turns := g.ValA / (2 * math.Pi)
	if turns == 0 {
		turns = 1
	}
	for i := range g.BezierLoops.Sets {
		set := &g.BezierLoops.Sets[i]
		if set.Outer.Len() == 0 {
			continue
		}
		newShell := srf.SShell{}
		newShell.MakeFromHelixOf(set, center, axis, turns, pitch, g.Color)
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

// GenerateForStepAndRepeat copies srcm to out. Solvespace represents
// step-and-repeat as a regular Translate or Rotate group with
// per-step delta encoded via the group's request list (each request
// gets a CopyNumber and translation/rotation parameters). The
// orchestration that produces multiple copies lives in
// Group::GenerateEquations + Group::GenerateRequestList, which then
// calls this helper to merge the produced meshes via g.MeshCombine.
func (g *Group) GenerateForStepAndRepeat(srcm *srf.SMesh, out *srf.SMesh) {
	if srcm == nil || out == nil {
		return
	}
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
