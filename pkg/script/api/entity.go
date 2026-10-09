package api

import (
	"fmt"
	"math"
	"slices"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/sketch"
)

// Entity is something on the drawing: a point, a line segment, a
// circle, an arc, a curve, a workplane, a normal, a face. What is
// drawn makes several: a line segment is an entity and so are its two
// ends.
type Entity struct {
	d *Doc
	H core.HEntity
}

// get is the entity itself, which mustn't be kept.
func (e Entity) get(sk *sketch.Sketch) *core.Entity {
	if e.d == nil || e.H.V == 0 {
		fail("no entity")
	}
	ent := sk.Entity(e.H)
	if ent == nil {
		fail("entity e%08x doesn't exist any more", e.H.V)
	}
	return ent
}

// kindOf names an entity's type.
func kindOf(ent *core.Entity) string {
	switch {
	case ent.IsPoint():
		return "point"
	case ent.IsNormal():
		return "normal"
	case ent.IsDistance():
		return "distance"
	case ent.IsFace():
		return "face"
	}
	switch ent.Type {
	case core.EntityWorkplane:
		return "workplane"
	case core.EntityLineSegment:
		return "line"
	case core.EntityCubic:
		return "cubic"
	case core.EntityCubicPeriodic:
		return "periodic-cubic"
	case core.EntityCircle:
		return "circle"
	case core.EntityArcOfCircle:
		return "arc"
	case core.EntityTTFText:
		return "text"
	case core.EntityImage:
		return "image"
	}
	return "entity"
}

// ownerOf describes the request or the group an entity is from.
func ownerOf(sk *sketch.Sketch, h core.HEntity) string {
	if h.IsFromRequest() {
		if r := sk.Request(h.Request()); r != nil {
			return r.DescriptionString()
		}
		return fmt.Sprintf("r%03x", h.Request().V)
	}
	return groupDescription(sk.Group(h.Group()))
}

func (e Entity) describe(sk *sketch.Sketch) string {
	if e.H.V == 0 {
		return "no entity"
	}
	ent := sk.Entity(e.H)
	if ent == nil {
		return fmt.Sprintf("deleted entity e%08x", e.H.V)
	}
	kind := kindOf(ent)
	owner := ownerOf(sk, e.H)
	switch {
	case ent.IsPoint():
		p := ent.PointGetNum(sk)
		return fmt.Sprintf("point (%.3f, %.3f, %.3f) of %s", p.X, p.Y, p.Z, owner)
	case ent.IsCircle():
		return fmt.Sprintf("%s %s, radius %.3f", kind, owner, ent.CircleGetRadiusNum(sk))
	case e.H.IsFromRequest() && e.H == e.H.Request().Entity(0):
		return kind + " " + owner
	}
	return kind + " of " + owner
}

// String describes the entity, as "line r004-line-segment".
func (e Entity) String() (s string) {
	if e.d == nil {
		return "no entity"
	}
	e.d.read(func(sk *sketch.Sketch) { s = e.describe(sk) })
	return s
}

// Valid reports whether the entity exists.
func (e Entity) Valid() (ok bool) {
	if e.d == nil || e.H.V == 0 {
		return false
	}
	e.d.read(func(sk *sketch.Sketch) { ok = sk.Entity(e.H) != nil })
	return ok
}

// Kind names what the entity is: "point", "line", "circle", "arc",
// "cubic", "workplane", "normal", "face", ...
func (e Entity) Kind() (kind string) {
	e.d.read(func(sk *sketch.Sketch) { kind = kindOf(e.get(sk)) })
	return kind
}

// Type is the entity's type, as the core package names them.
func (e Entity) Type() (t core.EntityType) {
	e.d.read(func(sk *sketch.Sketch) { t = e.get(sk).Type })
	return t
}

// Group is the group the entity is in.
func (e Entity) Group() (g Group) {
	e.d.read(func(sk *sketch.Sketch) { g = Group{e.d, e.get(sk).Group} })
	return g
}

