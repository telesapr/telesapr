package sketch

import (
	"context"
	"math"
	"runtime/trace"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/internal/par"
	"github.com/telesapr/telesapr/pkg/libm"
	"github.com/telesapr/telesapr/pkg/srf"
)

// AssembleLoops converts this group's visible, non-construction
// entities into Bezier curves and assembles them into closed loops: the
// outer loops grouped with their holes go into g.BezierLoops (and their
// pwl into g.PolyLoops), the leftovers into g.BezierOpens.
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
		sk.tol,
		&allClosed, &g.PolyError.NotClosed,
		&allCoplanar, &g.PolyError.ErrorPoint,
		&g.BezierOpens)
	return
}

// GenerateLoops rebuilds this group's loops: for sketch-like groups it
// assembles them from the group's entities and records the polygon
// error, otherwise it just clears them. The self-intersection check is
// slow, so it runs only with the sketch's CheckClosedContour.
func (g *Group) GenerateLoops(sk *Sketch, rt EntityBezierResolver) {
	defer sk.timed(g.H, timingLoops)()
	g.PolyLoops.Clear()
	g.BezierLoops.Clear()
	g.BezierOpens = nil

	if g.Type == GroupDrawing3D || g.Type == GroupDrawingWorkplane ||
		g.IsCopyGroup() || g.Type == GroupLinked {
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
			// The self-intersecting check is kind of slow, so don't run
			// it unless requested.
			if sk.CheckClosedContour {
				if g.PolyLoops.SelfIntersecting(&g.PolyError.ErrorPoint) {
					g.PolyError.How = int(core.PolySelfIntersecting)
				}
			}
		}
	}
}

