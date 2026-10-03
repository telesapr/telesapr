// The fillet of path_fillet_circle.script, made by a script
// (TestPathEquivalenceFilletCircle): the circle the mouse draws,
// extruded as Shift+X does, its top circle rounded at the radius the menu
// starts with.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	doc.Circle(V(-2, 7, 0), 10)
	doc.Extrude(80, "")
	doc.Fillet([]Entity{doc.Entity(0x80030005)}, 1, "")
}
