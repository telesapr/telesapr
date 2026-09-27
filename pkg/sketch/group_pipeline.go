package sketch

import (
	"fmt"
	"math"
	"os"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// AssembleLoops converts this group's visible, non-construction
// entities into Bezier curves and assembles them into closed loops:
// the outer loops grouped with their holes go into g.BezierLoops (and
// their pwl into g.PolyLoops), the leftovers into g.BezierOpens.
// Mirrors C++ Group::AssembleLoops (groupmesh.cpp:12).
func (g *Group) AssembleLoops(sk *Sketch, rt EntityBezierResolver) (allClosed, allCoplanar, allNonZeroLen bool) {
	var sbl srf.SBezierList

	for i := range sk.Entities {
		e := &sk.Entities[i]
		if e.Group != g.H {
			continue
		}
		if e.Construction {
			continue
		}
		if e.ForceHidden {
			continue
		}

		GenerateEntityBeziers(e, &sbl, rt)
	}

	allNonZeroLen = true
	for _, sb := range sbl.Curves {
		i := 1
		for ; i <= sb.Degree; i++ {
			if !sb.Ctrl[i].Equals(sb.Ctrl[0]) {
				break
			}
		}
		if i > sb.Degree {
			// This is a zero-length edge.
			allNonZeroLen = false
			g.PolyError.ErrorPoint = sb.Ctrl[0]
			return
		}
	}

	// Try to assemble all these Beziers into loops. The closed loops go into
	// bezierLoops, with the outer loops grouped with their holes. The
	// leftovers, if any, go in bezierOpens.
	g.BezierLoops.FindOuterFacesFrom(&sbl, &g.PolyLoops, nil,
		srf.ChordTolMm(),
		&allClosed, &g.PolyError.NotClosed,
		&allCoplanar, &g.PolyError.ErrorPoint,
		&g.BezierOpens)
	return
}

// GenerateLoops rebuilds this group's loops: for sketch-like groups
// it assembles them from the group's entities and records the
// polygon error, otherwise it just clears them. Mirrors C++
// Group::GenerateLoops (groupmesh.cpp:58). The self-intersection check
// is skipped, as it is by default in C++ (SS.checkClosedContour).
func (g *Group) GenerateLoops(sk *Sketch, rt EntityBezierResolver) {
	g.PolyLoops.Clear()
	g.BezierLoops.Clear()
	g.BezierOpens = nil

	if g.Type == GroupDrawing3D || g.Type == GroupDrawingWorkplane ||
		g.Type == GroupRotate || g.Type == GroupTranslate || g.Type == GroupLinked {
		allClosed, allCoplanar, allNonZeroLen := g.AssembleLoops(sk, rt)
		switch {
		case !allNonZeroLen:
			g.PolyError.How = int(core.PolyZeroLenEdge)
		case !allCoplanar:
			g.PolyError.How = int(core.PolyNotCoplanar)
		case !allClosed:
			g.PolyError.How = int(core.PolyNotClosed)
		default:
			g.PolyError.How = int(core.PolyGood)
		}
	}
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
		// Drawings, etc.: no shell/mesh output of their own. Mirror
		// the C++ behavior in GenerateForBoolean's "if empty, copy
		// prevs" branch by propagating the previous mesh group's
		// running mesh through, so the next mesh group's boolean
		// combine sees the right operand.
		g.propagateFromPreviousMeshGroup(sk)
	}
}