// GenerateShellAndMesh builds this group's shell (by extrusion,
// revolution, step and repeat, or linking), and combines it with the
// previous group's running shell by the group's Boolean. A group forced
// to a mesh combines meshes instead (see combineMeshes).
func (g *Group) GenerateShellAndMesh(sk *Sketch) {
	defer trace.StartRegion(context.Background(), "solid").End()
	defer sk.timed(g.H, timingSolid)()
	defer func() { g.DisplayDirty = true }()
	g.BooleanFailed = false
	g.Problem = GroupProblem{}
	g.forgetTopology()
	g.solidTol = sk.tol

	srcg := g

	g.ThisShell.Clear()
	g.ThisShell.Tol = sk.tol
	g.ThisMesh.Clear()
	g.RunningShell.Clear()
	g.RunningShell.Tol = sk.tol
	g.RunningMesh.Clear()

	// A group left out passes the solid so far through.
	if g.SuppressedBy.V != 0 {
		if prevg := g.PreviousGroup(sk); prevg != nil {
			g.RunningShell.MakeFromCopyOf(&prevg.RunningShell)
			g.RunningMesh.MakeFromCopyOf(&prevg.RunningMesh)
		}
		return
	}

	// A sketch on a face that lost its face says so first.
	if g.faceProblem != "" {
		g.setProblem(nil, "%s", g.faceProblem)
	}
	// A sketch that is no good as a profile says so, as does a solid
	// made from it.
	g.reportPolyProblem(sk)

	switch g.Type {
	case GroupTranslate, GroupRotate:
		// A step and repeat gets merged against the group's previous group,
		// not our own previous group.
		srcg = sk.Group(g.OpA)

		if !srcg.Suppress {
			if !g.IsForcedToMesh(sk) {
				g.generateForStepAndRepeatShell(sk, &srcg.ThisShell, &g.ThisShell, srcg.MeshCombine)
			} else {
				var prevm srf.SMesh
				prevm.MakeFromCopyOf(&srcg.ThisMesh)
				srcg.ThisShell.TriangulateInto(&prevm)
				g.generateForStepAndRepeatMesh(sk, &prevm, &g.ThisMesh, srcg.MeshCombine)
			}
		}
	case GroupMirror:
		srcg = sk.Group(g.OpA)
		if !srcg.Suppress && g.checkMirror(sk) {
			if !g.IsForcedToMesh(sk) {
				src := &srcg.ThisShell
				if g.mirrorsSolid(sk) {
					src = &srcg.RunningShell
				}
				g.generateMirrorShell(sk, src, &g.ThisShell, srcg.MeshCombine)
			} else {
				var prevm srf.SMesh
				if g.MirrorSolid {
					prevm.MakeFromCopyOf(&srcg.RunningMesh)
					srcg.RunningShell.TriangulateInto(&prevm)
				} else {
					prevm.MakeFromCopyOf(&srcg.ThisMesh)
					srcg.ThisShell.TriangulateInto(&prevm)
				}
				g.generateMirrorMesh(sk, &prevm, &g.ThisMesh, srcg.MeshCombine)
			}
		}
	case GroupPathPattern:
		srcg = sk.Group(g.OpA)
		switch {
		case g.IsForcedToMesh(sk):
			g.setProblem(nil, "a pattern along a path is made of solids, not of triangle meshes")
		case !srcg.Suppress && g.checkPattern(sk):
			g.generatePatternShell(sk, &srcg.ThisShell, &g.ThisShell, srcg.MeshCombine)
		}
	case GroupChamfer:
		if g.generateChamferShell(sk) {
			// Circular edges: it made the solid itself.
			return
		}
	case GroupSweep:
		g.generateSweepShell(sk)
	case GroupLoft:
		g.generateLoftShell(sk)
	case GroupWrap:
		if !g.IsForcedToMesh(sk) {
			// It combines the band with the solid so far by Booleans of
			// its own, loop set by loop set where it must.
			g.generateWrapShell(sk)
			return
		}
		g.generateWrapBand(sk)
	case GroupFillet:
		// It edits the solid so far into its running shell; no Boolean.
		g.generateFilletShell(sk)
		return
	case GroupShell:
		// It hollows the solid so far into its running shell, by a
		// Boolean of its own.
		g.generateShellShell(sk)
		return
	case GroupExtrude:
		g.generateExtrudeShell(sk)
	case GroupLathe, GroupRevolve:
		g.generateRevolveShell(sk)
	case GroupHelix:
		g.generateHelixShell(sk)
		g.reportThreadPart(sk)
	case GroupDrawingWorkplane:
		// A cosmetic thread's sketch.
		g.reportThreadPart(sk)
	case GroupLinked:
		g.generateLinkedShell(sk)
	}

	// Surfaces no entity names get an identity from this group, so
	// the edges of the faces they bound can be named.
	// A constructor may have given one a local origin from what made
	// it, which holds when the group makes more or fewer surfaces.
	for i := range g.ThisShell.Surface {
		s := &g.ThisShell.Surface[i]
		switch {
		case s.Face != 0:
		case s.Origin == 0:
			s.Origin = srf.OriginFor(g.H.V, i)
		case s.Origin&^0xffff == 0 && s.Origin&srf.LocalOriginBit != 0:
			s.Origin = srf.OriginFor(g.H.V, int(s.Origin))
		}
	}

	if srcg.MeshCombine != CombineAssemble {
		g.ThisShell.MergeCoincidentSurfaces()
	}

	// So now we've got the mesh or shell for this group. Combine it with
	// the previous group's mesh or shell with the requested Boolean, and
	// we're done.
	prevg := srcg.RunningMeshGroup(sk)

	// A mirror of the solid so far made the whole of it: no Boolean with
	// what it already holds.
	if g.Type == GroupMirror && g.mirrorsSolid(sk) {
		switch {
		case g.Suppress || srcg.Suppress:
			g.RunningShell.MakeFromCopyOf(&srcg.RunningShell)
			g.RunningMesh.MakeFromCopyOf(&srcg.RunningMesh)
		case !g.IsForcedToMesh(sk):
			g.RunningShell.MakeFromCopyOf(&g.ThisShell)
			g.RunningShell.MergeCoincidentSurfaces()
			g.BooleanFailed = g.RunningShell.BooleanFailed
		default:
			g.combineMeshes(nil, CombineUnion)
		}
		return
	}

	if !g.IsForcedToMesh(sk) {
		var prevs srf.SShell
		if prevg != nil {
			prevs = prevg.RunningShell
		}
		// A group that adds no solid, as a sketch, passes the solid so far
		// through as it is, whatever its own combine mode: merged, the
		// coplanar faces of the parts of an assembly before it would be
		// joined across parts into faces that don't mesh.
		combines := len(g.ThisShell.Surface) > 0 && !g.Suppress
		g.generateForBooleanShell(&prevs, &g.ThisShell, &g.RunningShell, srcg.MeshCombine)

		if combines && srcg.MeshCombine != CombineAssemble {
			g.RunningShell.MergeCoincidentSurfaces()
		}

		// If the Boolean failed, then we should note that in the text screen
		// for this group.
		g.BooleanFailed = g.RunningShell.BooleanFailed
	} else {
		g.combineMeshes(prevg, srcg.MeshCombine)
	}
}

