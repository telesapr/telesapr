package sketch

import (
	"github.com/spacesolve/spacesolve/pkg/core"
	"github.com/spacesolve/spacesolve/pkg/geom"
)

// UpgradeLegacyData brings a freshly loaded sketch up to date, and
// regenerates its entities without solving (C++
// SolveSpaceUI::UpgradeLegacyData, which LoadFromFile ends with).
func (sk *Sketch) UpgradeLegacyData() {
	for ri := range sk.Requests {
		r := &sk.Requests[ri]
		if r.Type != RequestTTFText {
			continue
		}
		// TTF text requests saved in versions prior to 3.0 only have two
		// reference points (origin and origin plus v); version 3.0 adds two
		// more points, and if we don't do anything, then they will appear
		// at workplane origin, and the solver will mess up the sketch if
		// it is not fully constrained.
		var entity core.EntityList
		var param core.ParamList
		r.Generate(sk, &entity, &param)

		// If we didn't load all of the entities and params that this
		// request would generate, then add them now, so that we can
		// force them to their appropriate positions.
		for _, p := range param {
			if sk.GetParam(p.H) != nil {
				continue
			}
			sk.Params = append(sk.Params, p)
		}
		allPointsExist := true
		for _, e := range entity {
			if sk.GetEntity(e.H) != nil {
				continue
			}
			sk.Entities = append(sk.Entities, e)
			allPointsExist = false
		}

		if !allPointsExist {
			text := sk.MustGetEntity(r.H.Entity(0))
			b := sk.MustGetEntity(text.Point[2])
			c := sk.MustGetEntity(text.Point[3])
			bex, cex := text.RectGetPointsExprs(sk)
			b.PointForceParamTo(sk, evalVector(sk, bex))
			c.PointForceParamTo(sk, evalVector(sk, cex))
		}
	}

	// Constraints saved in versions prior to 3.0 never had any params;
	// version 3.0 introduced params to constraints to avoid the hairy ball problem,
	// so force them where they belong.
	oldParam := make(map[core.HParam]bool, len(sk.Params))
	for _, p := range sk.Params {
		oldParam[p.H] = true
	}
	sk.GenerateAllTyped(GenerateRegen)

	allParamsExistFor := func(c *core.Constraint) bool {
		var param []core.Param
		cc := *c
		GenerateConstraint(&cc, &param)
		for _, p := range param {
			if !oldParam[p.H] {
				return false
			}
		}
		return true
	}

	for ci := range sk.Constraints {
		c := &sk.Constraints[ci]
		switch c.Type {
		case core.ConstraintPtOnLine:
			if allParamsExistFor(c) {
				continue
			}

			eln := sk.MustGetEntity(c.EntityA)
			ea := sk.MustGetEntity(eln.Point[0])
			eb := sk.MustGetEntity(eln.Point[1])
			ep := sk.MustGetEntity(c.PtA)

			exp := ep.PointGetExprsInWorkplane(sk, c.Workplane)
			exa := ea.PointGetExprsInWorkplane(sk, c.Workplane)
			exb := eb.PointGetExprsInWorkplane(sk, c.Workplane)
			exba := exb.Minus(exa)
			p := sk.MustGetParam(c.H.Param(0))
			p.Val = core.EvalExpr(sk, exba.Dot(exp.Minus(exa))) / core.EvalExpr(sk, exba.Dot(exba))

		case core.ConstraintCubicLineTangent:
			if allParamsExistFor(c) {
				continue
			}

			cubic := sk.MustGetEntity(c.EntityA)
			line := sk.MustGetEntity(c.EntityB)

			var a core.ExprVector
			if c.Other {
				a = cubic.CubicGetFinishTangentExprs(sk)
			} else {
				a = cubic.CubicGetStartTangentExprs(sk)
			}

			b := line.VectorGetExprs(sk)

			p := sk.MustGetParam(c.H.Param(0))
			p.Val = core.EvalExpr(sk, a.Dot(b)) / core.EvalExpr(sk, b.Dot(b))

		case core.ConstraintSameOrientation:
			if allParamsExistFor(c) {
				continue
			}

			an := sk.MustGetEntity(c.EntityA)
			bn := sk.MustGetEntity(c.EntityB)

			a := an.NormalExprsN(sk)
			b := bn.NormalExprsN(sk)

			p := sk.MustGetParam(c.H.Param(0))
			p.Val = core.EvalExpr(sk, a.Dot(b)) / core.EvalExpr(sk, b.Dot(b))

		case core.ConstraintParallel:
			if allParamsExistFor(c) {
				continue
			}

			ea := sk.MustGetEntity(c.EntityA)
			eb := sk.MustGetEntity(c.EntityB)
			a := ea.VectorGetExprsInWorkplane(sk, c.Workplane)
			b := eb.VectorGetExprsInWorkplane(sk, c.Workplane)

			p := sk.MustGetParam(c.H.Param(0))
			p.Val = core.EvalExpr(sk, a.Dot(b)) / core.EvalExpr(sk, b.Dot(b))
		}
	}
}

