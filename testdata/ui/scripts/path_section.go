// The section view of path_section.script, set by a script
// (TestPathEquivalence/section): the extrusion of path_extrude, cut
// parallel to its fifth face, the one the mouse clicks, 10 mm into the
// solid, keeping the outer side.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	ext := doc.Features.Extrude(80, "")
	doc.View.SetSection(ext.Faces()[4], "-10", true)
}