// stepsOf returns the range of copies a step and repeat makes.
func (g *Group) stepsOf() (n, a0 int) {
	n, a0 = int(g.ValA), 0
	if g.Subtype == GroupOneSided && g.SkipFirst {
		a0++
		n++
	}
	return n, a0
}

// stepTransform is the translation and rotation of step a of n.
func (g *Group) stepTransform(sk *Sketch, a, n int) (geom.Vector, geom.Quaternion) {
	ap := a*2 - stepOffset(g.Subtype, n)

	trans := geom.Vector{
		X: paramFloat(sk, g.H.Param(0)),
		Y: paramFloat(sk, g.H.Param(1)),
		Z: paramFloat(sk, g.H.Param(2)),
	}
	if g.Type == GroupTranslate {
		return trans.ScaledBy(float64(ap)), geom.QuaternionIdentity
	}
	theta := float64(ap) * paramFloat(sk, g.H.Param(3))
	c, sn := libm.Cos(theta), libm.Sin(theta)
	axis := geom.Vector{
		X: paramFloat(sk, g.H.Param(4)),
		Y: paramFloat(sk, g.H.Param(5)),
		Z: paramFloat(sk, g.H.Param(6)),
	}
	q := geom.Quaternion{W: c, X: sn * axis.X, Y: sn * axis.Y, Z: sn * axis.Z}
	// Rotation is centered at t; so A(x - t) + t = Ax + (t - At)
	return trans.Minus(q.Rotate(trans)), q
}

// stepTransforms is stepTransform for every step of n.
func (g *Group) stepTransforms(sk *Sketch, n int) ([]geom.Vector, []geom.Quaternion) {
	ts, qs := make([]geom.Vector, n), make([]geom.Quaternion, n)
	for a := range n {
		ts[a], qs[a] = g.stepTransform(sk, a, n)
	}
	return ts, qs
}

// generateForStepAndRepeatShell makes the transformed copies of steps
// and combines them, a pair at a time.
func (g *Group) generateForStepAndRepeatShell(sk *Sketch, steps, outs *srf.SShell, forWhat CombineAs) {
	n, a0 := g.stepsOf()
	ts, qs := g.stepTransforms(sk, n)
	g.unionCopies(sk, steps, outs, forWhat, ts, qs, a0, func(a int) int { return stepRemap(a, n) })
}

// unionCopies makes the copies of steps that ts and qs move it to, from
// a0 on, with their faces remapped by remap, and combines them a pair
// at a time.
func (g *Group) unionCopies(sk *Sketch, steps, outs *srf.SShell, forWhat CombineAs,
	ts []geom.Vector, qs []geom.Quaternion, a0 int, remap func(a int) int) {
	n := len(ts)

	// create all the transformed copies
	transd := make([]srf.SShell, n)
	workA := make([]srf.SShell, n)
	// first generate a shell with each transformed copy
	par.For(n-a0, func(i int) {
		a := a0 + i
		transd[a].MakeFromTransformationOf(steps, ts[a], qs[a], 1.0)
	})
	for a := a0; a < n; a++ {
		// We need to rewrite any plane face entities to the transformed ones.
		transd[a].RemapFaces(shellGroup{g: g, sk: sk}, remap(a))
	}

	soFar, scratch := transd, workA
	// do the boolean operations on pairs of equal size
	for n > 1 {
		// The pairs share nothing, so they are combined at once.
		skipFirst := a0 == 1
		par.For((n+1)/2, func(i int) {
			a := 2 * i
			scratch[a/2].Clear()
			// combine a pair of shells
			switch {
			case a == 0 && skipFirst: // if the first was skipped just copy the 2nd
				// if the first was skipped just copy the 2nd
				scratch[a/2].MakeFromCopyOf(&soFar[a+1])
				soFar[a+1].Clear()

			case a == n-1: // for an odd number just copy the last one
				// for an odd number just copy the last one
				scratch[a/2].MakeFromCopyOf(&soFar[a])
				soFar[a].Clear()

			case forWhat == CombineAssemble:
				scratch[a/2].MakeFromAssemblyOf(&soFar[a], &soFar[a+1])
				soFar[a].Clear()
				soFar[a+1].Clear()

			default:
				scratch[a/2].MakeFromUnionOf(&soFar[a], &soFar[a+1])
				soFar[a].Clear()
				soFar[a+1].Clear()
			}
		})
		a0 = 0
		soFar, scratch = scratch, soFar
		n = (n + 1) / 2
	}
	outs.Clear()
	*outs = soFar[0]
}

