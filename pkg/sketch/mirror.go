package sketch

import (
	"errors"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// A mirror group copies its source as a link group with a scale of -1
// is placed: a reflection in the plane through o with unit normal
// n is the point inversion p -> -p, then the half turn about n, then
// the translation by 2(o.n)n. Its parameters are that translation (0
// to 2) and the half turn's quaternion (3 to 6, w = 0), and its
// equations tie them to the plane, so the copies follow the plane
// when it moves.

// MirrorPlaneOK reports whether he can be the plane of a mirror: a
// workplane or a planar face.
func (sk *Sketch) MirrorPlaneOK(he core.HEntity) bool {
	e := sk.GetEntity(he)
	return e != nil && (e.IsWorkplane() || e.IsFace())
}

// mirrorPlaneNum is the point on and the unit normal of a mirror
// plane.
func mirrorPlaneNum(sk *Sketch, e *core.Entity) (o, n geom.Vector) {
	if e.IsWorkplane() {
		return e.WorkplaneGetOffset(sk), e.NormalEntity(sk).NormalN(sk).WithMagnitude(1)
	}
	return e.FaceGetPointNum(sk), e.FaceGetNormalNum(sk)
}

// mirrorPlaneExprs is mirrorPlaneNum as expressions of the plane's
// parameters.
func mirrorPlaneExprs(sk *Sketch, e *core.Entity) (o, n core.ExprVector) {
	if e.IsWorkplane() {
		return e.WorkplaneGetOffsetExprs(sk), e.NormalEntity(sk).NormalExprsN(sk)
	}
	return e.FaceGetPointExprs(sk), e.FaceGetNormalExprs()
}

// mirrorPlane is the group's plane entity, or nil when Predef.EntityB
// is not a workplane or a face.
func (g *Group) mirrorPlane(sk *Sketch) *core.Entity {
	if !sk.MirrorPlaneOK(g.Predef.EntityB) {
		return nil
	}
	return sk.GetEntity(g.Predef.EntityB)
}

// NewMirror starts a mirror group that reflects the active group in
// plane, a workplane or a planar face. It is what the interface and
// scripts add with AddGroupAfterActive.
func (sk *Sketch) NewMirror(plane core.HEntity, name string) (Group, error) {
	g := NewGroup()
	g.Type = GroupMirror
	g.Name = name
	if name == "" {
		g.Name = "mirror"
	}
	src := sk.GetGroup(sk.Active)
	if src == nil || src.H.V == HGroupReferences {
		return g, errors.New("activate the group to mirror first: the active group is the one mirrored")
	}
	if !sk.MirrorPlaneOK(plane) {
		return g, errors.New("a mirror reflects in a plane: select a workplane or a planar face")
	}
	g.OpA = src.H
	g.Predef.EntityB = plane
	g.Subtype = GroupOneSided
	return g, nil
}

// generateMirror makes the group's parameters and the reflected copies
// of its source group's entities.
func (g *Group) generateMirror(sk *Sketch, entity *[]core.Entity, param *[]core.Param) {
	h := g.H
	if src := sk.GetGroup(g.OpA); src != nil {
		g.MeshCombine = src.MeshCombine
	}
	o, n := geom.Vector{}, geom.Vector{Z: 1}
	if e := g.mirrorPlane(sk); e != nil {
		o, n = mirrorPlaneNum(sk, e)
	}
	t := n.ScaledBy(2 * o.Dot(n))
	AddParam(param, h.Param(0), t.X)
	AddParam(param, h.Param(1), t.Y)
	AddParam(param, h.Param(2), t.Z)
	AddParam(param, h.Param(3), 0)
	AddParam(param, h.Param(4), n.X)
	AddParam(param, h.Param(5), n.Y)
	AddParam(param, h.Param(6), n.Z)

	for i := 0; i < len(*entity); i++ {
		e := &(*entity)[i]
		if e.Group != g.OpA {
			continue
		}
		e.CalculateNumerical(sk, false)
		g.copyEntityScaled(sk, entity, e, 0, 0,
			h.Param(0), h.Param(1), h.Param(2),
			h.Param(3), h.Param(4), h.Param(5), h.Param(6), core.NoParam,
			CopyAsNRotTrans, -1)
	}
}

// mirrorEquations ties the group's parameters to its plane.
func (g *Group) mirrorEquations(sk *Sketch, addEq func(*core.Expr, int)) {
	e := g.mirrorPlane(sk)
	if e == nil {
		return
	}
	ep := func(i int) *core.Expr { return core.NewParam(g.H.Param(i)) }
	o, n := mirrorPlaneExprs(sk, e)
	addEq(ep(3), 0)
	addEq(n.X.Minus(ep(4)), 1)
	addEq(n.Y.Minus(ep(5)), 2)
	addEq(n.Z.Minus(ep(6)), 3)
	qv := core.ExprVector{X: ep(4), Y: ep(5), Z: ep(6)}
	k := o.Dot(qv).Times(core.NewConstant(2))
	addEq(ep(0).Minus(ep(4).Times(k)), 4)
	addEq(ep(1).Minus(ep(5).Times(k)), 5)
	addEq(ep(2).Minus(ep(6).Times(k)), 6)
}

// mirrorTransform is the translation and quaternion that, after a
// scale of -1, reflect in the group's plane.
func (g *Group) mirrorTransform(sk *Sketch) (geom.Vector, geom.Quaternion) {
	t := geom.Vector{
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
	return t, q
}

// checkMirror records what is wrong with the group's input.
func (g *Group) checkMirror(sk *Sketch) bool {
	if g.mirrorPlane(sk) == nil {
		g.setProblem(nil, "the mirror plane %s is not a workplane or a planar face", sk.entityName(g.Predef.EntityB))
		return false
	}
	return true
}


// generateMirrorShell reflects the source's shell and, unless
// SkipFirst, combines it with the original as the source combines.
func (g *Group) generateMirrorShell(sk *Sketch, src *srf.SShell, out *srf.SShell, forWhat CombineAs) {
	t, q := g.mirrorTransform(sk)
	var m srf.SShell
	m.MakeFromTransformationOf(src, t, q, -1)
	m.RemapFaces(shellGroup{g: g, sk: sk}, 0)
	out.Clear()
	if g.SkipFirst {
		*out = m
		return
	}
	var orig srf.SShell
	orig.MakeFromCopyOf(src)
	if forWhat == CombineAssemble {
		out.MakeFromAssemblyOf(&orig, &m)
	} else {
		out.MakeFromUnionOf(&orig, &m)
	}
}

// generateMirrorMesh is generateMirrorShell for a group forced to a
// mesh.
func (g *Group) generateMirrorMesh(sk *Sketch, src *srf.SMesh, out *srf.SMesh, forWhat CombineAs) {
	t, q := g.mirrorTransform(sk)
	var m srf.SMesh
	m.MakeFromTransformationOf(src, t, q, -1)
	m.RemapFaces(shellGroup{g: g, sk: sk}, 0)
	out.Clear()
	if g.SkipFirst {
		*out = m
		return
	}
	if forWhat == CombineAssemble {
		out.MakeFromAssemblyOf(src, &m)
	} else {
		out.MakeFromUnionOf(src, &m)
	}
}
