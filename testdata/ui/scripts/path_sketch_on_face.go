// The sketch on a face of path_sketch_on_face.script, made by a script
// (TestPathEquivalence/sketch_on_face): the extrusion of path_extrude with
// a sketch on the top face the mouse picks.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	block := doc.Features.Extrude(80, "")
	doc.Features.SketchOnFace(block.Solid().Face(0x8003002c), "")
}