// generateForStepAndRepeatMesh is generateForStepAndRepeatShell for a
// group forced to a mesh.
func (g *Group) generateForStepAndRepeatMesh(sk *Sketch, steps, outs *srf.SMesh, forWhat CombineAs) {
	n, a0 := g.stepsOf()

	transd := make([]srf.SMesh, n)
	workA := make([]srf.SMesh, n)
	ts, qs := g.stepTransforms(sk, n)
	par.For(n-a0, func(i int) {
		a := a0 + i
		transd[a].MakeFromTransformationOf(steps, ts[a], qs[a], 1.0)
	})
	for a := a0; a < n; a++ {
		transd[a].RemapFaces(shellGroup{g: g, sk: sk}, stepRemap(a, n))
	}

	soFar, scratch := transd, workA
	for n > 1 {
		skipFirst := a0 == 1
		par.For((n+1)/2, func(i int) {
			a := 2 * i
			scratch[a/2].Clear()
			switch {
			case a == 0 && skipFirst:
				scratch[a/2].MakeFromCopyOf(&soFar[a+1])
				soFar[a+1].Clear()

			case a == n-1:
				scratch[a/2].MakeFromCopyOf(&soFar[a])
				soFar[a].Clear()

			case forWhat == CombineAssemble:
				scratch[a/2].MakeFromAssemblyOf(&soFar[a], &soFar[a+1])
				soFar[a].Clear()
				soFar[a+1].Clear()

			default:
				scratch[a/2].MakeFromUnionOf(&soFar[a], &soFar[a+1])
				soFar[a].Clear()
				soFar[a+1].Clear()
			}
		})
		a0 = 0
		soFar, scratch = scratch, soFar
		n = (n + 1) / 2
	}
	outs.Clear()
	*outs = soFar[0]
}

// generateLinkedShell makes a linked group's solid: the imported shell
// or mesh are copied over, with the group's transformation applied, and
// their face entities remapped.
func (g *Group) generateLinkedShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisShell.Tol = sk.tol
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
	g.ThisMesh.MakeFromTransformationOf(&g.ImpMesh, offset, q, scale)
	g.ThisMesh.RemapFaces(shellGroup{g: g, sk: sk}, 0)

	g.ThisShell.MakeFromTransformationOf(&g.ImpShell, offset, q, scale)
	g.ThisShell.RemapFaces(shellGroup{g: g, sk: sk}, 0)
}

// shellGroup is the group as the shell constructors see it: its
// remap table gives the faces of the new surfaces.
type shellGroup struct {
	g  *Group
	sk *Sketch
}

func (sg shellGroup) Remap(in core.HEntity, copyNumber int) core.HEntity {
	return sg.g.RemapEntity(in, copyNumber)
}

func (sg shellGroup) EntityExists(h core.HEntity) bool {
	return sg.sk.Entity(h) != nil
}

// haveSource reports whether the source section of an extrusion or
// lathe is good: planar and not self-intersecting.
func (g *Group) haveSource(sk *Sketch) bool {
	if g.Type == GroupExtrude || g.Type == GroupLathe || g.Type == GroupRevolve {
		return sk.Group(g.OpA).PolyError.How == int(core.PolyGood)
	}
	return true
}

