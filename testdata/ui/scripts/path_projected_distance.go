// The projected distance of path_projected_distance.script, made by a
// script (TestPathEquivalenceProjectedDistance).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	a := doc.Sketch.Point(V(-20, 0, 0))
	b := doc.Sketch.Point(V(-4, 2, 0))
	along := doc.Sketch.Line(V(-20, 20, 0), V(16, 20, 0))
	// The dimension is made at what it measures, as the interface makes
	// it: how far the one point is from the other along the line.
	across := b.Pos().Minus(a.Pos())
	direction := along.End().Pos().Minus(along.Start().Pos())
	direction = direction.ScaledBy(1 / direction.Magnitude())
	doc.Constrain.ProjectedDistance(a, b, along, across.Dot(direction))
}