// evalVector evaluates v at the sketch's param values (C++
// ExprVector::Eval).
func evalVector(sk *Sketch, v core.ExprVector) geom.Vector {
	return geom.Vector{X: core.EvalExpr(sk, v.X), Y: core.EvalExpr(sk, v.Y), Z: core.EvalExpr(sk, v.Z)}
}

// ActiveGroup is the active group: Active if it exists, else the one
// C++ makes active after loading a file (GraphicsWindow::Init and
// EnsureValidActives): the last group, or the first that isn't the
// references.
func (sk *Sketch) ActiveGroup() core.HGroup {
	if sk.Active.V != 0 && sk.GetGroup(sk.Active) != nil {
		return sk.Active
	}
	if len(sk.GroupOrder) == 0 {
		return core.HGroup{}
	}
	active := sk.GroupOrder[len(sk.GroupOrder)-1]
	if active.V == core.HGroupReferences {
		for _, h := range sk.GroupOrder {
			if h.V != core.HGroupReferences {
				return h
			}
		}
	}
	return active
}

// groupIsVisible is C++ Group::IsVisible: shown, and not after the
// active group.
func (sk *Sketch) groupIsVisible(g *Group) bool {
	if !g.Visible {
		return false
	}
	active := sk.GetGroup(sk.ActiveGroup())
	return active == nil || g.Order <= active.Order
}

// EntityIsVisible is C++ Entity::IsVisible, with the show flags of
// sk.Show (GraphicsWindow::Init's when nil).
func (sk *Sketch) EntityIsVisible(e *core.Entity) bool {
	show := sk.visibility()
	g := sk.GetGroup(e.Group)

	if g.H.V == core.HGroupReferences && e.IsNormal() {
		// The reference normals are always shown
		return true
	}
	if !sk.groupIsVisible(g) {
		return false
	}

	if e.IsPoint() && !show.Points {
		return false
	}
	if e.IsNormal() && !show.Normals {
		return false
	}
	if e.Construction && !show.Construction {
		return false
	}

	if !show.Workplanes {
		if e.IsWorkplane() && !e.H.IsFromRequest() {
			if g.H != sk.ActiveGroup() {
				// The group-associated workplanes are hidden outside
				// their group.
				return false
			}
		}
	}

	if e.Style.V != 0 {
		if s := sk.StyleGet(e.Style); !s.Visible {
			return false
		}
	}

	return !e.ForceHidden
}

// ActiveWorkplane is the active group's workplane (C++
// GraphicsWindow::ActiveWorkplane).
func (sk *Sketch) ActiveWorkplane() core.HEntity {
	if g := sk.GetGroup(sk.ActiveGroup()); g != nil {
		return g.ActiveWorkplane
	}
	return core.FreeIn3D
}
