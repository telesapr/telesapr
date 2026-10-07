package sketch

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Remap keys for entity copies made by Group::Generate (C++
// Group::REMAP_*). Small non-negative keys are step-and-repeat copy
// numbers.
const (
	RemapLast           = 1000
	RemapTop            = 1001
	RemapBottom         = 1002
	RemapPtToLine       = 1003
	RemapLineToFace     = 1004
	RemapLatheStart     = 1006
	RemapLatheEnd       = 1007
	RemapPtToArc        = 1008
	RemapPtToNormal     = 1009
	RemapLatheArcCenter = 1010
)

// CopyAs is the transformation a copied entity gets (C++
// Group::CopyAs).
type CopyAs int

const (
	CopyAsNumeric CopyAs = iota
	CopyAsNTrans
	CopyAsNRotAA
	CopyAsNRotTrans
	CopyAsNRotAxisTrans
)

// AddParam appends a group param with an initial value (C++
// Group::AddParam).
func AddParam(param *[]core.Param, hp core.HParam, v float64) {
	*param = append(*param, core.Param{H: hp, Val: v})
}

// Generate creates this group's entities and params (C++
// Group::Generate). It appends to entity and param, which during
// GenerateAll are sk.Entities and sk.Params themselves.
func (g *Group) Generate(sk *Sketch, entity *[]core.Entity, param *[]core.Param) {
	// C++ seeds new group params from SS.GW's view; it only matters
	// for params without a previous value.
	view := sk.view()
	gn := view.ProjRight.Cross(view.ProjUp)
	gp := view.ProjRight.Plus(view.ProjUp)
	gc := view.Offset.ScaledBy(-1)
	gn = gn.WithMagnitude(200 / view.Scale)
	gp = gp.WithMagnitude(200 / view.Scale)
	h := g.H

	switch g.Type {
	case GroupDrawing3D:
		return

	case GroupDrawingWorkplane:
		var q geom.Quaternion
		switch g.Subtype {
		case GroupWorkplaneByLineSegments:
			u := sk.MustGetEntity(g.Predef.EntityB).VectorGetNum(sk)
			v := sk.MustGetEntity(g.Predef.EntityC).VectorGetNum(sk)
			u = u.WithMagnitude(1)
			n := u.Cross(v)
			v = n.Cross(u).WithMagnitude(1)

			if g.Predef.SwapUV {
				u, v = v, u
			}
			if g.Predef.NegateU {
				u = u.ScaledBy(-1)
			}
			if g.Predef.NegateV {
				v = v.ScaledBy(-1)
			}
			q = geom.QuaternionFromBasis(u, v)
		case GroupWorkplaneByPointOrtho:
			// Already given, numerically.
			q = g.Predef.Q
		case GroupWorkplaneByPointNormal:
			q = sk.MustGetEntity(g.Predef.EntityB).NormalGetNum(sk)
		default:
			panic(fmt.Sprintf("unexpected workplane subtype %d", g.Subtype))
		}

		normal := core.Entity{
			Type:      core.EntityNormalNCopy,
			NumNormal: q}
		normal.Point[0] = h.Entity(2)
		normal.Group = h
		normal.H = h.Entity(1)
		*entity = append(*entity, normal)

		point := core.Entity{
			Type:         core.EntityPointNCopy,
			NumPoint:     sk.MustGetEntity(g.Predef.Origin).PointGetNum(sk),
			Construction: true,
			Group:        h,
			H:            h.Entity(2)}
		*entity = append(*entity, point)

		wp := core.Entity{
			Type:   core.EntityWorkplane,
			Normal: normal.H}
		wp.Point[0] = point.H
		wp.Group = h
		wp.H = h.Entity(0)
		*entity = append(*entity, wp)
		return

	case GroupExtrude:
		AddParam(param, h.Param(0), gn.X)
		AddParam(param, h.Param(1), gn.Y)
		AddParam(param, h.Param(2), gn.Z)
		var ai, af int
		switch g.Subtype {
		case GroupOneSided, GroupOneSkewed:
			ai, af = 0, 2
		case GroupTwoSided, GroupTwoSkewed:
			ai, af = -1, 1
		default:
			panic(fmt.Sprintf("unexpected extrusion subtype %d", g.Subtype))
		}

		// Get some arbitrary point in the sketch, that will be used
		// as a reference when defining top and bottom faces.
		var pt core.HEntity
		// Not using range here because we're changing the size of
		// entity in the loop.
		for i := 0; i < len(*entity); i++ {
			e := &(*entity)[i]
			if e.Group != g.OpA {
				continue
			}

			if e.IsPoint() {
				pt = e.H
			}

			e.CalculateNumerical(sk, false)
			he := e.H
			// As soon as I call CopyEntity, e may become invalid! That
			// adds entities, which may cause a realloc.
			n0 := len(*entity)
			g.CopyEntity(sk, entity, sk.MustGetEntity(he), ai, RemapBottom,
				h.Param(0), h.Param(1), h.Param(2),
				core.NoParam, core.NoParam, core.NoParam, core.NoParam, core.NoParam,
				CopyAsNTrans)
			n1 := len(*entity)
			g.CopyEntity(sk, entity, sk.MustGetEntity(he), af, RemapTop,
				h.Param(0), h.Param(1), h.Param(2),
				core.NoParam, core.NoParam, core.NoParam, core.NoParam, core.NoParam,
				CopyAsNTrans)
			g.MakeExtrusionLines(sk, entity, he)
			// A draft moves the far end and the sides off where these
			// are (draft.go), and both ends of a two-sided extrusion.
			if g.HasDraft() {
				from := n1
				if ai != 0 || g.ValB != 0 {
					// Two-sided, or leaning from another plane: both ends
					// move.
					from = n0
				}
				for i := from; i < len(*entity); i++ {
					(*entity)[i].ForceHidden = true
				}
			}
		}
		// Remapped versions of that arbitrary point will be used to
		// provide points on the plane faces.
		g.MakeExtrusionTopBottomFaces(sk, entity, pt)
		return

	case GroupLathe:
		axisPos := sk.MustGetEntity(g.Predef.Origin).PointGetNum(sk)
		axisDir := sk.MustGetEntity(g.Predef.EntityB).VectorGetNum(sk)

		for i := 0; i < len(*entity); i++ {
			e := &(*entity)[i]
			if e.Group != g.OpA {
				continue
			}

			e.CalculateNumerical(sk, false)
			he := e.H

			// As soon as I call CopyEntity, e may become invalid! That
			// adds entities, which may cause a realloc.

			// this is the regular copy of all entities
			g.CopyEntity(sk, entity, sk.MustGetEntity(he), 0, RemapLatheStart,
				core.NoParam, core.NoParam, core.NoParam,
				core.NoParam, core.NoParam, core.NoParam, core.NoParam, core.NoParam,
				CopyAsNumeric)

			e = &(*entity)[i] // because we copied.
			if e.IsPoint() {
				// for points this copy is used for the circle centers
				g.CopyEntity(sk, entity, sk.MustGetEntity(he), 0, RemapLatheArcCenter,
					core.NoParam, core.NoParam, core.NoParam,
					core.NoParam, core.NoParam, core.NoParam, core.NoParam, core.NoParam,
					CopyAsNumeric)
				g.MakeLatheCircles(sk, entity, he, axisPos, axisDir)
			}
			g.MakeLatheSurfacesSelectable(sk, entity, he, axisDir)
		}
		return

	case GroupRevolve:
		// this was borrowed from LATHE and ROTATE
		axisPos := sk.MustGetEntity(g.Predef.Origin).PointGetNum(sk)
		axisDir := sk.MustGetEntity(g.Predef.EntityB).VectorGetNum(sk)

		// The center of rotation
		AddParam(param, h.Param(0), axisPos.X)
		AddParam(param, h.Param(1), axisPos.Y)
		AddParam(param, h.Param(2), axisPos.Z)
		// The rotation quaternion
		AddParam(param, h.Param(3), 30*geom.Pi/180)
		AddParam(param, h.Param(4), axisDir.X)
		AddParam(param, h.Param(5), axisDir.Y)
		AddParam(param, h.Param(6), axisDir.Z)

		// Get some arbitrary point in the sketch, that will be used
		// as a reference when defining end faces.
		var pt core.HEntity

		ai, af := 0, 2
		if g.Subtype == GroupTwoSided {
			ai, af = -1, 1
		}
		for i := 0; i < len(*entity); i++ {
			e := &(*entity)[i]
			if e.Group != g.OpA {
				continue
			}

			if e.IsPoint() {
				pt = e.H
			}

			e.CalculateNumerical(sk, false)
			he := e.H
			// one copy for each end of the revolved surface
			g.CopyEntity(sk, entity, e, ai, RemapLatheStart, h.Param(0),
				h.Param(1), h.Param(2), h.Param(3), h.Param(4), h.Param(5),
				h.Param(6), core.NoParam, CopyAsNRotAA)

			e = &(*entity)[i] // because we copied.
			g.CopyEntity(sk, entity, e, af, RemapLatheEnd, h.Param(0),
				h.Param(1), h.Param(2), h.Param(3), h.Param(4), h.Param(5),
				h.Param(6), core.NoParam, CopyAsNRotAA)

			// Arcs are not generated for revolve groups, for now, because our current arc
			// entity is not chiral, and dragging a revolve may break the arc by inverting it.
			// MakeLatheCircles(entity, param, he, axis_pos, axis_dir);
			g.MakeLatheSurfacesSelectable(sk, entity, he, axisDir)
		}
		g.MakeRevolveEndFaces(sk, entity, pt, ai, af)
		return

	case GroupHelix:
		axisPos := sk.MustGetEntity(g.Predef.Origin).PointGetNum(sk)
		axisDir := sk.MustGetEntity(g.Predef.EntityB).VectorGetNum(sk)

		// The center of rotation
		AddParam(param, h.Param(0), axisPos.X)
		AddParam(param, h.Param(1), axisPos.Y)
		AddParam(param, h.Param(2), axisPos.Z)
		// The rotation quaternion
		AddParam(param, h.Param(3), 30*geom.Pi/180)
		AddParam(param, h.Param(4), axisDir.X)
		AddParam(param, h.Param(5), axisDir.Y)
		AddParam(param, h.Param(6), axisDir.Z)
		// distance to translate along the rotation axis
		AddParam(param, h.Param(7), 20)

		// Get some arbitrary point in the sketch, that will be used
		// as a reference when defining end faces.
		var pt core.HEntity

		ai, af := 0, 2 // initial and final number of transformations
		if g.Subtype != GroupOneSided {
			ai, af = -1, 1
		}

		for i := 0; i < len(*entity); i++ {
			e := &(*entity)[i]
			if e.Group != g.OpA && e.H != g.Predef.Origin {
				continue
			}

			if e.IsPoint() {
				pt = e.H
			}

			e.CalculateNumerical(sk, false)

			// one copy for each end of the helix
			g.CopyEntity(sk, entity, e, ai, RemapLatheStart, h.Param(0),
				h.Param(1), h.Param(2), h.Param(3), h.Param(4), h.Param(5),
				h.Param(6), h.Param(7), CopyAsNRotAxisTrans)

			e = &(*entity)[i] // because we copied.
			g.CopyEntity(sk, entity, e, af, RemapLatheEnd, h.Param(0),
				h.Param(1), h.Param(2), h.Param(3), h.Param(4), h.Param(5),
				h.Param(6), h.Param(7), CopyAsNRotAxisTrans)

			// For point entities on the axis, create a construction line
			e = &(*entity)[i]
			if e.IsPoint() {
				check := e.PointGetNum(sk).Minus(axisPos).Cross(axisDir)
				if check.Dot(check) < core.LengthEps {
					ep := sk.MustGetEntity(e.H)
					en := core.Entity{}
					// A point gets extruded to form a line segment
					en.Point[0] = g.RemapEntity(ep.H, RemapLatheStart)
					en.Point[1] = g.RemapEntity(ep.H, RemapLatheEnd)
					en.Group = h
					en.Construction = ep.Construction
					en.Style = ep.Style
					en.H = g.RemapEntity(ep.H, RemapPtToLine)
					en.Type = core.EntityLineSegment
					*entity = append(*entity, en)
				}
			}
		}
		g.MakeRevolveEndFaces(sk, entity, pt, ai, af)
		return

	case GroupTranslate:
		// inherit meshCombine from source group
		srcg := sk.GetGroup(g.OpA)
		g.MeshCombine = srcg.MeshCombine
		// The translation vector
		AddParam(param, h.Param(0), gp.X)
		AddParam(param, h.Param(1), gp.Y)
		AddParam(param, h.Param(2), gp.Z)

		n, a0 := int(g.ValA), 0
		if g.Subtype == GroupOneSided && g.SkipFirst {
			a0++
			n++
		}

		for a := a0; a < n; a++ {
			for i := 0; i < len(*entity); i++ {
				e := &(*entity)[i]
				if e.Group != g.OpA {
					continue
				}

				e.CalculateNumerical(sk, false)
				g.CopyEntity(sk, entity, e,
					a*2-stepOffset(g.Subtype, n),
					stepRemap(a, n),
					h.Param(0), h.Param(1), h.Param(2),
					core.NoParam, core.NoParam, core.NoParam, core.NoParam, core.NoParam,
					CopyAsNTrans)
			}
		}
		return

	case GroupRotate:
		// inherit meshCombine from source group
		srcg := sk.GetGroup(g.OpA)
		g.MeshCombine = srcg.MeshCombine
		// The center of rotation
		AddParam(param, h.Param(0), gc.X)
		AddParam(param, h.Param(1), gc.Y)
		AddParam(param, h.Param(2), gc.Z)
		// The rotation quaternion
		AddParam(param, h.Param(3), 30*geom.Pi/180)
		AddParam(param, h.Param(4), gn.X)
		AddParam(param, h.Param(5), gn.Y)
		AddParam(param, h.Param(6), gn.Z)

		n, a0 := int(g.ValA), 0
		if g.Subtype == GroupOneSided && g.SkipFirst {
			a0++
			n++
		}

		for a := a0; a < n; a++ {
			for i := 0; i < len(*entity); i++ {
				e := &(*entity)[i]
				if e.Group != g.OpA {
					continue
				}

				e.CalculateNumerical(sk, false)
				g.CopyEntity(sk, entity, e,
					a*2-stepOffset(g.Subtype, n),
					stepRemap(a, n),
					h.Param(0), h.Param(1), h.Param(2),
					h.Param(3), h.Param(4), h.Param(5), h.Param(6), core.NoParam,
					CopyAsNRotAA)
			}
		}
		return

	case GroupMirror:
		g.generateMirror(sk, entity, param)
		return

	case GroupChamfer, GroupFillet, GroupLoft, GroupShell, GroupPathPattern:
		return

	case GroupSweep:
		g.generateSweep(sk, entity)
		return

	case GroupLinked:
		// The translation vector. A model comes in beside what is
		// drawn, off the middle of the view, as in C++; a STEP file or
		// a mesh where it was drawn, so that its coordinates are the
		// file's (roadmap 16.13).
		if !g.linksModel() {
			gp = geom.Vector{}
		}
		AddParam(param, h.Param(0), gp.X)
		AddParam(param, h.Param(1), gp.Y)
		AddParam(param, h.Param(2), gp.Z)
		// The rotation quaternion
		AddParam(param, h.Param(3), 1)
		AddParam(param, h.Param(4), 0)
		AddParam(param, h.Param(5), 0)
		AddParam(param, h.Param(6), 0)

		for i := 0; i < len(g.ImpEntity); i++ {
			ie := &g.ImpEntity[i]
			g.CopyEntity(sk, entity, ie, 0, 0,
				h.Param(0), h.Param(1), h.Param(2),
				h.Param(3), h.Param(4), h.Param(5), h.Param(6), core.NoParam,
				CopyAsNRotTrans)
		}
		return
	}
	panic(fmt.Sprintf("unexpected group type %d", g.Type))
}

// linksModel reports whether a link group links a model file, rather
// than a STEP file, a mesh or a board.
func (g *Group) linksModel() bool {
	ext := strings.ToLower(filepath.Ext(g.LinkFile))
	return ext == ".tlspr" || ext == ".slvs"
}

// stepOffset and stepRemap are the step-and-repeat timesApplied offset
// and remap key of copy a out of n (inline expressions in C++).
func stepOffset(subtype GroupSubtype, n int) int {
	if subtype == GroupOneSided {
		return 0
	}
	return n - 1
}

func stepRemap(a, n int) int {
	if a == n-1 {
		return RemapLast
	}
	return a
}

// RemapEntity returns the handle of copy copyNumber of entity in, allocating
// a new one in the group's remap table if needed (C++ Group::Remap).
// The table is saved with the file, so copies keep their handles.
func (g *Group) RemapEntity(in core.HEntity, copyNumber int) core.HEntity {
	if g.Remap == nil {
		g.Remap = make(core.EntityMap)
	}
	key := core.EntityKey{Input: in, CopyNumber: copyNumber}
	id, ok := g.Remap[key]
	if !ok {
		// Due to the way the remap value is combined with the group handle into a 32-bit
		// handle value to generate an entity handle, the mapped value must fit in a 16-bit
		// variable.
		if len(g.Remap) >= (1<<16)-1 {
			panic("Too many entities in group")
		}
		id = core.EntityID{V: uint32(len(g.Remap)) + 1}
		g.Remap[key] = id
	}
	return g.H.Entity(int(id.V))
}

func (g *Group) MakeExtrusionLines(sk *Sketch, el *[]core.Entity, in core.HEntity) {
	ep := sk.MustGetEntity(in)

	en := core.Entity{}
	if ep.IsPoint() {
		// A point gets extruded to form a line segment
		en.Point[0] = g.RemapEntity(ep.H, RemapTop)
		en.Point[1] = g.RemapEntity(ep.H, RemapBottom)
		en.Group = g.H
		en.Construction = ep.Construction
		en.Style = ep.Style
		en.H = g.RemapEntity(ep.H, RemapPtToLine)
		en.Type = core.EntityLineSegment
		*el = append(*el, en)
	} else if ep.Type == core.EntityLineSegment {
		// A line gets extruded to form a plane face; an endpoint of the
		// original line is a point in the plane, and the line is in the plane.
		a := sk.MustGetEntity(ep.Point[0]).PointGetNum(sk)
		b := sk.MustGetEntity(ep.Point[1]).PointGetNum(sk)
		ab := b.Minus(a)

		en.Param[0] = g.H.Param(0)
		en.Param[1] = g.H.Param(1)
		en.Param[2] = g.H.Param(2)
		en.NumPoint = a
		// we need a point somewhere on the face, and it must be from the current group.
		en.Point[0] = g.RemapEntity(ep.Point[0], RemapTop)
		en.NumNormal = geom.Quaternion{W: 0, X: ab.X, Y: ab.Y, Z: ab.Z}

		en.Group = g.H
		en.Construction = ep.Construction
		en.Style = ep.Style
		en.H = g.RemapEntity(ep.H, RemapLineToFace)
		en.Type = core.EntityFaceXProd
		*el = append(*el, en)
	}
}

func (g *Group) MakeLatheCircles(sk *Sketch, el *[]core.Entity, in core.HEntity, pt, axis geom.Vector) {
	ep := sk.MustGetEntity(in)

	en := core.Entity{}
	if ep.IsPoint() {
		// A point gets revolved to form an arc.
		en.Point[0] = g.RemapEntity(ep.H, RemapLatheArcCenter)
		en.Point[1] = g.RemapEntity(ep.H, RemapLatheStart)
		en.Point[2] = en.Point[1] // Remap(ep->h, REMAP_LATHE_END);

		// Get arc center and point on arc.
		pc := sk.MustGetEntity(en.Point[0])
		pp := sk.MustGetEntity(en.Point[1])

		// Project arc point to the revolution axis and use it for arc center.
		k := pp.NumPoint.Minus(pt).Dot(axis) / axis.Dot(axis)
		pc.NumPoint = pt.Plus(axis.ScaledBy(k))

		// Create arc entity.
		en.Group = g.H
		en.Construction = ep.Construction
		en.Style = ep.Style
		en.H = g.RemapEntity(ep.H, RemapPtToArc)
		en.Type = core.EntityArcOfCircle

		// Generate a normal.
		n := core.Entity{
			Workplane: en.Workplane,
			H:         g.RemapEntity(ep.H, RemapPtToNormal),
			Group:     en.Group,
			Style:     en.Style,
			Type:      core.EntityNormalNCopy}

		// Create basis for the normal.
		nu := pp.NumPoint.Minus(pc.NumPoint).WithMagnitude(1.0)
		nv := nu.Cross(axis).WithMagnitude(1.0)
		n.NumNormal = geom.QuaternionFromBasis(nv, nu)

		// The point determines where the normal gets displayed on-screen;
		// it's entirely cosmetic.
		n.Point[0] = en.Point[0]
		*el = append(*el, n)
		en.Normal = n.H
		*el = append(*el, en)
	}
}

func (g *Group) MakeLatheSurfacesSelectable(sk *Sketch, el *[]core.Entity, in core.HEntity, axis geom.Vector) {
	ep := sk.MustGetEntity(in)

	en := core.Entity{}
	if ep.Type == core.EntityLineSegment {
		// An axis-perpendicular line gets revolved to form a face.
		a := sk.MustGetEntity(ep.Point[0]).PointGetNum(sk)
		b := sk.MustGetEntity(ep.Point[1]).PointGetNum(sk)
		u := b.Minus(a).WithMagnitude(1.0)

		// Check for perpendicularity: calculate cosine of the angle
		// between axis and line direction and check that
		// cos(angle) == 0 <-> angle == +-90 deg.
		if math.Abs(u.Dot(axis)/axis.Magnitude()) < core.AngleCosEps {
			en.Param[0] = g.H.Param(0)
			en.Param[1] = g.H.Param(1)
			en.Param[2] = g.H.Param(2)
			v := axis.Cross(u).WithMagnitude(1.0)
			n := u.Cross(v)
			en.NumNormal = geom.Quaternion{W: 0, X: n.X, Y: n.Y, Z: n.Z}

			en.Group = g.H
			en.Construction = ep.Construction
			en.Style = ep.Style
			en.H = g.RemapEntity(ep.H, RemapLineToFace)
			en.Type = core.EntityFaceNormalPt
			en.Point[0] = g.RemapEntity(ep.Point[0], RemapLatheStart)
			*el = append(*el, en)
		}
	}
}

// sourceLoopNormal is the normal of the source group's loops, falling
// back to its workplane's normal, for the end faces of extrusions and
// revolutions.
func (g *Group) sourceLoopNormal(sk *Sketch) geom.Vector {
	src := sk.GetGroup(g.OpA)
	n := src.PolyLoops.Normal

	// When there is no loop normal (e.g. if the loop is broken), use normal of workplane
	// as fallback, to avoid breaking constraints depending on the faces.
	if n.Equals(geom.Vector{}) && src.Type == GroupDrawingWorkplane {
		n = sk.MustGetEntity(src.H.Entity(0)).NormalEntity(sk).NormalN(sk)
	}
	return n
}

// For Revolve and Helix groups the end faces are remapped from an arbitrary
// point on the sketch. We reference the transformed point but there is
// no existing normal so we need to define the rotation and timesApplied.
func (g *Group) MakeRevolveEndFaces(sk *Sketch, el *[]core.Entity, pt core.HEntity, ai, af int) {
	if pt.V == 0 {
		return
	}
	n := g.sourceLoopNormal(sk)

	en := core.Entity{
		Type:  core.EntityFaceRotNormalPt,
		Group: g.H}
	// The center of rotation
	en.Param[0] = g.H.Param(0)
	en.Param[1] = g.H.Param(1)
	en.Param[2] = g.H.Param(2)
	// The rotation quaternion
	en.Param[3] = g.H.Param(3)
	en.Param[4] = g.H.Param(4)
	en.Param[5] = g.H.Param(5)
	en.Param[6] = g.H.Param(6)

	en.NumNormal = geom.Quaternion{W: 0, X: n.X, Y: n.Y, Z: n.Z}
	en.Point[0] = g.RemapEntity(pt, RemapLatheStart)
	en.TimesApplied = ai
	en.H = g.RemapEntity(core.NoEntity, RemapLatheStart)
	*el = append(*el, en)

	en.Point[0] = g.RemapEntity(pt, RemapLatheEnd)
	en.TimesApplied = af
	en.H = g.RemapEntity(core.NoEntity, RemapLatheEnd)
	*el = append(*el, en)
}

func (g *Group) MakeExtrusionTopBottomFaces(sk *Sketch, el *[]core.Entity, pt core.HEntity) {
	if pt.V == 0 {
		return
	}
	n := g.sourceLoopNormal(sk)

	en := core.Entity{
		Type:  core.EntityFaceNormalPt,
		Group: g.H,

		NumNormal: geom.Quaternion{W: 0, X: n.X, Y: n.Y, Z: n.Z}}
	en.Point[0] = g.RemapEntity(pt, RemapTop)
	en.H = g.RemapEntity(core.NoEntity, RemapTop)
	*el = append(*el, en)

	en.Point[0] = g.RemapEntity(pt, RemapBottom)
	en.H = g.RemapEntity(core.NoEntity, RemapBottom)
	*el = append(*el, en)
}

// CopyEntity appends a transformed copy of ep (C++ Group::CopyEntity).
func (g *Group) CopyEntity(sk *Sketch, el *[]core.Entity,
	ep *core.Entity, timesApplied, remap int,
	dx, dy, dz, qw, qvx, qvy, qvz, dist core.HParam,
	as CopyAs) {
	g.copyEntityScaled(el, ep, timesApplied, remap, dx, dy, dz, qw, qvx, qvy, qvz, dist, as, g.Scale)
}

// copyEntityScaled is CopyEntity scaling by scale instead of the
// group's Scale: a mirror copies at -1 without being a scaled group.
func (g *Group) copyEntityScaled(el *[]core.Entity,
	ep *core.Entity, timesApplied, remap int,
	dx, dy, dz, qw, qvx, qvy, qvz, dist core.HParam,
	as CopyAs, scale float64) {
	en := core.Entity{
		Type:         ep.Type,
		ExtraPoints:  ep.ExtraPoints,
		H:            g.RemapEntity(ep.H, remap),
		TimesApplied: timesApplied,
		Group:        g.H,
		Construction: ep.Construction,
		Style:        ep.Style,
		Str:          ep.Str,
		Font:         ep.Font,
		File:         ep.File}

	switch ep.Type {
	case core.EntityWorkplane:
		// Don't copy these.
		return

	case core.EntityPointNCopy, core.EntityPointNTrans, core.EntityPointNRotTrans,
		core.EntityPointNRotAA, core.EntityPointNRotAxisTrans,
		core.EntityPointIn3D, core.EntityPointIn2D:
		if as == CopyAsNTrans {
			en.Type = core.EntityPointNTrans
			en.Param[0] = dx
			en.Param[1] = dy
			en.Param[2] = dz
		} else if as == CopyAsNumeric {
			en.Type = core.EntityPointNCopy
		} else {
			if as == CopyAsNRotAA {
				en.Type = core.EntityPointNRotAA
			} else if as == CopyAsNRotAxisTrans {
				en.Type = core.EntityPointNRotAxisTrans
			} else {
				en.Type = core.EntityPointNRotTrans
			}
			en.Param[0] = dx
			en.Param[1] = dy
			en.Param[2] = dz
			en.Param[3] = qw
			en.Param[4] = qvx
			en.Param[5] = qvy
			en.Param[6] = qvz
			if as == CopyAsNRotAxisTrans {
				en.Param[7] = dist
			}
		}
		en.NumPoint = ep.ActPoint.ScaledBy(scale)

	case core.EntityNormalNCopy, core.EntityNormalNRot, core.EntityNormalNRotAA,
		core.EntityNormalIn3D, core.EntityNormalIn2D:
		if as == CopyAsNTrans || as == CopyAsNumeric {
			en.Type = core.EntityNormalNCopy
		} else { // N_ROT_AXIS_TRANS probably doesn't warrant a new entity Type
			if as == CopyAsNRotAA || as == CopyAsNRotAxisTrans {
				en.Type = core.EntityNormalNRotAA
			} else {
				en.Type = core.EntityNormalNRot
			}
			en.Param[0] = qw
			en.Param[1] = qvx
			en.Param[2] = qvy
			en.Param[3] = qvz
		}
		en.NumNormal = ep.ActNormal
		if scale < 0 {
			en.NumNormal = en.NumNormal.Mirror()
		}

		en.Point[0] = g.RemapEntity(ep.Point[0], remap)

	case core.EntityDistanceNCopy, core.EntityDistance:
		en.Type = core.EntityDistanceNCopy
		en.NumDistance = ep.ActDistance * math.Abs(scale)

	case core.EntityFaceNormalPt, core.EntityFaceXProd, core.EntityFaceNRotTrans,
		core.EntityFaceNTrans, core.EntityFaceNRotAA, core.EntityFaceRotNormalPt,
		core.EntityFaceNRotAxisTrans:
		if as == CopyAsNTrans {
			en.Type = core.EntityFaceNTrans
			en.Param[0] = dx
			en.Param[1] = dy
			en.Param[2] = dz
		} else if as == CopyAsNumeric {
			en.Type = core.EntityFaceNormalPt
		} else if as == CopyAsNRotAxisTrans {
			en.Type = core.EntityFaceNRotAxisTrans
			en.Param[0] = dx
			en.Param[1] = dy
			en.Param[2] = dz
			en.Param[3] = qw
			en.Param[4] = qvx
			en.Param[5] = qvy
			en.Param[6] = qvz
			en.Param[7] = dist
		} else {
			if as == CopyAsNRotAA {
				en.Type = core.EntityFaceNRotAA
			} else {
				en.Type = core.EntityFaceNRotTrans
			}
			en.Param[0] = dx
			en.Param[1] = dy
			en.Param[2] = dz
			en.Param[3] = qw
			en.Param[4] = qvx
			en.Param[5] = qvy
			en.Param[6] = qvz
		}
		en.NumPoint = ep.ActPoint.ScaledBy(scale)
		en.NumNormal = ep.ActNormal.ScaledBy(scale)
		// the new face needs an associated point (old files may not have one)
		if ep.Point[0].V != 0 {
			en.Point[0] = g.RemapEntity(ep.Point[0], remap)
		}

	default:
		if ep.Type == core.EntityImage && ep.Construction {
			// Do not copy image entities if they are construction.
			return
		}

		_, points, hasNormal, hasDistance, ok := getEntityInfo(ep.Type, ep.ExtraPoints)
		if !ok {
			panic(fmt.Sprintf("no entity info for type %d", ep.Type))
		}
		for i := range points {
			en.Point[i] = g.RemapEntity(ep.Point[i], remap)
		}
		if hasNormal {
			en.Normal = g.RemapEntity(ep.Normal, remap)
		}
		if hasDistance {
			en.Distance = g.RemapEntity(ep.Distance, remap)
		}
	}

	// If the entity came from an linked file where it was invisible then
	// ep->actiVisble will be false, and we should hide it. Or if the entity
	// came from a copy (e.g. step and repeat) of a force-hidden linked
	// entity, then we also want to hide it.
	en.ForceHidden = !ep.ActVisible || ep.ForceHidden

	*el = append(*el, en)
}

// GenerateEquations appends the group's own equations (C++
// Group::GenerateEquations).
func (g *Group) GenerateEquations(sk *Sketch, l *[]core.Equation) {
	addEq := func(expr *core.Expr, index int) {
		*l = append(*l, core.Equation{H: g.H.Equation(index), Expr: expr})
	}
	ep := func(i int) *core.Expr { return core.NewParam(g.H.Param(i)) }
	ec := core.NewConstant

	switch {
	case g.Type == GroupLinked:
		// Normalize the quaternion
		q := core.ExprQuaternion{W: ep(3), X: ep(4), Y: ep(5), Z: ep(6)}
		addEq(q.Magnitude().Minus(ec(1)), 0)

	case g.Type == GroupRotate || g.Type == GroupRevolve || g.Type == GroupHelix:
		// The axis and center of rotation are specified numerically
		orig := sk.MustGetEntity(g.Predef.Origin).PointGetExprs(sk)
		addEq(orig.X.Minus(ep(0)), 0)
		addEq(orig.Y.Minus(ep(1)), 1)
		addEq(orig.Z.Minus(ep(2)), 2)
		// param 3 is the angle, which is free
		axis := sk.MustGetEntity(g.Predef.EntityB).VectorGetNum(sk)
		axis = axis.WithMagnitude(1)
		addEq(ec(axis.X).Minus(ep(4)), 3)
		addEq(ec(axis.Y).Minus(ep(5)), 4)
		addEq(ec(axis.Z).Minus(ep(6)), 5)
		if g.Type == GroupHelix {
			if g.ValB != 0.0 {
				addEq(ep(7).Times(ec(math.Pi)).
					Minus(ep(3).Times(ec(g.ValB))), 6)
			}
		}

	case g.Type == GroupExtrude && g.Subtype != GroupOneSkewed && g.Subtype != GroupTwoSkewed:
		if !core.IsFreeIn3D(g.Predef.EntityB) {
			// The extrusion path is locked along a line, normal to the
			// specified workplane. Don't constrain for skewed extrusions.
			w := sk.MustGetEntity(g.Predef.EntityB)
			u := w.NormalEntity(sk).NormalExprsU(sk)
			v := w.NormalEntity(sk).NormalExprsV(sk)
			extruden := core.ExprVector{X: ep(0), Y: ep(1), Z: ep(2)}

			addEq(u.Dot(extruden), 0)
			addEq(v.Dot(extruden), 1)
		}

	case g.Type == GroupMirror:
		g.mirrorEquations(sk, addEq)

	case g.Type == GroupTranslate:
		if !core.IsFreeIn3D(g.Predef.EntityB) {
			w := sk.MustGetEntity(g.Predef.EntityB)
			n := w.NormalEntity(sk).NormalExprsN(sk)
			trans := core.ExprVector{X: ep(0), Y: ep(1), Z: ep(2)}

			// The translation vector is parallel to the workplane
			addEq(trans.Dot(n), 0)
		}
	}
}

// GenerateConstraint adds the params the constraint needs (C++
// ConstraintBase::Generate).
func GenerateConstraint(c *core.Constraint, l *[]core.Param) {
	switch c.Type {
	case core.ConstraintParallel, core.ConstraintCubicLineTangent:
		// Add new parameter only when we operate in 3d space
		if !core.IsFreeIn3D(c.Workplane) {
			break
		}
		fallthrough
	case core.ConstraintSameOrientation, core.ConstraintPtOnLine:
		c.ValP = c.H.Param(0)
		*l = append(*l, core.Param{H: c.ValP})
	}
}