// propagateFromPreviousMeshGroup walks back through previous groups
// to find the last mesh group and copies its runningMesh into this
// group. Mirrors the C++ GenerateForBoolean early-exit for empty
// `thiss` shells.
func (g *Group) propagateFromPreviousMeshGroup(sk *Sketch) {
	prev := g.PreviousGroup(sk)
	for prev != nil && !prev.IsMeshGroup2() {
		prev = prev.PreviousGroup(sk)
	}
	if prev == nil {
		return
	}
	g.RunningMesh.Clear()
	g.RunningMesh.MakeFromCopyOf(&prev.RunningMesh)
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
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	if sk == nil {
		return
	}
	// Read the LINKED transform parameters: 0..2 are the offset,
	// 3..6 are the quaternion.
	offset := geom.Vector{
		X: paramFloat(sk, g.H.Param(0)),
		Y: paramFloat(sk, g.H.Param(1)),
		Z: paramFloat(sk, g.H.Param(2)),
	}
	q := geom.Quaternion{
		W: paramFloat(sk, g.H.Param(3)),
		X: paramFloat(sk, g.H.Param(4)),
		Y: paramFloat(sk, g.H.Param(5)),
		Z: paramFloat(sk, g.H.Param(6)),
	}
	scale := g.Scale
	// Apply transform to imported mesh/shell if present.
	if len(g.ImportedMesh.Triangles) > 0 {
		g.ThisMesh.MakeFromTransformationOf(&g.ImportedMesh, offset, q, scale)
	}
	if len(g.ImportedShell.Surface) > 0 || len(g.ImportedShell.Curve) > 0 {
		g.ThisShell.MakeFromTransformationOf(&g.ImportedShell, offset, q, scale, true)
	}
	// Note: SMesh/SShell RemapFacesViaGroup requires Group to implement
	// a Remap method that maps source entity handles to new ones.
	// That's a UI/link-import concern; we leave it as a no-op for now.
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
	// Match C++ Group::GenerateShellAndMesh extrusion convention:
	//   ONE_SIDED / ONE_SKEWED: tbot = 0, ttop = 2 * translate
	//   TWO_SIDED / TWO_SKEWED: tbot = -translate, ttop = translate
	translate := g.ExtrusionVector(sk)
	if translate.Magnitude() < 1e-9 {
		return
	}
	t0 := geom.Vector{}
	t1 := translate
	switch g.Subtype {
	case GroupOneSided, GroupOneSkewed:
		t1 = translate.ScaledBy(2)
	default:
		t0 = translate.ScaledBy(-1)
		t1 = translate
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
					dstSet.Outer.Tag = srcSet.Outer.Tag
					dstSet.Holes = append(dstSet.Holes, srcSet.Holes...)
					dstSet.Normal = srcSet.Normal
					dstSet.Point = srcSet.Point
					dstSet.Area = srcSet.Area
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
		// Extrude this outer contour (plus its inner contours, if present)
		g.ThisShell.MakeFromExtrusionOf(set, t0, t1, g.Color)
	}
	g.RunningShell = g.ThisShell
	// Mesh from shell.
	g.ThisShell.TriangulateInto(&g.ThisMesh)
}

// revolveAxis returns the point on and the unit direction of the
// axis of a LATHE/REVOLVE/HELIX group, from its predef origin point
// and axis vector entities.
func (g *Group) revolveAxis(sk *Sketch) (pt, axis geom.Vector) {
	pt = sk.MustGetEntity(g.Predef.Origin).PointGetNum(sk)
	axis = sk.MustGetEntity(g.Predef.EntityB).VectorGetNum(sk)
	return pt, axis.WithMagnitude(1)
}

// generateRevolveShell is the LATHE/REVOLVE group type: revolve each
// BezierLoopSet of the source group (g.OpA) about the group's axis, a
// full turn for LATHE, by the group's angle for REVOLVE. Mirrors the
// LATHE and REVOLVE branches of C++ Group::GenerateShellAndMesh
// (groupmesh.cpp:317-351).
func (g *Group) generateRevolveShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	src := sk.GetGroup(g.OpA)
	if src == nil {
		return
	}
	pt, axis := g.revolveAxis(sk)

	anglef, angles := 2*math.Pi, 0.0
	if g.Type == GroupRevolve {
		anglef = paramFloat(sk, g.H.Param(3)) * 4 // why the 4 is needed?
		if g.Subtype != GroupOneSided {
			anglef *= 0.5
			angles = -anglef
		}
	}
	for i := range src.BezierLoops.Sets {
		sbls := &src.BezierLoops.Sets[i]
		if g.Type == GroupRevolve && math.Abs(anglef-angles) < 2*math.Pi {
			g.ThisShell.MakeFromHelicalRevolutionOf(sbls, pt, axis, g.Color,
				angles, anglef, 0, 0, 0, 0)
		} else {
			g.ThisShell.MakeFromRevolutionOf(sbls, pt, axis, g.Color)
		}
	}
	g.ThisShell.TriangulateInto(&g.ThisMesh)
}

