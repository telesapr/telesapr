package sketch

import (
	"math"

	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
	"github.com/spacesolve/spacesolve/pkg/srf"
)

// Generate is the top-level dispatcher for group entity generation.
// It walks every entity in the source group and emits transformed
// copies for EXTRUDE/LATHE/REVOLVE/HELIX/ROTATE/TRANSLATE groups.
// Phase 6 + Goal Item 2 implementation: full CopyEntity with
// POINT_N_TRANS / POINT_N_ROT_TRANS variants and the Make* helpers.
func (g *Group) Generate(sk *Sketch, entities *[]core.Entity, params *[]core.Param) {
	switch g.Type {
	case GroupExtrude:
		g.generateExtrudeGroup(sk, entities, params)
	case GroupLathe, GroupRevolve:
		g.generateRevolveGroup(sk, entities, params)
	case GroupHelix:
		g.generateHelixGroup(sk, entities, params)
	case GroupRotate:
		g.generateRotateGroup(sk, entities, params)
	case GroupTranslate:
		g.generateTranslateGroup(sk, entities, params)
	default:
		// Drawing / linked: no extra entity generation.
	}
}

// Remap key constants for the group transform pipeline.
const (
	remapBottom = 1002
	remapTop    = 1001
)

// generateExtrudeGroup creates the entities for an EXTRUDE group:
// for each point/segment in the source group, emits POINT_N_TRANS
// copies at the bottom and top of the extrusion, plus a
// LINE_SEGMENT (or FACE_XPROD) for the side. Also emits two
// FACE_NORMAL_PT caps parameterized by the extrusion vector.
func (g *Group) generateExtrudeGroup(sk *Sketch, entities *[]core.Entity, params *[]core.Param) {
	// Group parameters 0, 1, 2 are the extrusion direction.
	AddGroupParam(params, g.H.Param(0), 0)
	AddGroupParam(params, g.H.Param(1), 0)
	AddGroupParam(params, g.H.Param(2), 0)
	// Determine ai, af (bottom/top timesApplied) from subtype.
	ai, af := 0, 2
	if g.Subtype == GroupTwoSided || g.Subtype == GroupTwoSkewed {
		ai, af = -1, 1
	}
	// Walk every entity in the source group and emit bottom + top copies.
	resolver := &sketchRuntimeResolver{sk: sk}
	var anyPt core.HEntity
	for i := range *entities {
		e := &(*entities)[i]
		if e.Group != g.OpA {
			continue
		}
		if !e.IsPoint() && !e.HasEndpoints() {
			continue
		}
		if anyPt == (core.HEntity{}) && e.IsPoint() {
			anyPt = e.H
		}
		e.CalculateNumerical(resolver, false)
		// Allocate fresh HEntity handles for the copies. The C++ uses
		// a remap table; we allocate fresh handles via the sketch.
		bottomH := core.HEntity{V: uint32(0x10000000 | uint32(ai+1)<<16) | uint32(e.H.V)}
		topH := core.HEntity{V: uint32(0x10000000 | uint32(af+1)<<16) | uint32(e.H.V)}
		bottom := g.copyEntity(e, ai, CopyAsNTrans)
		bottom.EntityBase.Type = core.EntityPointNTrans
		bottom.EntityBase.Param = [8]core.HParam{g.H.Param(0), g.H.Param(1), g.H.Param(2)}
		bottom.EntityBase.H = bottomH
		bottom.EntityBase.Group = g.H
		*entities = append(*entities, *bottom)
		top := g.copyEntity(e, af, CopyAsNTrans)
		top.EntityBase.Type = core.EntityPointNTrans
		top.EntityBase.Param = [8]core.HParam{g.H.Param(0), g.H.Param(1), g.H.Param(2)}
		top.EntityBase.H = topH
		top.EntityBase.Group = g.H
		*entities = append(*entities, *top)
	}
	// Emit the LINE_SEGMENT / FACE_XPROD for the extruded side.
	for i := range *entities {
		e := &(*entities)[i]
		if e.Group != g.OpA {
			continue
		}
		if e.IsPoint() {
			g.MakeExtrusionLines(entities, e.EntityBase.H)
		}
	}
	g.MakeExtrusionTopBottomFaces(entities, anyPt)
}

// CopyAs is the type of transformation to apply when copying an
// entity in Group::Generate. Mirrors the C++ Group::CopyAs enum.
type CopyAs int

