// The fillet of path_fillet_circle.script, made by a script
// (TestPathEquivalenceFilletCircle): the circle the mouse draws,
// extruded as Shift+X does, its top circle rounded at the radius the menu
// starts with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Circle(V(-2, 7, 0), 10)
	doc.Features.Extrude(80, "")
	doc.Features.Fillet([]Entity{doc.Entity(0x80030005)}, 1, "")
}
