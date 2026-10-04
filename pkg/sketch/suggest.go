package sketch

import (
	"math"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// Suggestion is a constraint an entity being drawn would get when it is
// placed (roadmap 16.11 for lines, 29.4 for arcs and circles): its type,
// its entities, and which ends of curves it takes.
type Suggestion struct {
	Type          core.ConstraintType
	A, B          core.HEntity
	Other, Other2 bool
}

// suggestTolerance is how near a drawn entity must be to what is
// suggested: a share of a right angle's cosine, or of a length.
const suggestTolerance = 0.02

// SuggestConstraint suggests a constraint for the entity of request as
// it is drawn in workplane wp and group, when it nearly meets one; none
// in 3d, where nothing is drawn locked to a plane, but for circles.
//
// A line: horizontal or vertical first, as SolveSpace does; else tangent
// to an arc it starts on, perpendicular to a line it starts on, parallel
// to a line, or as long as one, in that order.
//
// An arc: tangent at either end to a line or an arc that ends there,
// else the same radius as an arc or a circle of the group.
//
// A circle: the same radius as an arc or a circle of the group.
//
// Only entities of the group's requests are looked at.
func (sk *Sketch) SuggestConstraint(request core.HRequest, wp core.HEntity, group core.HGroup) (Suggestion, bool) {
	r := sk.GetRequest(request)
	if r == nil {
		return Suggestion{}, false
	}
	switch r.Type {
	case RequestLineSegment:
		if wp.V == 0 || core.IsFreeIn3D(wp) {
			return Suggestion{}, false
		}
		return sk.suggestForLine(request, wp, group)
	case RequestArcOfCircle:
		if wp.V == 0 || core.IsFreeIn3D(wp) {
			return Suggestion{}, false
		}
		if s, ok := sk.suggestArcTangent(request, group); ok {
			return s, true
		}
		return sk.suggestEqualRadius(request, group)
	case RequestCircle:
		return sk.suggestEqualRadius(request, group)
	}
	return Suggestion{}, false
}

// ApplySuggestion adds the suggested constraint to the active group,
// unless it would be redundant; it returns the constraint, or none.
func (sk *Sketch) ApplySuggestion(s Suggestion) core.HConstraint {
	return sk.TryConstrain(s.Type, core.HEntity{}, core.HEntity{}, s.A, s.B, s.Other, s.Other2)
}

// others calls f with each entity of group's requests but request's.
func (sk *Sketch) others(request core.HRequest, group core.HGroup, f func(e *core.Entity)) {
	for i := range sk.Entities {
		e := &sk.Entities[i]
		if e.Group != group || e.H.Request() == request || !e.H.IsFromRequest() {
			continue
		}
		f(e)
	}
}

func (sk *Sketch) pointNum(h core.HEntity) geom.Vector {
	return sk.MustGetEntity(h).PointGetNum(sk)
}

func (sk *Sketch) suggestForLine(request core.HRequest, wp core.HEntity, group core.HGroup) (Suggestion, bool) {
	ptA := sk.MustGetEntity(request.Entity(1))
	ptB := sk.MustGetEntity(request.Entity(2))

	au, av := ptA.PointGetExprsInWorkplaneUV(sk, wp)
	bu, bv := ptB.PointGetExprsInWorkplaneUV(sk, wp)

	rt := NewRuntime(sk)
	du := au.Minus(bu).Eval(rt, nil)
	dv := av.Minus(bv).Eval(rt, nil)
	line := request.Entity(0)

	if math.Abs(dv) > geom.LengthEps && math.Abs(du/dv) < suggestTolerance {
		return Suggestion{Type: core.ConstraintVertical, A: line}, true
	} else if math.Abs(du) > geom.LengthEps && math.Abs(dv/du) < suggestTolerance {
		return Suggestion{Type: core.ConstraintHorizontal, A: line}, true
	}

	a, b := ptA.PointGetNum(sk), ptB.PointGetNum(sk)
	length := b.Minus(a).Magnitude()
	if length < geom.LengthEps {
		return Suggestion{}, false
	}
	dir := b.Minus(a).ScaledBy(1 / length)
	var tangent, perpendicular, parallel, equal Suggestion
	nearest := math.Inf(1)
	sk.others(request, group, func(e *core.Entity) {
		switch e.Type {
		case core.EntityArcOfCircle:
			if tangent.Type != 0 {
				return
			}
			c := sk.pointNum(e.Point[0])
			for k, end := range []core.HEntity{e.Point[1], e.Point[2]} {
				p := sk.pointNum(end)
				if !p.Equals(a) {
					continue
				}
				if r := p.Minus(c); r.Magnitude() > geom.LengthEps && math.Abs(r.WithMagnitude(1).Dot(dir)) < suggestTolerance {
					tangent = Suggestion{Type: core.ConstraintArcLineTangent, A: e.H, B: line, Other: k == 1}
				}
			}
		case core.EntityLineSegment:
			p0, p1 := sk.pointNum(e.Point[0]), sk.pointNum(e.Point[1])
			m := p1.Minus(p0).Magnitude()
			if m < geom.LengthEps {
				return
			}
			o := p1.Minus(p0).ScaledBy(1 / m)
			cos := math.Abs(o.Dot(dir))
			starts := p0.Equals(a) || p1.Equals(a)
			switch {
			case starts && cos < suggestTolerance && perpendicular.Type == 0:
				perpendicular = Suggestion{Type: core.ConstraintPerpendicular, A: line, B: e.H}
			case !starts && 1-cos < suggestTolerance*suggestTolerance/2:
				// The nearest of the parallel lines.
				if d := p0.Plus(p1).ScaledBy(0.5).Minus(a.Plus(b).ScaledBy(0.5)).Magnitude(); d < nearest {
					nearest = d
					parallel = Suggestion{Type: core.ConstraintParallel, A: line, B: e.H}
				}
			case math.Abs(m-length) < suggestTolerance/2*length && equal.Type == 0:
				equal = Suggestion{Type: core.ConstraintEqualLengthLines, A: line, B: e.H}
			}
		}
	})
	for _, s := range []Suggestion{tangent, perpendicular, parallel, equal} {
		if s.Type != 0 {
			return s, true
		}
	}
	return Suggestion{}, false
}

// suggestArcTangent is a tangent at an end of the arc of request to a
// line or another arc that ends where it does, when the two nearly are.
func (sk *Sketch) suggestArcTangent(request core.HRequest, group core.HGroup) (Suggestion, bool) {
	arc := sk.MustGetEntity(request.Entity(0))
	c := sk.pointNum(arc.Point[0])
	var found Suggestion
	for k, end := range []core.HEntity{arc.Point[1], arc.Point[2]} {
		p := sk.pointNum(end)
		radius := p.Minus(c)
		if radius.Magnitude() < geom.LengthEps {
			continue
		}
		radius = radius.WithMagnitude(1)
		sk.others(request, group, func(e *core.Entity) {
			if found.Type != 0 {
				return
			}
			switch e.Type {
			case core.EntityLineSegment:
				p0, p1 := sk.pointNum(e.Point[0]), sk.pointNum(e.Point[1])
				if !p0.Equals(p) && !p1.Equals(p) {
					return
				}
				d := p1.Minus(p0)
				if d.Magnitude() > geom.LengthEps && math.Abs(d.WithMagnitude(1).Dot(radius)) < suggestTolerance {
					found = Suggestion{Type: core.ConstraintArcLineTangent, A: arc.H, B: e.H, Other: k == 1}
				}
			case core.EntityArcOfCircle:
				oc := sk.pointNum(e.Point[0])
				for j, oend := range []core.HEntity{e.Point[1], e.Point[2]} {
					q := sk.pointNum(oend)
					if !q.Equals(p) {
						continue
					}
					or := q.Minus(oc)
					// Tangent where the radii are parallel.
					if or.Magnitude() > geom.LengthEps && 1-math.Abs(or.WithMagnitude(1).Dot(radius)) < suggestTolerance*suggestTolerance/2 {
						found = Suggestion{Type: core.ConstraintCurveCurveTangent, A: arc.H, B: e.H, Other: k == 1, Other2: j == 1}
						return
					}
				}
			}
		})
		if found.Type != 0 {
			return found, true
		}
	}
	return Suggestion{}, false
}

// suggestEqualRadius is the same radius as the nearest in size of the
// group's arcs and circles, when the circle or arc of request nearly has
// it.
func (sk *Sketch) suggestEqualRadius(request core.HRequest, group core.HGroup) (Suggestion, bool) {
	e := sk.MustGetEntity(request.Entity(0))
	r := e.CircleGetRadiusNum(sk)
	if r < geom.LengthEps {
		return Suggestion{}, false
	}
	var found Suggestion
	best := math.Inf(1)
	sk.others(request, group, func(o *core.Entity) {
		if o.Type != core.EntityCircle && o.Type != core.EntityArcOfCircle {
			return
		}
		if d := math.Abs(o.CircleGetRadiusNum(sk) - r); d < suggestTolerance/2*r && d < best {
			best = d
			found = Suggestion{Type: core.ConstraintEqualRadius, A: e.H, B: o.H}
		}
	})
	return found, found.Type != 0
}