// generateExtrudeShell makes an extrusion's solid: extrude each of the
// source group's loop sets, and give the plane faces their entities so
// they can be selected.
func (g *Group) generateExtrudeShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisShell.Tol = sk.tol
	g.ThisMesh.Clear()
	if !g.haveSource(sk) {
		return
	}
	src := sk.Group(g.OpA)
	translate := geom.Vector{
		X: paramFloat(sk, g.H.Param(0)),
		Y: paramFloat(sk, g.H.Param(1)),
		Z: paramFloat(sk, g.H.Param(2)),
	}

	var tbot, ttop geom.Vector
	if g.Subtype == GroupOneSided || g.Subtype == GroupOneSkewed {
		tbot, ttop = geom.Vector{}, translate.ScaledBy(2)
	} else {
		tbot, ttop = translate.ScaledBy(-1), translate.ScaledBy(1)
	}
	if g.HasDraft() && len(src.BezierLoops.Sets) > 0 {
		if g.generateDraftShell(sk, src, tbot, ttop) {
			g.annotateDraftFaces(sk, src.BezierLoops.Sets[0].Point, tbot, ttop)
			return
		}
		g.ThisShell.Clear()
		g.ThisShell.Tol = sk.tol
	}

	for k := range src.BezierLoops.Sets {
		sbls := &src.BezierLoops.Sets[k]
		is := len(g.ThisShell.Surface)
		// Extrude this outer contour (plus its inner contours, if present)
		g.ThisShell.MakeFromExtrusionOf(sbls, tbot, ttop, g.Color)

		// And for any plane faces, annotate the model with the entity for
		// that face, so that the user can select them with the mouse.
		onOrig := sbls.Point
		for i := is; i < len(g.ThisShell.Surface); i++ {
			ss := &g.ThisShell.Surface[i]

			p := ss.PointAt(0, 0)
			n := ss.NormalAt(0, 0).WithMagnitude(1)
			d := n.Dot(p)

			if i == is || i == is+1 {
				// These are the top and bottom of the shell.
				if math.Abs(onOrig.Plus(ttop).Dot(n)-d) < core.LengthEps {
					ss.Face = srf.FaceID(g.RemapEntity(core.NoEntity, RemapTop).V)
				}
				if math.Abs(onOrig.Plus(tbot).Dot(n)-d) < core.LengthEps {
					ss.Face = srf.FaceID(g.RemapEntity(core.NoEntity, RemapBottom).V)
				}
				continue
			}

			// So these are the sides
			if ss.DegM != 1 || ss.DegN != 1 {
				continue
			}

			for ei := range sk.Entities {
				e := &sk.Entities[ei]
				if e.Group != g.OpA {
					continue
				}
				if e.Type != core.EntityLineSegment {
					continue
				}

				a := sk.MustEntity(e.Point[0]).PointGetNum(sk)
				b := sk.MustEntity(e.Point[1]).PointGetNum(sk)
				a = a.Plus(ttop)
				b = b.Plus(ttop)
				// Could get taken backwards, so check all cases.
				if (a.Equals(ss.Ctrl[0][0]) && b.Equals(ss.Ctrl[1][0])) ||
					(b.Equals(ss.Ctrl[0][0]) && a.Equals(ss.Ctrl[1][0])) ||
					(a.Equals(ss.Ctrl[0][1]) && b.Equals(ss.Ctrl[1][1])) ||
					(b.Equals(ss.Ctrl[0][1]) && a.Equals(ss.Ctrl[1][1])) {
					ss.Face = srf.FaceID(g.RemapEntity(e.H, RemapLineToFace).V)
					break
				}
			}
		}
	}
}

// revolveAxis returns the point on and the unit direction of the
// axis of a LATHE/REVOLVE/HELIX group, from its predef origin point
// and axis vector entities.
func (g *Group) revolveAxis(sk *Sketch) (pt, axis geom.Vector) {
	pt = sk.MustEntity(g.Predef.Origin).PointGetNum(sk)
	axis = sk.MustEntity(g.Predef.EntityB).VectorGetNum(sk)
	return pt, axis.WithMagnitude(1)
}

