// The model of path_redundant_sets.script, made by a script
// (TestPathEquivalence/redundant_sets): two pairs of lines, each line
// vertical and each pair parallel too, so that the group over-determines
// itself in two sets of three, which Group.Redundancies names.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	a := doc.Sketch.Line(V(-40, 30, 0), V(-34, 0, 0))
	b := doc.Sketch.Line(V(-20, 30, 0), V(-12, 0, 0))
	c := doc.Sketch.Line(V(0, 30, 0), V(10, 0, 0))
	d := doc.Sketch.Line(V(20, 30, 0), V(32, 0, 0))
	doc.Constrain.Vertical(a)
	doc.Constrain.Vertical(b)
	doc.Constrain.Parallel(a, b)
	doc.Constrain.Vertical(c)
	doc.Constrain.Vertical(d)
	doc.Constrain.Parallel(c, d)
}
