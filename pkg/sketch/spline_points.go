package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/geom"
)

// A cubic spline's points are its request's: an open spline has its two
// ends, the control points beside them and ExtraPoints points it passes
// through between; a periodic one has ExtraPoints+3 points it passes
// through. A point is added or taken away by moving the points after it
// along by one, with the constraints on them, and changing ExtraPoints;
// the request's handle and its entities' handles stay as they were.

// ErrSplinePointsFull is returned when a spline has as many points as an
// entity holds.
var ErrSplinePointsFull = errors.New("the spline has as many points as it can hold")

// splineRequest is the cubic request that drew entity he.
func (sk *Sketch) splineRequest(he core.HEntity) (*Request, error) {
	if !he.IsFromRequest() {
		return nil, fmt.Errorf("e%03x was not drawn as a cubic spline", he.V)
	}
	r := sk.Request(he.Request())
	if r == nil || (r.Type != RequestCubic && r.Type != RequestCubicPeriodic) {
		return nil, fmt.Errorf("e%03x is not part of a cubic spline", he.V)
	}
	return r, nil
}

// splinePointCount is the number of points of spline request r.
func splinePointCount(r *Request) int {
	if r.Type == RequestCubicPeriodic {
		return r.ExtraPoints + 3
	}
	return r.ExtraPoints + 4
}

// RemoveSplinePoint takes point hp out of the cubic spline it belongs
// to. An open spline keeps its ends and their control points, and every
// spline at least the points it is drawn with. The points the removed
// one was made coincident with stay coincident with each other; other
// constraints on it are deleted, since they would otherwise fall to the
// next point and bend the curve.
func (sk *Sketch) RemoveSplinePoint(hp core.HEntity) error {
	r, err := sk.splineRequest(hp)
	if err != nil {
		return err
	}
	index := r.IndexOfPoint(hp)
	interior := r.Type == RequestCubicPeriodic || (index > 1 && index < r.ExtraPoints+2)
	if index < 0 || !interior {
		return fmt.Errorf("point %d is an end or an end's control point of the spline, which it can't lose", index)
	}
	if r.ExtraPoints == 0 {
		return errors.New("the spline has no more points than it is drawn with")
	}
	e := sk.Entity(r.H.Entity(0))
	sk.FixConstraintsForPointBeingDeleted(e.Point[index])
	sk.RemoveConstraintsForPointBeingDeleted(e.Point[index])
	for i := index; i < core.MaxPointsInEntity-1; i++ {
		if e.Point[i+1].V == 0 {
			break
		}
		p0 := sk.Entity(e.Point[i])
		p1 := sk.Entity(e.Point[i+1])
		sk.ReplacePointInConstraints(p1.H, p0.H)
		p0.PointForceTo(sk, p1.PointGetNum(sk))
	}
	sk.Request(r.H).ExtraPoints--
	sk.MarkGroupDirtyByEntity(hp)
	return nil
}

// AddSplinePoint adds a point at at to the cubic spline that entity he
// belongs to, after its point number after, which for an open spline is
// one of the points between its ends' control points or the first of
// those. The spline is regenerated, so that the new point exists, before
// the points after it are moved along.
func (sk *Sketch) AddSplinePoint(he core.HEntity, after int, at geom.Vector) error {
	r, err := sk.splineRequest(he)
	if err != nil {
		return err
	}
	n := splinePointCount(r)
	if n >= core.MaxPointsInEntity {
		return ErrSplinePointsFull
	}
	first, last := 0, n-1
	if r.Type == RequestCubic {
		first, last = 1, n-3
	}
	if after < first || after > last {
		return fmt.Errorf("a point goes after point %d to %d of the spline, not %d", first, last, after)
	}
	hr := r.H
	r.ExtraPoints++
	sk.MarkGroupDirtyByEntity(he)
	sk.GenerateAllTyped(GenerateRegen)

	e := sk.Entity(hr.Entity(0))
	for i := core.MaxPointsInEntity - 1; i > after+1; i-- {
		p0 := sk.Entity(e.Point[i])
		if e.Point[i].V == 0 || p0 == nil {
			continue
		}
		p1 := sk.Entity(e.Point[i-1])
		sk.ReplacePointInConstraints(p1.H, p0.H)
		p0.PointForceTo(sk, p1.PointGetNum(sk))
	}
	sk.Entity(e.Point[after+1]).PointForceTo(sk, at)
	sk.MarkGroupDirtyByEntity(he)
	return nil
}

// NearestSplineSpan is the point of the spline of entity he that a
// point added nearest at should follow: the start of the span between
// two neighbouring points, of those AddSplinePoint takes, that passes
// nearest at.
func (sk *Sketch) NearestSplineSpan(he core.HEntity, at geom.Vector) (int, error) {
	r, err := sk.splineRequest(he)
	if err != nil {
		return 0, err
	}
	n := splinePointCount(r)
	first, last := 0, n-1
	if r.Type == RequestCubic {
		first, last = 1, n-3
	}
	e := sk.Entity(r.H.Entity(0))
	best, bestDist := first, -1.0
	for i := first; i <= last; i++ {
		a := sk.Entity(e.Point[i]).PointGetNum(sk)
		b := sk.Entity(e.Point[(i+1)%n]).PointGetNum(sk)
		if d := distanceToSpan(at, a, b); bestDist < 0 || d < bestDist {
			best, bestDist = i, d
		}
	}
	return best, nil
}

// distanceToSpan is the distance from p to the segment from a to b.
func distanceToSpan(p, a, b geom.Vector) float64 {
	d := b.Minus(a)
	l := d.Dot(d)
	if l == 0 {
		return p.DistanceTo(a)
	}
	t := max(0, min(1, p.Minus(a).Dot(d)/l))
	return p.DistanceTo(a.Plus(d.ScaledBy(t)))
}

// PointIsCoincident reports whether a constraint makes point hp
// coincident with another.
func (sk *Sketch) PointIsCoincident(hp core.HEntity) bool {
	for i := range sk.Constraints {
		c := &sk.Constraints[i]
		if c.Type == core.ConstraintPointsCoincident && (c.PtA == hp || c.PtB == hp) {
			return true
		}
	}
	return false
}

// DeletePointCoincidences deletes the constraints that make point hp
// coincident with others, and reports an error when there are none.
func (sk *Sketch) DeletePointCoincidences(hp core.HEntity) error {
	p := sk.Entity(hp)
	if p == nil || !p.IsPoint() {
		return fmt.Errorf("e%03x is not a point", hp.V)
	}
	kept := sk.Constraints[:0]
	n := 0
	for _, c := range sk.Constraints {
		if c.Type == core.ConstraintPointsCoincident && (c.PtA == hp || c.PtB == hp) {
			n++
			continue
		}
		kept = append(kept, c)
	}
	sk.Constraints = kept
	if n == 0 {
		return errors.New("the point is coincident with no other")
	}
	sk.MarkGroupDirtyByEntity(hp)
	return nil
}
