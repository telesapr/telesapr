// The bounds of path_distance_bounds.script, made by a script
// (TestPathEquivalence/distance_bounds): a line with the mouse's ends
// given a minimum length and a maximum, each made at the line's length
// and then edited as the interface does, and a point put on the line
// as a segment (F2).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	l := doc.Sketch.Line(V(-20, 10, 0), V(10, 10, 0))
	lo := doc.Constrain.MinDistance(l, Entity{}, 30)
	doc.Edit.Regen()
	lo.Set(40)
	doc.Edit.Regen()
	hi := doc.Constrain.MaxDistance(l, Entity{}, 40)
	doc.Edit.Regen()
	hi.Set(50)
	doc.Edit.Regen()
	p := doc.Sketch.Point(V(-8, -6, 0))
	doc.Edit.Regen()
	doc.Constrain.OnSegment(p, l)
}