// Request is what was drawn to make the entity; it isn't Valid for an
// entity a group made, such as the copies an extrusion makes.
func (e Entity) Request() Request {
	if e.H.IsFromRequest() {
		return Request{e.d, e.H.Request()}
	}
	return Request{d: e.d}
}

// Pos is where a point is.
func (e Entity) Pos() (p Vec) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if !ent.IsPoint() {
			fail("%s isn't a point; see Start, End, Center and Points", e.describe(sk))
		}
		p = ent.PointGetNum(sk)
	})
	return p
}

// MoveTo puts a point at p, as dragging it there does: the solver
// then moves it as the constraints require.
func (e Entity) MoveTo(p Vec) {
	e.d.change(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if !ent.IsPoint() {
			fail("%s isn't a point", e.describe(sk))
		}
		ent.PointForceTo(sk, p)
		sk.MarkGroupDirtyByEntity(e.H)
	})
}

// Points are the points of the entity: the ends of a line, the centre
// and the ends of an arc, the centre of a circle, the control points
// of a cubic, the origin of a workplane. A point has itself.
func (e Entity) Points() (points []Entity) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if ent.IsPoint() {
			points = []Entity{e}
			return
		}
		for _, hp := range ent.Point {
			if hp.V != 0 {
				points = append(points, Entity{e.d, hp})
			}
		}
	})
	return points
}

// point is the i-th point of an entity of one of the kinds.
func (e Entity) point(what string, i int, kinds ...core.EntityType) (p Entity) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if slices.Contains(kinds, ent.Type) {
			p = Entity{e.d, ent.Point[i]}
			return
		}
		fail("%s has no %s", e.describe(sk), what)
	})
	return p
}

// Start is the first end of a line segment, an arc or a cubic.
func (e Entity) Start() Entity {
	i := 0
	if e.Type() == core.EntityArcOfCircle {
		i = 1
	}
	return e.point("start", i, core.EntityLineSegment, core.EntityArcOfCircle, core.EntityCubic)
}

// End is the other end of a line segment, an arc or a cubic.
func (e Entity) End() (p Entity) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		switch ent.Type {
		case core.EntityLineSegment:
			p = Entity{e.d, ent.Point[1]}
		case core.EntityArcOfCircle:
			p = Entity{e.d, ent.Point[2]}
		case core.EntityCubic:
			p = Entity{e.d, ent.Point[3+ent.ExtraPoints]}
		default:
			fail("%s has no end", e.describe(sk))
		}
	})
	return p
}

// Center is the centre of a circle or an arc, or the origin of a
// workplane.
func (e Entity) Center() Entity {
	return e.point("centre", 0, core.EntityCircle, core.EntityArcOfCircle, core.EntityWorkplane)
}

// Normal is the normal of a circle, an arc or a workplane.
func (e Entity) Normal() (n Entity) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if ent.Normal.V == 0 {
			fail("%s has no normal", e.describe(sk))
		}
		n = Entity{e.d, ent.Normal}
	})
	return n
}

// Radius is the radius of a circle or an arc.
func (e Entity) Radius() (r float64) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if !ent.IsCircle() {
			fail("%s isn't a circle or an arc", e.describe(sk))
		}
		r = ent.CircleGetRadiusNum(sk)
	})
	return r
}

// SetRadius gives a circle a radius, as dragging it does; constrain
// it with Diameter or Radius to keep it.
func (e Entity) SetRadius(r float64) {
	e.d.change(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if ent.Type != core.EntityCircle {
			fail("%s isn't a circle", e.describe(sk))
		}
		sk.MustEntity(ent.Distance).DistanceForceTo(sk, r)
		sk.MarkGroupDirtyByEntity(e.H)
	})
}

// Length is the length of a line segment, an arc or a circle.
func (e Entity) Length() (l float64) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		switch ent.Type {
		case core.EntityLineSegment:
			l = ent.EndpointStart(sk).Minus(ent.EndpointFinish(sk)).Magnitude()
		case core.EntityCircle:
			l = 2 * math.Pi * ent.CircleGetRadiusNum(sk)
		case core.EntityArcOfCircle:
			_, _, dtheta := ent.ArcGetAngles(sk)
			l = ent.CircleGetRadiusNum(sk) * dtheta
		default:
			fail("%s has no length", e.describe(sk))
		}
	})
	return l
}

