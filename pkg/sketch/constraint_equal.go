package sketch

import (
	"errors"
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
)

// ErrEqualSelection is wrapped by the error EqualConstraints returns for
// entities that no equal constraint takes.
var ErrEqualSelection = errors.New("not a selection that can be made equal")

// EqualConstraints makes the constraints that Constrain > Equal Length /
// Radius makes of the entities es, whatever order they come in, each a
// copy of base with its type and entities set. The window and the
// scripts both take their selection through here, so they read it the
// same way:
//
//   - two or more line segments: equal length, the first against each
//     of the others;
//   - two or more circles or arcs: equal radius, likewise;
//   - a line segment and an arc: the line segment's length is the arc's;
//   - two line segments and two points: the first point's distance from
//     the first line segment is the second point's from the second
//     (line segments and points each taken in the order they come in);
//   - one line segment and two points: the points are as far from it;
//   - two line segments and a point: the point's distance from the first
//     line segment is the second one's length.
//
// Anything else is an error wrapping ErrEqualSelection.
func (sk *Sketch) EqualConstraints(base core.Constraint, es []core.HEntity) ([]core.Constraint, error) {
	var lines, circles, points []core.HEntity
	arcs, other := 0, 0
	for _, he := range es {
		e := sk.Entity(he)
		switch {
		case e == nil:
			other++
		case e.Type == core.EntityLineSegment:
			lines = append(lines, he)
		case e.IsCircle():
			circles = append(circles, he)
			if e.Type == core.EntityArcOfCircle {
				arcs++
			}
		case e.IsPoint():
			points = append(points, he)
		default:
			other++
		}
	}
	n := len(es)
	var out []core.Constraint
	chain := func(t core.ConstraintType, hs []core.HEntity) {
		for _, h := range hs[1:] {
			c := base
			c.Type, c.EntityA, c.EntityB = t, hs[0], h
			out = append(out, c)
		}
	}
	c := base
	switch {
	case other > 0:
		return nil, fmt.Errorf("equal: %w", ErrEqualSelection)
	case len(lines) >= 2 && len(lines) == n:
		chain(core.ConstraintEqualLengthLines, lines)
	case len(circles) >= 2 && len(circles) == n:
		chain(core.ConstraintEqualRadius, circles)
	case len(lines) == 2 && len(points) == 2:
		c.Type = core.ConstraintEqPtLnDistances
		c.EntityA, c.PtA = lines[0], points[0]
		c.EntityB, c.PtB = lines[1], points[1]
		out = append(out, c)
	case len(lines) == 1 && len(points) == 2:
		// The same line segment for both distances.
		c.Type = core.ConstraintEqPtLnDistances
		c.EntityA, c.PtA = lines[0], points[0]
		c.EntityB, c.PtB = lines[0], points[1]
		out = append(out, c)
	case len(lines) == 2 && len(points) == 1:
		c.Type = core.ConstraintEqLenPtLineD
		c.EntityA, c.EntityB, c.PtA = lines[0], lines[1], points[0]
		out = append(out, c)
	case len(lines) == 1 && arcs == 1 && len(circles) == 1 && n == 2:
		c.Type = core.ConstraintEqualLineArcLen
		c.EntityA, c.EntityB = lines[0], circles[0]
		out = append(out, c)
	default:
		return nil, fmt.Errorf("equal: %w", ErrEqualSelection)
	}
	return out, nil
}