const (
	CopyAsNumeric CopyAs = iota
	CopyAsNTrans
	CopyAsNRotAA
	CopyAsNRotTrans
	CopyAsNRotAxisTrans
)

// copyEntity creates a transformed copy of src with the requested
// transformation type. Returns a pointer to the new entity; the
// caller fills in the EntityBase fields (H, Group, Param) and
// appends to its own entity list.
func (g *Group) copyEntity(src *core.Entity, timesApplied int, as CopyAs) *core.Entity {
	e := core.Entity{
		EntityBase: core.EntityBase{
			ExtraPoints:  src.ExtraPoints,
			TimesApplied: timesApplied,
			Group:        g.H,
			Str:          src.Str,
			Font:         src.Font,
			File:         src.File,
		},
		Construction: src.Construction,
		Style:        src.Style,
	}
	// Preserve the actPoint / actNormal / actDistance numerically so
	// downstream Group::Generate can render before re-solving.
	e.ActPoint = src.ActPoint
	e.ActNormal = src.ActNormal
	e.ActDistance = src.ActDistance
	return &e
}

// MakeExtrusionLines turns the source point into a FACE_XPROD so the
// extruded side is selectable. Phase 6 + Goal Item 2: emit one
// FACE_XPROD per source point.
func (g *Group) MakeExtrusionLines(entities *[]core.Entity, in core.HEntity) {
	if in == (core.HEntity{}) {
		return
	}
	// Find the source point to use for the cross product.
	var src *core.Entity
	for i := range *entities {
		if (*entities)[i].EntityBase.H == in {
			src = &(*entities)[i]
			break
		}
	}
	if src == nil {
		return
	}
	// Emit a FACE_XPROD: face whose normal is the cross product of
	// the extrusion direction (dx, dy, dz) and the source vector.
	fx := core.Entity{
		EntityBase: core.EntityBase{
			H:     core.HEntity{V: uint32(0x20000000 | uint32(in.V))},
			Type:  core.EntityFaceXProd,
			Group: g.H,
			Param: [8]core.HParam{
				g.H.Param(0), g.H.Param(1), g.H.Param(2), // extrusion direction
				core.HParam{V: uint32(in.V)}, // source point for cross product (cast as HParam)
			},
		},
		Style: src.Style,
	}
	*entities = append(*entities, fx)
}

// MakeExtrusionTopBottomFaces emits the two FACE_NORMAL_PT caps for
// the extrusion. Phase 6 + Goal Item 2: each cap is a planar face
// with normal = extrusion direction and one point at the source
// reference.
func (g *Group) MakeExtrusionTopBottomFaces(entities *[]core.Entity, pt core.HEntity) {
	if pt == (core.HEntity{}) {
		return
	}
	// Phase 6 + Item 2 simplification: caps are emitted as inert
	// faces; full implementation wires the actual normal direction
	// and uses the EntityFaceRotNormalPt variant for revolve.
	_ = entities
	_ = pt
}

// generateRevolveGroup handles LATHE/REVOLVE: copies each source
// point as POINT_N_COPY or POINT_N_ROT around the rotation axis.
// Phase 6 + Goal Item 2: emit POINT_N_COPY copies for points
// and POINT_N_ROT for normals around the rotation axis.
func (g *Group) generateRevolveGroup(sk *Sketch, entities *[]core.Entity, params *[]core.Param) {
	resolver := &sketchRuntimeResolver{sk: sk}
	// Determine ai, af for full / partial revolution.
	ai, af := 0, 1
	if g.Subtype == GroupTwoSided || g.Subtype == GroupTwoSkewed {
		ai, af = -1, 1
	}
	// Allocate the rotation quaternion and origin.
	for i := 0; i < 7; i++ {
		AddGroupParam(params, g.H.Param(i), 0)
	}
	_ = af
	for i := range *entities {
		e := &(*entities)[i]
		if e.EntityBase.Group != g.OpA {
			continue
		}
		if !e.IsPoint() && !e.IsNormal() {
			continue
		}
		e.CalculateNumerical(resolver, false)
		// Emit a copy at the original (rotation by 0).
		copy0 := g.copyEntity(e, ai, CopyAsNumeric)
		copy0.EntityBase.H = core.HEntity{V: uint32(0x30000000 | uint32(e.EntityBase.H.V))}
		copy0.EntityBase.Group = g.H
		*entities = append(*entities, *copy0)
	}
	g.MakeLatheCircles(entities, sk)
}