// Dir is the direction of a line segment, from its start to its end,
// or of a normal; it isn't a unit vector for a line.
func (e Entity) Dir() (v Vec) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if !ent.HasVector() {
			fail("%s has no direction", e.describe(sk))
		}
		v = ent.VectorGetNum(sk)
		if ent.Type == core.EntityLineSegment {
			// A line's vector runs from its second point to its first.
			v = v.Negated()
		}
	})
	return v
}

// FaceNormal is a face's normal, a unit vector.
func (e Entity) FaceNormal() (n Vec) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if !ent.IsFace() {
			fail("%s isn't a face", e.describe(sk))
		}
		n = ent.FaceGetNormalNum(sk).WithMagnitude(1)
	})
	return n
}

// FacePoint is a point a face's plane passes through.
func (e Entity) FacePoint() (p Vec) {
	e.d.read(func(sk *sketch.Sketch) {
		ent := e.get(sk)
		if !ent.IsFace() {
			fail("%s isn't a face", e.describe(sk))
		}
		p = ent.FaceGetPointNum(sk)
	})
	return p
}

// Construction reports whether the entity is construction geometry,
// which helps to draw but makes no part of the outline.
func (e Entity) Construction() (construction bool) {
	e.d.read(func(sk *sketch.Sketch) { construction = e.get(sk).Construction })
	return construction
}

// SetConstruction makes what was drawn construction geometry, or not.
func (e Entity) SetConstruction(construction bool) {
	e.Request().SetConstruction(construction)
}

// Delete deletes what was drawn to make the entity, and the
// constraints on it.
func (e Entity) Delete() {
	e.d.change(func(sk *sketch.Sketch) {
		e.get(sk)
		if !e.H.IsFromRequest() || e.H.Request().IsFromReferences() {
			fail("%s wasn't drawn, so it can't be deleted; delete its group", e.describe(sk))
		}
		sk.DeleteSelection([]sketch.Selection{{Entity: e.H}})
	})
}

// AddSplinePoint adds a point at p to the cubic spline the entity is,
// in the span of the spline's points that passes nearest p, as the
// context menu's Add Spline Point does where the curve is clicked. The
// spline then passes through p, or for a periodic one through it too.
func (e Entity) AddSplinePoint(p Vec) {
	e.d.change(func(sk *sketch.Sketch) {
		e.get(sk)
		after, err := sk.NearestSplineSpan(e.H, p)
		if err == nil {
			err = sk.AddSplinePoint(e.H, after, p)
		}
		if err != nil {
			fail("%s: %v", e.describe(sk), err)
		}
	})
}

// RemoveSplinePoint takes the point out of the cubic spline it belongs
// to, as the context menu's Remove Spline Point does. The points it was
// coincident with stay coincident with each other; its other
// constraints are deleted. An open spline's ends and their control
// points can't be removed.
func (e Entity) RemoveSplinePoint() {
	e.d.change(func(sk *sketch.Sketch) {
		e.get(sk)
		if err := sk.RemoveSplinePoint(e.H); err != nil {
			fail("%s: %v", e.describe(sk), err)
		}
	})
}

// DeleteCoincidences deletes the constraints that make the point
// coincident with others, as the context menu's Delete Point-Coincident
// Constraint does, so it can be dragged away from them.
func (e Entity) DeleteCoincidences() {
	e.d.change(func(sk *sketch.Sketch) {
		e.get(sk)
		if err := sk.DeletePointCoincidences(e.H); err != nil {
			fail("%s: %v", e.describe(sk), err)
		}
	})
}

// Request is something that was drawn: a line segment, a circle, a
// datum point. The property browser lists the requests of a group.
type Request struct {
	d *Doc
	H core.HRequest
}

