package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
	"github.com/telesapr/telesapr/pkg/srf"
)

// ErrNotPoint is returned by Overlap for an entity that isn't a point.
var ErrNotPoint = errors.New("not a point")

// Overlap is what lies where a point is, so that one of them can be
// chosen where a click on the drawing picks only one.
type Overlap struct {
	// Points are the other points at the same position.
	Points []core.HEntity
	// Curves are the line segments, circles, arcs and cubics that pass
	// through the position without a point of their own there; a curve
	// that ends there is reached through that end point instead.
	Curves []core.HEntity
}

// Overlap lists what lies where point he is, in the groups up to the
// active one, in the order of the entities. Positions are compared in
// 3D, to within core.LengthEps.
func (sk *Sketch) Overlap(he core.HEntity) (Overlap, error) {
	rt := &Runtime{Sketch: sk}
	e := sk.Entity(he)
	if e == nil {
		return Overlap{}, fmt.Errorf("e%03x doesn't exist", he.V)
	}
	if !e.IsPoint() {
		return Overlap{}, fmt.Errorf("e%03x: %w", he.V, ErrNotPoint)
	}
	p, err := rt.Point(he)
	if err != nil {
		return Overlap{}, fmt.Errorf("e%03x: %w", he.V, err)
	}
	active := sk.Group(sk.ActiveGroup())
	at := func(h core.HEntity) bool {
		q, err := rt.Point(h)
		return err == nil && q.EqualsTol(p, core.LengthEps)
	}

	var o Overlap
	for i := range sk.Entities {
		c := &sk.Entities[i]
		if c.H == he {
			continue
		}
		if g := sk.Group(c.Group); g == nil || (active != nil && g.Order > active.Order) {
			continue
		}
		switch {
		case c.IsPoint():
			if at(c.H) {
				o.Points = append(o.Points, c.H)
			}
		case isOverlapCurve(c):
			if !curveHasPointAt(c, at) && curvePasses(c, p, rt) {
				o.Curves = append(o.Curves, c.H)
			}
		}
	}
	return o, nil
}

func isOverlapCurve(e *core.Entity) bool {
	switch e.Type {
	case core.EntityLineSegment, core.EntityCircle, core.EntityArcOfCircle,
		core.EntityCubic, core.EntityCubicPeriodic:
		return true
	}
	return false
}

// curveHasPointAt reports whether one of the curve's own points (an end,
// a centre, a control point) is where at says.
func curveHasPointAt(e *core.Entity, at func(core.HEntity) bool) bool {
	for _, h := range e.Point {
		if h.V != 0 && at(h) {
			return true
		}
	}
	return false
}

// curvePasses reports whether the curve of e goes through p.
func curvePasses(e *core.Entity, p geom.Vector, rt *Runtime) bool {
	var sbl srf.SBezierList
	GenerateEntityBeziers(e, &sbl, rt)
	for i := range sbl.Curves {
		b := &sbl.Curves[i]
		t, _ := b.ClosestPointTo(p, true)
		if b.PointAt(min(max(t, 0), 1)).EqualsTol(p, core.LengthEps) {
			return true
		}
	}
	return false
}
