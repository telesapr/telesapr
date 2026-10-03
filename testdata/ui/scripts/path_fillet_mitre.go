// The fillet of path_fillet_mitre.script, made by a script
// (TestPathEquivalenceFilletMitre): the two top edges of the extrusion
// of path_extrude that the mouse picks, meeting at a corner, at the
// radius the menu starts with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "")
	doc.Fillet([]Entity{doc.Entity(0x80030004), doc.Entity(0x8003001f)}, 1, "")
}
