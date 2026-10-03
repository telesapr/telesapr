// The fillet of path_fillet_concave.script, made by a script
// (TestPathEquivalenceFilletConcave): a block, a smaller one on its top
// sketched in a workplane through its corner, and the smaller block's
// foot edge the mouse picks filled at the radius the menu starts with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "")
	doc.SketchAt(doc.Entity(0x80030021), V(1, 0, 0), V(0, 1, 0), "")
	doc.Rect(V(-16, 16, 80), V(0, 4, 80))
	doc.Extrude(80, "")
	doc.Fillet([]Entity{doc.Entity(0x8005000a)}, 1, "")
}