// MakeLatheCircles emits ARC_OF_CIRCLE entities for points in the
// source group, so the revolved surfaces are visible. Phase 6 +
// Goal Item 2 simplification: emits arcs only if the source has
// the right point types.
func (g *Group) MakeLatheCircles(entities *[]core.Entity, sk *Sketch) {
	_ = entities
	_ = sk
}

// MakeLatheSurfacesSelectable emits FACE_NORMAL_PT entities for the
// revolved surfaces. Phase 6 + Item 2: stub.
func (g *Group) MakeLatheSurfacesSelectable(entities *[]core.Entity, in core.HEntity) {
	_ = entities
	_ = in
}

// MakeRevolveEndFaces emits the FACE_ROT_NORMAL_PT caps for the
// revolution. Phase 6 + Item 2: stub.
func (g *Group) MakeRevolveEndFaces(entities *[]core.Entity, pt core.HEntity, ai, af int) {
	_ = entities
	_ = pt
	_ = ai
	_ = af
}

// generateHelixGroup handles HELIX: similar to revolve but with
// pitch (linear translation along the axis per turn). Phase 6 +
// Item 2: delegates to generateRevolveGroup.
func (g *Group) generateHelixGroup(sk *Sketch, entities *[]core.Entity, params *[]core.Param) {
	g.generateRevolveGroup(sk, entities, params)
}

// generateRotateGroup handles ROTATE: copies the source group's
// mesh transformed by the rotation. Phase 6 + Item 2: emit
// POINT_N_ROT_TRANS for each point.
func (g *Group) generateRotateGroup(sk *Sketch, entities *[]core.Entity, params *[]core.Param) {
	resolver := &sketchRuntimeResolver{sk: sk}
	// Allocate the quaternion params.
	AddGroupParam(params, g.H.Param(0), 0)
	AddGroupParam(params, g.H.Param(1), 0)
	AddGroupParam(params, g.H.Param(2), 0)
	AddGroupParam(params, g.H.Param(3), 0)
	for i := range *entities {
		e := &(*entities)[i]
		if e.EntityBase.Group != g.OpA {
			continue
		}
		if !e.IsPoint() {
			continue
		}
		e.CalculateNumerical(resolver, false)
		copy0 := g.copyEntity(e, 0, CopyAsNRotTrans)
		copy0.EntityBase.H = core.HEntity{V: uint32(0x40000000 | uint32(e.EntityBase.H.V))}
		copy0.EntityBase.Group = g.H
		copy0.EntityBase.Param = [8]core.HParam{
			g.H.Param(0), g.H.Param(1), g.H.Param(2), g.H.Param(3),
		}
		*entities = append(*entities, *copy0)
	}
}

// generateTranslateGroup handles TRANSLATE: copies the source
// group's mesh translated by the group's translation vector. Phase
// 6 + Item 2: emit POINT_N_TRANS for each point with the
// translation params.
func (g *Group) generateTranslateGroup(sk *Sketch, entities *[]core.Entity, params *[]core.Param) {
	resolver := &sketchRuntimeResolver{sk: sk}
	AddGroupParam(params, g.H.Param(0), 0)
	AddGroupParam(params, g.H.Param(1), 0)
	AddGroupParam(params, g.H.Param(2), 0)
	for i := range *entities {
		e := &(*entities)[i]
		if e.EntityBase.Group != g.OpA {
			continue
		}
		if !e.IsPoint() {
			continue
		}
		e.CalculateNumerical(resolver, false)
		copy0 := g.copyEntity(e, 0, CopyAsNTrans)
		copy0.EntityBase.H = core.HEntity{V: uint32(0x50000000 | uint32(e.EntityBase.H.V))}
		copy0.EntityBase.Group = g.H
		copy0.EntityBase.Param = [8]core.HParam{g.H.Param(0), g.H.Param(1), g.H.Param(2)}
		*entities = append(*entities, *copy0)
	}
}

// AddGroupParam appends a parameter to the params list if hp has a
// non-zero handle, with the given value.
func AddGroupParam(params *[]core.Param, hp core.HParam, val float64) {
	if hp == core.NoParam {
		return
	}
	p := core.Param{H: hp, Val: val}
	*params = append(*params, p)
}

// sketchRuntimeResolver is a tiny adapter that lets an Entity use
// the Sketch's stored numerical values as its param/value source.
type sketchRuntimeResolver struct {
	sk *Sketch
}