// generateHelixShell is the HELIX group type: sweep each
// BezierLoopSet of the source group (g.OpA) about the group's axis
// while translating along it. Mirrors the HELIX branch of C++
// Group::GenerateShellAndMesh (groupmesh.cpp:352-374).
func (g *Group) generateHelixShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisMesh.Clear()
	src := sk.GetGroup(g.OpA)
	if src == nil {
		return
	}
	pt, axis := g.revolveAxis(sk)

	anglef := paramFloat(sk, g.H.Param(3)) * 4 // why the 4 is needed?
	angles := 0.0
	dists := 0.0
	distf := paramFloat(sk, g.H.Param(7)) * 2 // dist is applied twice
	if g.Subtype != GroupOneSided {
		anglef *= 0.5
		angles = -anglef
		distf *= 0.5
		dists = -distf
	}
	for i := range src.BezierLoops.Sets {
		g.ThisShell.MakeFromHelicalRevolutionOf(&src.BezierLoops.Sets[i], pt, axis, g.Color,
			angles, anglef, dists, distf, 0, 0)
	}
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

// combineWithRunningShell combines this group's shell (or, for a
// group forced to mesh, its mesh) with the previous mesh group's
// running shell using the group's MeshCombine, producing
// g.RunningShell and g.RunningMesh. Mirrors the tail of C++
// Group::GenerateShellAndMesh (groupmesh.cpp:394-448); the running
// mesh stands in for the displayMesh C++ triangulates from
// runningShell in GenerateDisplayItems.
func (g *Group) combineWithRunningShell(sk *Sketch) {
	if g.MeshCombine != CombineAssemble {
		g.ThisShell.MergeCoincidentSurfaces()
	}

	// So now we've got the mesh or shell for this group. Combine it with
	// the previous group's mesh or shell with the requested Boolean, and
	// we're done.
	prevg := findPreviousMeshGroup(g, sk)

	if !g.IsForcedToMesh() {
		var prevs srf.SShell
		if prevg != nil {
			prevs = prevg.RunningShell
		}
		var outs srf.SShell
		g.generateForBooleanShell(&prevs, &g.ThisShell, &outs)
		if g.MeshCombine != CombineAssemble {
			outs.MergeCoincidentSurfaces()
		}
		g.RunningShell = outs

		// If the Boolean failed, then we should note that in the text screen
		// for this group.
		g.BooleanFailed = g.RunningShell.BooleanFailed

		g.RunningMesh.Clear()
		g.RunningShell.TriangulateInto(&g.RunningMesh)
		return
	}

	g.RunningMesh = g.ThisMesh
	if prevg != nil && len(prevg.RunningMesh.Triangles) > 0 {
		var combined srf.SMesh
		g.GenerateForBoolean(prevg, &combined)
		g.RunningMesh = combined
	}
}

// generateForBooleanShell combines prevs and thiss into outs with the
// group's MeshCombine. Mirrors C++ Group::GenerateForBoolean<SShell>.
func (g *Group) generateForBooleanShell(prevs, thiss, outs *srf.SShell) {
	// If this group contributes no new mesh, then our running mesh is the
	// same as last time, no combining required. Likewise if we have a mesh
	// but it's suppressed.
	if len(thiss.Surface) == 0 || g.Suppress {
		outs.MakeFromCopyOf(prevs)
		return
	}

	// So our group's shell appears in thisShell. Combine this with the
	// previous group's shell, using the requested operation.
	switch g.MeshCombine {
	case CombineUnion:
		outs.MakeFromUnionOf(prevs, thiss)
	case CombineDifference:
		outs.MakeFromDifferenceOf(prevs, thiss)
	case CombineIntersection:
		outs.MakeFromIntersectionOf(prevs, thiss)
	case CombineAssemble:
		outs.MakeFromAssemblyOf(prevs, thiss)
	}
}