// generateRevolveShell makes a lathe's or a revolve's solid: revolve
// each of the source group's loop sets about the group's axis, a full
// turn for LATHE, by the group's angle for REVOLVE.
func (g *Group) generateRevolveShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisShell.Tol = sk.tol
	g.ThisMesh.Clear()
	if !g.haveSource(sk) {
		return
	}
	src := sk.Group(g.OpA)
	pt, axis := g.revolveAxis(sk)
	sg := shellGroup{g: g, sk: sk}

	angles, anglef, whole := g.revolveAngles(sk)
	for i := range src.BezierLoops.Sets {
		sbls := &src.BezierLoops.Sets[i]
		if whole {
			g.ThisShell.MakeFromRevolutionOf(sbls, pt, axis, g.Color, sg)
		} else {
			g.ThisShell.MakeFromHelicalRevolutionOf(sbls, pt, axis, g.Color, sg,
				angles, anglef, 0, 0)
		}
	}
}

// revolveAngles are the angles a lathe or a revolve turns its profile
// between, in radians, and whether that is a whole turn.
func (g *Group) revolveAngles(sk *Sketch) (angles, anglef float64, whole bool) {
	if g.Type == GroupLathe {
		return 0, 2 * math.Pi, true
	}
	anglef = paramFloat(sk, g.H.Param(3)) * 4 // why the 4 is needed?
	if g.Subtype != GroupOneSided {
		anglef *= 0.5
		angles = -anglef
	}
	return angles, anglef, math.Abs(anglef-angles) >= 2*math.Pi
}

// generateHelixShell makes a helix's solid: sweep each of the source
// group's loop sets about the group's axis while translating along it.
func (g *Group) generateHelixShell(sk *Sketch) {
	g.ThisShell.Clear()
	g.ThisShell.Tol = sk.tol
	g.ThisMesh.Clear()
	if !g.haveSource(sk) {
		return
	}
	g.sweepHelix(sk)
	if fine, ok := g.ridgeRetol(sk); ok {
		g.ThisShell.Clear()
		g.ThisShell.Tol = fine
		g.sweepHelix(sk)
	}
}

// sweepHelix sweeps the source group's loop sets into ThisShell, at
// its tolerance.
func (g *Group) sweepHelix(sk *Sketch) {
	src := sk.Group(g.OpA)
	pt, axis := g.revolveAxis(sk)

	anglef := paramFloat(sk, g.H.Param(3)) * 4 // why the 4 is needed?
	dists, distf := 0.0, 0.0
	angles := 0.0
	distf = paramFloat(sk, g.H.Param(7)) * 2 // dist is applied twice
	if g.Subtype != GroupOneSided {
		anglef *= 0.5
		angles = -anglef
		distf *= 0.5
		dists = -distf
	}
	for i := range src.BezierLoops.Sets {
		g.ThisShell.MakeFromHelicalRevolutionOf(&src.BezierLoops.Sets[i], pt, axis, g.Color,
			shellGroup{g: g, sk: sk}, angles, anglef, dists, distf)
	}
}

// generateForBooleanShell combines prevs and thiss into outs with the
// group's MeshCombine.
func (g *Group) generateForBooleanShell(prevs, thiss, outs *srf.SShell, how CombineAs) {
	// If this group contributes no new mesh, then our running mesh is the
	// same as last time, no combining required. Likewise if we have a mesh
	// but it's suppressed.
	if len(thiss.Surface) == 0 || g.Suppress {
		outs.MakeFromCopyOf(prevs)
		return
	}

	// So our group's shell appears in thisShell. Combine this with the
	// previous group's shell, using the requested operation.
	switch how {
	case CombineUnion:
		outs.MakeFromUnionOf(prevs, thiss)
	case CombineDifference, CombineIntersection:
		intersect := how == CombineIntersection
		if intersect {
			outs.MakeFromIntersectionOf(prevs, thiss)
		} else {
			outs.MakeFromDifferenceOf(prevs, thiss)
		}
		// Parts assembled side by side that touch may fail a Boolean
		// each of them passes; the cut is made part by part then.
		if outs.BooleanFailed {
			var by srf.SShell
			if by.MakeFromPiecewiseOf(prevs, thiss, intersect) && !by.BooleanFailed {
				*outs = by
			}
		}
	case CombineAssemble:
		outs.MakeFromAssemblyOf(prevs, thiss)
	}
}