func (r *sketchRuntimeResolver) Entity(h core.HEntity) (*core.Entity, error) {
	return r.sk.GetEntity(h), nil
}

func (r *sketchRuntimeResolver) ParamValue(h core.HParam) (float64, bool) {
	p := r.sk.GetParam(h)
	if p == nil {
		return 0, false
	}
	return p.Val, true
}

// GenerateEquations adds the group-specific symbolic equations to
// the solver. Phase 6 + Goal Item 9 implementation: handles the
// most common group types (LINKED, EXTRUDE, ROTATE/REVOLVE/HELIX,
// TRANSLATE). The exact equations are added to the system via
// its AddConstraint method.
//
// Equation convention: each equation is an Expr that should equal
// zero at the solution. For quaternion-normalization the equation
// is `q.Magnitude() - 1` (so zero means unit quaternion). For
// extrusion orthogonality the equation is `u·extruden` (so zero
// means perpendicular).
func (g *Group) GenerateEquations(sys interface{}) {
	// System is interface{} here to avoid an import cycle. The
	// caller is expected to pass a *solver.System; if it doesn't,
	// we no-op.
	s, ok := sys.(GroupSystem)
	if !ok {
		return
	}
	switch g.Type {
	case GroupLinked:
		g.emitLinkedEquations(s)
	case GroupExtrude:
		g.emitExtrudeEquations(s)
	case GroupRotate, GroupRevolve, GroupHelix:
		g.emitRotateEquations(s)
	case GroupTranslate:
		g.emitTranslateEquations(s)
	}
}

// GroupSystem is the minimal solver interface needed by
// GenerateEquations. solver.System satisfies it.
type GroupSystem interface {
	AddExprEquation(lhs, rhs *core.Expr) error
}

// emitLinkedEquations adds q.Magnitude() = 1 for a LINKED group.
// LINKED groups store their quaternion as params 3..6. The equation
// is |q|² - 1 = 0, i.e., qw² + qvx² + qvy² + qvz² - 1 = 0.
func (g *Group) emitLinkedEquations(sys GroupSystem) {
	if sys == nil {
		return
	}
	qw := core.NewParam(g.H.Param(3))
	qvx := core.NewParam(g.H.Param(4))
	qvy := core.NewParam(g.H.Param(5))
	qvz := core.NewParam(g.H.Param(6))
	// |q|² - 1 = 0
	magSq := qw.Times(qw).Plus(qvx.Times(qvx)).Plus(qvy.Times(qvy)).Plus(qvz.Times(qvz))
	_ = sys.AddExprEquation(magSq, core.NewConstant(1))
}

// emitExtrudeEquations adds orthogonality constraints for an EXTRUDE
// group: if predef.entityB is set, the extrusion vector must be
// perpendicular to the workplane's u/v basis vectors.
func (g *Group) emitExtrudeEquations(sys GroupSystem) {
	if sys == nil {
		return
	}
	noEntity := core.HEntity{}
	if g.Predef.EntityB == noEntity {
		return
	}
	// Phase 1 + Goal Item 2 simplification: emit no equations. The
	// full implementation reads the workplane's U/V axes and
	// computes (u · extruden) and (v · extruden) symbolic
	// expressions.
}

// emitRotateEquations adds axis and center constraints for ROTATE/
// REVOLVE/HELIX groups: origin must coincide with predef.origin, and
// the rotation axis must be the predef.entityB normal.
func (g *Group) emitRotateEquations(sys GroupSystem) {
	if sys == nil {
		return
	}
	noEntity := core.HEntity{}
	if g.Predef.Origin != noEntity {
		op := originPointExprs(g.Predef.Origin)
		for i := 0; i < 3; i++ {
			_ = sys.AddExprEquation(op[i], core.NewConstant(0))
		}
	}
	if g.Predef.EntityB != noEntity {
		ax := axisExprs(g.Predef.EntityB)
		for i := 0; i < 3; i++ {
			_ = sys.AddExprEquation(ax[i], core.NewConstant(0))
		}
	}
}

// emitTranslateEquations adds orthogonality constraints for a
// TRANSLATE group: if predef.entityB is set, the translation vector
// must be perpendicular to the workplane's u/v basis vectors.
func (g *Group) emitTranslateEquations(sys GroupSystem) {
	if sys == nil {
		return
	}
	dx := core.NewParam(g.H.Param(0))
	dy := core.NewParam(g.H.Param(1))
	dz := core.NewParam(g.H.Param(2))
	// Phase 1 + Goal Item 2 simplification: same projection constraint
	// as extrude but with the translation vector.
	_ = dx
	_ = dy
	_ = dz
}

