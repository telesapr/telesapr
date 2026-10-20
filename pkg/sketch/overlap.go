package sketch

import (
	"errors"
	"fmt"
	"slices"

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

// Overlap lists what is drawn where point he is, in the order of the
// entities: what a click there could have picked. Entities of hidden
// groups, of groups after the active one, and those the show flags
// hide are left out, as EntityIsVisible says. Positions are compared
// in 3D, to within core.LengthEps.
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
	drawn := sk.drawnGroups()
	at := func(h core.HEntity) bool {
		q, err := rt.Point(h)
		return err == nil && q.EqualsTol(p, core.LengthEps)
	}

	// The geometry is tested before the visibility: the show flags
	// matter for few entities, and a position match is rare.
	var o Overlap
	for i := range sk.Entities {
		c := &sk.Entities[i]
		if c.H == he || !drawn[c.Group] {
			continue
		}
		switch {
		case c.IsPoint():
			if at(c.H) && sk.EntityIsVisible(c) {
				o.Points = append(o.Points, c.H)
			}
		case isOverlapCurve(c):
			if !curveHasPointAt(c, at) && !curveBoundExcludes(c, p, rt) &&
				sk.EntityIsVisible(c) && curvePasses(c, p, rt) {
				o.Curves = append(o.Curves, c.H)
			}
		}
	}
	return o, nil
}

// drawnGroups is the set of groups whose entities are drawn: the shown
// groups up to the active one.
func (sk *Sketch) drawnGroups() map[core.HGroup]bool {
	drawn := make(map[core.HGroup]bool, len(sk.Groups))
	for i := range sk.Groups {
		if g := &sk.Groups[i]; sk.groupIsVisible(g) {
			drawn[g.H] = true
		}
	}
	return drawn
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
	return slices.ContainsFunc(e.Point[:], func(h core.HEntity) bool {
		return h.V != 0 && at(h)
	})
}

// curveBoundExcludes reports whether p is certainly not on the curve of
// e, from a box the curve stays in that its points give without the
// curve itself: the ends of a line, the centre and radius of a circle
// or an arc. A cubic's points are interpolated, so its curve leaves
// their box; it has no such reject.
func curveBoundExcludes(e *core.Entity, p geom.Vector, rt *Runtime) bool {
	switch e.Type {
	case core.EntityLineSegment:
		a, errA := rt.Point(e.Point[0])
		b, errB := rt.Point(e.Point[1])
		if errA != nil || errB != nil {
			return true
		}
		bmax, bmin := a, a
		b.MakeMaxMin(&bmax, &bmin)
		return p.OutsideAndNotOn(bmax, bmin)
	case core.EntityCircle, core.EntityArcOfCircle:
		c, err := rt.Point(e.Point[0])
		if err != nil {
			return true
		}
		var r float64
		if e.Type == core.EntityCircle {
			r, err = rt.Radius(e.Distance)
		} else {
			var a geom.Vector
			a, err = rt.Point(e.Point[1])
			r = a.Minus(c).Magnitude()
		}
		if err != nil {
			return true
		}
		d := geom.NewVector(r, r, r)
		return p.OutsideAndNotOn(c.Plus(d), c.Minus(d))
	}
	return false
}

// curvePasses reports whether the curve of e goes through p: the
// closest point of one of its Bézier pieces is within core.LengthEps
// of p. A piece stays in the box of its control points, so one whose
// box leaves p out is not searched.
func curvePasses(e *core.Entity, p geom.Vector, rt *Runtime) bool {
	var sbl srf.SBezierList
	GenerateEntityBeziers(e, &sbl, rt)
	for i := range sbl.Curves {
		b := &sbl.Curves[i]
		bmax, bmin := b.Ctrl[0], b.Ctrl[0]
		for _, c := range b.Ctrl[1 : b.Degree+1] {
			c.MakeMaxMin(&bmax, &bmin)
		}
		if p.OutsideAndNotOn(bmax, bmin) {
			continue
		}
		t, _ := b.ClosestPointTo(p, true)
		if b.PointAt(min(max(t, 0), 1)).EqualsTol(p, core.LengthEps) {
			return true
		}
	}
	return false
}