// combineMeshes is GenerateShellAndMesh's combine for a group forced to
// a mesh: triangulate both operands, combine the meshes, and make the
// result vertex to vertex.
func (g *Group) combineMeshes(prevg *Group, how CombineAs) {
	defer trace.StartRegion(context.Background(), "mesh boolean").End()
	var prevm, thism srf.SMesh
	if prevg != nil {
		prevm.MakeFromCopyOf(&prevg.RunningMesh)
		prevg.RunningShell.TriangulateInto(&prevm)
	}

	thism.MakeFromCopyOf(&g.ThisMesh)
	g.ThisShell.TriangulateInto(&thism)

	var outm srf.SMesh
	switch {
	case thism.IsEmpty() || g.Suppress:
		outm.MakeFromCopyOf(&prevm)
	case how == CombineUnion:
		outm.MakeFromUnionOf(&prevm, &thism)
	case how == CombineDifference:
		outm.MakeFromDifferenceOf(&prevm, &thism)
	case how == CombineIntersection:
		outm.MakeFromIntersectionOf(&prevm, &thism)
	case how == CombineAssemble:
		outm.MakeFromAssemblyOf(&prevm, &thism)
	}

	// Remove degenerate triangles; if we don't, they'll get split in SnapToMesh
	// in every generated group, resulting in polynomial increase in triangle count,
	// and corresponding slowdown.
	outm.RemoveDegenerateTriangles()

	if how != CombineAssemble {
		// And make sure that the output mesh is vertex-to-vertex.
		root := srf.SKdNodeFrom(&outm)
		root.SnapToMesh(&outm)
		root.MakeMeshInto(&g.RunningMesh)
	} else {
		g.RunningMesh.MakeFromCopyOf(&outm)
	}
}

// GenerateDisplayItems builds the mesh and outlines to display: the
// previous group's, if this group adds no solid, else the running shell
// triangulated, with the running mesh.
func (g *Group) GenerateDisplayItems(sk *Sketch) {
	// This is potentially slow (since we've got to triangulate a shell,
	// or to find the emphasized edges for a mesh), so we will run it
	// only if its inputs have changed.
	if !g.DisplayDirty {
		return
	}
	defer func() { g.DisplayDirty = false }()
	pg := g.RunningMeshGroup(sk)
	if pg != nil && len(g.ThisMesh.Triangles) == 0 && len(g.ThisShell.Surface) == 0 {
		// We don't contribute any new solid model in this group, so our
		// display items are identical to the previous group's.
		pg.GenerateDisplayItems(sk)

		g.DisplayMesh.Clear()
		g.DisplayMesh.MakeFromCopyOf(&pg.DisplayMesh)

		g.DisplayOutlines.Clear()
		if show := sk.visibility(); show.Edges || show.Outlines {
			g.DisplayOutlines.Outlines = append(g.DisplayOutlines.Outlines, pg.DisplayOutlines.Outlines...)
		}
	} else {
		g.generateOwnDisplayItems(sk)
	}

	// If we render this mesh, we need to know whether it's transparent,
	// and we'll want all transparent triangles last, to make the depth
	// test work correctly. This also sets the order the mesh exports in.
	g.DisplayMesh.PrecomputeTransparency()
}