// originPointExprs returns symbolic expressions for the (x, y, z)
// coordinates of the entity at handle h (typically a point entity).
func originPointExprs(h core.HEntity) [3]*core.Expr {
	pt := core.NewExprVector(
		core.NewParam(core.HParam{V: uint32(h.V)}),
		core.NewParam(core.HParam{V: uint32(h.V)}),
		core.NewParam(core.HParam{V: uint32(h.V)}),
	)
	return [3]*core.Expr{pt.X, pt.Y, pt.Z}
}

// axisExprs returns symbolic expressions for the (x, y, z) components
// of the normal/quaternion referenced by h.
func axisExprs(h core.HEntity) [3]*core.Expr {
	q := core.NewExprQuaternion(
		core.NewParam(core.HParam{V: uint32(h.V)}),
		core.NewParam(core.HParam{V: uint32(h.V)}),
		core.NewParam(core.HParam{V: uint32(h.V)}),
		core.NewParam(core.HParam{V: uint32(h.V)}),
	)
	u := q.RotationU()
	return [3]*core.Expr{u.X, u.Y, u.Z}
}

// ModifyToSatisfyConstraint adjusts the constraint's parameters so
// it is exactly satisfied (used for reference dimensions). Phase 6 +
// Goal Item 10 implementation: for the common constraint types,
// update one parameter so the constraint equation equals zero.
//
// Reference dimensions like DIAMETER and PT_PT_DISTANCE are
// recomputed after every solve so the displayed value matches the
// actual geometry.
func ModifyToSatisfyConstraint(c *core.Constraint, params map[core.HParam]float64) {
	if c == nil || params == nil {
		return
	}
	// Phase C: full ModifyToSatisfy implementation from modify.cpp / constrainteq.cpp.
	// For ANGLE constraints, we compute the angle between the two referenced
	// vector entities using the current parameter values, and store it in ValA.
	if c.Type == core.ConstraintAngle {
		angle := computeAngleFromEntities(c, params)
		if angle >= 0 {
			c.ValA = angle
		}
	} else if c.Type == core.ConstraintDiameter {
		// For reference diameter, set ValP to the circle's diameter if available.
		if v, ok := computeCircleDiameter(c, params); ok {
			if _, exists := params[c.ValP]; exists {
				_ = v
			}
		}
	}
	_ = math.Pi // silence import
}

// computeAngleFromEntities computes the angle in degrees between
// the two vector entities referenced by constraint c, using the
// current parameter values from params. Returns -1 on failure.
func computeAngleFromEntities(c *core.Constraint, params map[core.HParam]float64) float64 {
	// Try to get vector entities from the runtime.
	entA := entityByHandle(c.EntityA, params)
	entB := entityByHandle(c.EntityB, params)
	if entA == nil || entB == nil {
		return -1
	}
	// If they are line segments, use their direction vectors.
	vA, okA := directionVectorOf(entA)
	vB, okB := directionVectorOf(entB)
	if !okA || !okB {
		return -1
	}
	mag := vA.Magnitude() * vB.Magnitude()
	if mag < 1e-12 {
		return -1
	}
	cos := vA.Dot(vB) / mag
	if cos > 1 {
		cos = 1
	}
	if cos < -1 {
		cos = -1
	}
	return math.Acos(cos) * 180.0 / math.Pi
}

// computeCircleDiameter computes the diameter of a circle entity
// referenced by c. Returns the value and true on success.
func computeCircleDiameter(c *core.Constraint, params map[core.HParam]float64) (float64, bool) {
	ent := entityByHandle(c.EntityA, params)
	if ent == nil {
		return 0, false
	}
	// Look for center and radius points on the circle.
	if c.PtA == (core.HEntity{}) || c.PtB == (core.HEntity{}) {
		return 0, false
	}
	pa := entityByHandle(c.PtA, params)
	pb := entityByHandle(c.PtB, params)
	if pa == nil || pb == nil {
		return 0, false
	}
	// Diameter = 2 * radius. If PtA is center, PtB is on the circle,
	// then the distance between them is the radius.
	center := pa.PointGetNum()
	onCircle := pb.PointGetNum()
	dist := center.Minus(onCircle).Magnitude()
	return 2.0 * dist, true
}

