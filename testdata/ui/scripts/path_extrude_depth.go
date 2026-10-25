// The held depth of path_extrude_depth.script, made by a script
// (TestPathEquivalence/extrude_depth): the extrusion of path_extrude, its
// depth held at 25.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "").AsExtrude().SetDepth(25)
}