// generateOwnDisplayItems triangulates the running shell and adds the
// running mesh, for a group that contributes new solid model.
func (g *Group) generateOwnDisplayItems(sk *Sketch) {
	// We do contribute new solid model, so we have to triangulate the
	// shell, and edge-find the mesh.
	g.DisplayMesh.Clear()
	g.RunningShell.TriangulateInto(&g.DisplayMesh)
	for _, t := range g.RunningMesh.Triangles {
		trn := t
		n := trn.Normal()
		trn.NA, trn.NB, trn.NC = n, n, n
		g.DisplayMesh.Triangles = append(g.DisplayMesh.Triangles, trn)
	}

	g.DisplayOutlines.Clear()
	if show := sk.visibility(); !show.Edges && !show.Outlines {
		return
	}
	var rawOutlines srf.SOutlineList
	if len(g.RunningMesh.Triangles) > 0 {
		// Triangle mesh only; no shell or emphasized edges.
		g.RunningMesh.MakeOutlinesInto(&rawOutlines, srf.EdgeEmphasized)
	} else {
		// The edges between faces too where they meet smoothly, as
		// about a fillet; not the seams within a face, whose patches
		// carry one face (SShell.displayFace).
		g.DisplayMesh.MakeOutlinesInto(&rawOutlines, srf.EdgeSharpOrFace)
	}
	var builder srf.PolylineBuilder
	builder.MakeFromOutlines(&rawOutlines)
	builder.GenerateOutlines(&g.DisplayOutlines)
}

// IsForcedToMesh reports whether the group is built as a triangle mesh
// instead of a shell: asked to be, an imported mesh, or combined with a
// mesh.
func (g *Group) IsForcedToMesh(sk *Sketch) bool {
	return g.ForceToMesh || g.IsTriangleMeshAssembly() || g.IsForcedToMeshBySource(sk)
}

// IsForcedToMeshBySource reports whether the group combines with a
// group forced to a mesh.
func (g *Group) IsForcedToMeshBySource(sk *Sketch) bool {
	srcg := g
	if g.IsCopyGroup() {
		// A step and repeat gets merged against the group's previous group,
		// not our own previous group.
		srcg = sk.Group(g.OpA)
		if srcg.ForceToMesh {
			return true
		}
	}
	rg := srcg.RunningMeshGroup(sk)
	if rg == nil {
		return false
	}
	return rg.ForceToMesh || rg.IsForcedToMeshBySource(sk)
}

// IsTriangleMeshAssembly reports whether the group links a triangle
// mesh, without a shell.
func (g *Group) IsTriangleMeshAssembly() bool {
	if g.Type != GroupLinked {
		return false
	}
	return len(g.ImpMesh.Triangles) > 0 && len(g.ImpShell.Surface) == 0
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

// ExtrusionForceVectorTo sets the extrusion vector's parameters.
func (g *Group) ExtrusionForceVectorTo(sk *Sketch, v geom.Vector) {
	sk.MustParam(g.H.Param(0)).Val = v.X
	sk.MustParam(g.H.Param(1)).Val = v.Y
	sk.MustParam(g.H.Param(2)).Val = v.Z
}

// ExtrusionGetVector returns the extrusion vector, from its parameters.
func (g *Group) ExtrusionGetVector(sk *Sketch) geom.Vector {
	return geom.Vector{
		X: sk.MustParam(g.H.Param(0)).Val,
		Y: sk.MustParam(g.H.Param(1)).Val,
		Z: sk.MustParam(g.H.Param(2)).Val,
	}
}

// TransformImportedBy moves a linked group by t and rotates it by q,
// composing them into its placement parameters.
func (g *Group) TransformImportedBy(sk *Sketch, t geom.Vector, q geom.Quaternion) {
	if g.Type != GroupLinked {
		panic("Expected a linked group")
	}

	tx, ty, tz := g.H.Param(0), g.H.Param(1), g.H.Param(2)
	qw, qx, qy, qz := g.H.Param(3), g.H.Param(4), g.H.Param(5), g.H.Param(6)
	val := func(h core.HParam) float64 { return sk.MustParam(h).Val }

	qg := geom.NewQuaternion(val(qw), val(qx), val(qy), val(qz))
	qg = q.Times(qg)

	tg := geom.Vector{X: val(tx), Y: val(ty), Z: val(tz)}
	tg = tg.Plus(t)

	sk.MustParam(tx).Val = tg.X
	sk.MustParam(ty).Val = tg.Y
	sk.MustParam(tz).Val = tg.Z

	sk.MustParam(qw).Val = qg.W
	sk.MustParam(qx).Val = qg.X
	sk.MustParam(qy).Val = qg.Y
	sk.MustParam(qz).Val = qg.Z
}

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