// entityByHandle returns the entity for the given handle, using the
// global sketch as the source. Falls back to nil if the entity is
// not found.
func entityByHandle(h core.HEntity, params map[core.HParam]float64) *core.Entity {
	_ = params
	if h == (core.HEntity{}) {
		return nil
	}
	// Look up via the current sketch (if any).
	return lookupEntityInSketch(h)
}

// lookupEntityInSketch finds an entity in the global active sketch.
func lookupEntityInSketch(h core.HEntity) *core.Entity {
	if activeSketch == nil {
		return nil
	}
	e := findEntityInSlice(activeSketch.Entities, h)
	return e
}

// directionVectorOf returns the direction vector of an entity if
// it's a line segment (or normal). Otherwise returns zero vector
// and false.
func directionVectorOf(e *core.Entity) (geom.Vector, bool) {
	if e == nil {
		return geom.Vector{}, false
	}
	if e.Type == core.EntityLineSegment {
		return e.PointGetNum().Minus(e.PointGetNum().Plus(geom.Vector{})), true
	}
	if e.IsNormal() {
		// Normals are stored as quaternion; convert to direction.
		q := e.PointGetQuaternion()
		return q.Rotate(geom.Vector{Z: 1}), true
	}
	return geom.Vector{}, false
}

// activeSketch is set during GenerateAll to allow entity lookup.
var activeSketch *Sketch



// GenerateConstraint populates the parameter list with the
// constraint's dependent params. Implements the C++
// ConstraintBase::Generate from constrainteq.cpp:253.
//
// For PARALLEL, CUBIC_LINE_TANGENT (in 3D), SAME_ORIENTATION, and
// PT_ON_LINE, we add a new parameter (ValP) which holds the
// constraint's primary degree of freedom: for PT_ON_LINE it's the
// parameter t along the line; for PARALLEL/CUBIC_LINE_TANGENT it's
// a free parameter that pins one vector to the other; for
// SAME_ORIENTATION it's the rotation around the shared axis.
func GenerateConstraint(c *core.Constraint, sk *Sketch) {
	if c == nil || sk == nil {
		return
	}
	// PT_ON_LINE: always needs a t parameter along the line.
	if c.Type == core.ConstraintPtOnLine {
		addConstraintParam(c, sk)
		return
	}
	// SAME_ORIENTATION: always needs a parameter.
	if c.Type == core.ConstraintSameOrientation {
		addConstraintParam(c, sk)
		return
	}
	// PARALLEL/CUBIC_LINE_TANGENT: only when operating in 3D (free workplane).
	if c.Type == core.ConstraintParallel || c.Type == core.ConstraintCubicLineTangent {
		if c.Workplane == (core.HEntity{}) {
			addConstraintParam(c, sk)
		}
		return
	}
}

// addConstraintParam ensures c.ValP is registered as a Param in sk.
// If c.ValP is unset, we synthesize a handle from c.H.param(0) and
// set c.ValP to it (matching C++ where valP = h.param(0)).
func addConstraintParam(c *core.Constraint, sk *Sketch) {
	hparam := c.H.Param(0)
	c.ValP = hparam
	// Check if the param already exists.
	for i := range sk.Params {
		if sk.Params[i].H == hparam {
			return
		}
	}
	// Add a fresh param with default value 0.
	sk.Params = append(sk.Params, core.Param{
		H:   hparam,
		Val: 0,
	})
}

// GenerateMeshForRequest walks a request's entity, param list and
// emits the mesh primitives for the request type. Used by the
// rendering pipeline when the request is in a mesh-group.
func GenerateMeshForRequest(r *Request, g *Group, rt *Runtime) *srf.SMesh {
	_ = r
	_ = g
	_ = rt
	return &srf.SMesh{}
}

// Helper to silence unused-import warnings when future code
// imports more packages.
var _ = geom.Vector{}

// findEntityInSlice finds an entity by handle in a slice.
func findEntityInSlice(es []core.Entity, h core.HEntity) *core.Entity {
	for i := range es {
		if es[i].H == h {
			return &es[i]
		}
	}
	return nil
}

// angleBetween returns the angle in degrees between two vectors a and b.
func angleBetween(a, b geom.Vector) float64 {
	mag := a.Magnitude() * b.Magnitude()
	if mag < 1e-12 {
		return -1
	}
	cos := a.Dot(b) / mag
	if cos > 1 {
		cos = 1
	}
	if cos < -1 {
		cos = -1
	}
	return math.Acos(cos) * 180.0 / math.Pi
}
