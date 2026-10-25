// The model of path_redundant_dimension.script, made by a script
// (TestPathEquivalence/redundant_dimension): a rectangle with its width
// and height, and a diagonal that the group already holds, which the
// sketch makes a reference dimension as the interface does.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	sides := doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Constrain.Length(sides[0], 36)
	doc.Constrain.Length(sides[1], 26)
	doc.Edit.Regen()
	doc.Constrain.Distance(sides[2].End(), sides[0].End(), 44.4)
}
