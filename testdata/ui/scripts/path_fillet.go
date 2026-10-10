// The fillet of path_fillet.script, made by a script
// (TestPathEquivalenceFillet): the top edge of the extrusion of
// path_extrude that the mouse picks, at the radius the menu starts
// with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	doc.Features.Fillet([]Entity{doc.Entity(0x80030004)}, 1, "")
}
