// The drawing of path_drawing_section.script, made by a script
// (TestPathEquivalence/drawing_section): the extrusion of path_extrude,
// and on its drawing a section of the top view 8 mm toward the viewer
// and a detail of the front view, added by the calls the drawing
// screen's links reach.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(20, "")
	doc.Drawing.AddSection("top", 8)
	doc.Drawing.AddDetail("front", 5, 5, 6, 2)
}
