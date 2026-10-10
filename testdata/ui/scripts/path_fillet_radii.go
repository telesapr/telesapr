// The fillet of path_fillet_radii.script, made by a script
// (TestPathEquivalenceFilletRadii): the two top edges of the extrusion
// of path_extrude that the mouse picks, at the radius the menu starts
// with, the second at 3 of its own.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	f := doc.Features.Fillet([]Entity{doc.Entity(0x80030004), doc.Entity(0x80030016)}, 1, "")
	f.AsFillet().SetEdgeRadius(1, 3)
}
