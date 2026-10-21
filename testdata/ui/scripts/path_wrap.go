// The wrap of path_wrap.script, made by a script
// (TestPathEquivalenceWrap): a rectangle lathed into a cylinder about its
// lower edge, a sketch in a workplane through a corner of the
// rectangle, and a small rectangle in it raised from the cylinder the
// mouse picks by the height the menu starts with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-10, 20, 0), V(10, 10, 0))
	// The rectangle's lower edge, which the mouse picks.
	cyl := doc.Features.Lathe(doc.Entity(0x50000), "")
	doc.Features.SketchAt(doc.Entity(0x80030020), V(1, 0, 0), V(0, 1, 0), "")
	doc.Sketch.Rect(V(-5, 13, 0), V(-1, 17, 0))
	doc.Features.Emboss(cyl.Solid().FaceAt(V(6, 4, 8)), 1, "")
}