// GenerateForBoolean combines the previous group's running mesh with
// this group's running mesh using the boolean combination type. Uses
// the SShell-level CSG pipeline so coplanar faces are merged via
// MergeCoincidentSurfaces, matching the C++ boolean.cpp behavior.
func (g *Group) GenerateForBoolean(prev *Group, out *srf.SMesh) {
	if prev == nil || out == nil {
		return
	}
	// Convert the previous and current running meshes back to shells
	// so the SShell-level boolean CSG can be used. This is an
	// approximation: we lose the parametric surface representation
	// when we triangulated, so we can't regenerate the trimmed
	// bezier shell. As a fallback, we use the SMesh-level CSG
	// (triangle classification BSP).
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
	g.DisplayMesh.MakeFromCopyOf(&g.RunningMesh)
	// C++ outlines the triangulated shell's sharp edges (and then joins
	// them into polylines for display, which is out of scope here).
	g.DisplayMesh.MakeOutlinesInto(&g.DisplayOutlines, srf.EdgeSharp)
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

// annotateExtrudeFaces assigns face IDs to side surfaces of an
// extruded shell, mapping line-segment entities from the source
// group to the corresponding side face. Mirrors the face-id
// annotation loop in C++ Group::GenerateShellAndMesh (groupmesh.cpp).
//
// The top and bottom surfaces get REMAP_TOP / REMAP_BOTTOM face IDs.
// Side surfaces (degm=1, degn=1) are matched to line segments from
// the source group whose endpoints lie on the side's edges.
func (g *Group) annotateExtrudeFaces(sk *Sketch, shell *srf.SShell, tbot, ttop geom.Vector) {
	if sk == nil || shell == nil {
		return
	}
	src := sk.GetGroup(g.OpA)
	if src == nil {
		return
	}
	onOrig := geom.Vector{}
	for i := range shell.Surface {
		ss := &shell.Surface[i]
		if ss == nil {
			continue
		}
		p := ss.PointAt(0, 0)
		n := ss.NormalAt(0, 0).WithMagnitude(1)
		d := n.Dot(p)
		if math.Abs((onOrig.Plus(ttop)).Dot(n)-d) < 1e-9 {
			ss.Face = FaceIDTop
			continue
		}
		if math.Abs((onOrig.Plus(tbot)).Dot(n)-d) < 1e-9 {
			ss.Face = FaceIDBottom
			continue
		}
		if ss.DegM != 1 || ss.DegN != 1 {
			continue
		}
		for _, e := range sk.Entities {
			if e.Type != core.EntityLineSegment {
				continue
			}
			var a, b geom.Vector
			var okA, okB bool
			for j := range sk.Entities {
				if sk.Entities[j].H == e.Point[0] {
					a = sk.Entities[j].PointGetNum(sk)
					okA = true
				}
				if sk.Entities[j].H == e.Point[1] {
					b = sk.Entities[j].PointGetNum(sk)
					okB = true
				}
			}
			if !okA || !okB {
				continue
			}
			aT := a.Plus(ttop)
			bT := b.Plus(ttop)
			c00 := ss.Ctrl[0][0]
			c01 := ss.Ctrl[0][1]
			c10 := ss.Ctrl[1][0]
			c11 := ss.Ctrl[1][1]
			if (aT.Equals(c00) && bT.Equals(c10)) ||
				(bT.Equals(c00) && aT.Equals(c10)) ||
				(aT.Equals(c01) && bT.Equals(c11)) ||
				(bT.Equals(c01) && aT.Equals(c11)) {
				ss.Face = srf.FaceID(e.H.V)
				break
			}
		}
	}
}

// FaceIDTop and FaceIDBottom are placeholder face IDs for the top
// and bottom surfaces of an extrude.
const (
	FaceIDTop    srf.FaceID = 0xFFFFFFFE
	FaceIDBottom srf.FaceID = 0xFFFFFFFD
)

// paramFloat returns the float value of a param by handle, or 0 if missing.
// Used by generateLinkedShell to read the LINKED transform parameters.
func paramFloat(sk *Sketch, h core.HParam) float64 {
	if sk == nil {
		return 0
	}
	for i := range sk.Params {
		if sk.Params[i].H == h {
			return sk.Params[i].Val
		}
	}
	return 0
}