func (r Request) get(sk *sketch.Sketch) *sketch.Request {
	if r.d == nil || r.H.V == 0 {
		fail("no request")
	}
	req := sk.Request(r.H)
	if req == nil {
		fail("request r%03x doesn't exist any more", r.H.V)
	}
	return req
}

// String describes the request as the property browser does:
// "r004-line-segment".
func (r Request) String() (s string) {
	if r.d == nil || r.H.V == 0 {
		return "no request"
	}
	r.d.read(func(sk *sketch.Sketch) {
		if req := sk.Request(r.H); req != nil {
			s = req.DescriptionString()
		} else {
			s = fmt.Sprintf("deleted request r%03x", r.H.V)
		}
	})
	return s
}

// Valid reports whether the request exists.
func (r Request) Valid() (ok bool) {
	if r.d == nil || r.H.V == 0 {
		return false
	}
	r.d.read(func(sk *sketch.Sketch) { ok = sk.Request(r.H) != nil })
	return ok
}

// Type is the request's type, as the sketch package names them.
func (r Request) Type() (t sketch.RequestType) {
	r.d.read(func(sk *sketch.Sketch) { t = r.get(sk).Type })
	return t
}

// Group is the group the request is in.
func (r Request) Group() (g Group) {
	r.d.read(func(sk *sketch.Sketch) { g = Group{r.d, r.get(sk).Group} })
	return g
}

// Entity is the entity the request makes: the line, the circle. Its
// points are that entity's Points.
func (r Request) Entity() Entity {
	return Entity{r.d, r.H.Entity(0)}
}

// Construction reports whether the request is construction geometry.
func (r Request) Construction() (construction bool) {
	r.d.read(func(sk *sketch.Sketch) { construction = r.get(sk).Construction })
	return construction
}

// SetConstruction makes the request construction geometry, or not.
func (r Request) SetConstruction(construction bool) {
	r.d.change(func(sk *sketch.Sketch) {
		req := r.get(sk)
		req.Construction = construction
		sk.MarkGroupDirty(req.Group, false)
	})
}

// Delete deletes the request and the constraints on it.
func (r Request) Delete() { r.Entity().Delete() }

// Param is one of the numbers the solver works out: a coordinate of a
// point, a radius, the depth of an extrusion.
type Param struct {
	d *Doc
	H core.HParam
}

func (p Param) get(sk *sketch.Sketch) *core.Param {
	if p.d == nil || p.H.V == 0 {
		fail("no parameter")
	}
	param := sk.Param(p.H)
	if param == nil {
		fail("parameter p%08x doesn't exist any more", p.H.V)
	}
	return param
}

// Valid reports whether the parameter exists.
func (p Param) Valid() (ok bool) {
	if p.d == nil || p.H.V == 0 {
		return false
	}
	p.d.read(func(sk *sketch.Sketch) { ok = sk.Param(p.H) != nil })
	return ok
}

// Val is the parameter's value.
func (p Param) Val() (v float64) {
	p.d.read(func(sk *sketch.Sketch) { v = p.get(sk).Val })
	return v
}

// Set gives the parameter a value for the solver to start from; what
// constrains it decides where it ends.
func (p Param) Set(v float64) {
	p.d.change(func(sk *sketch.Sketch) {
		p.get(sk).Val = v
		markAllDirty(sk)
	})
}

// String shows the parameter's value.
func (p Param) String() string {
	if !p.Valid() {
		return "no parameter"
	}
	return fmt.Sprintf("p%08x = %g", p.H.V, p.Val())
}

// Free is how much of the entity its group's constraints fix, as the
// drawing colours it: sketch.FreedomFixed when it is fully constrained,
// sketch.FreedomPartly when some of it can still move,
// sketch.FreedomFree when nothing holds it, and sketch.FreedomUnknown
// when its group did not solve. Parameters from earlier groups, such
// as a workplane's, count as fixed.
func (e Entity) Free() (f sketch.Freedom) {
	e.d.read(func(sk *sketch.Sketch) { f = sk.EntityFreedom(e.get(sk).H) })
	return f
}
