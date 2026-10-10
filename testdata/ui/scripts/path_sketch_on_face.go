// The sketch on a face of path_sketch_on_face.script, made by a script
// (TestPathEquivalenceSketchOnFace): the extrusion of path_extrude with
// a sketch on the top face the mouse picks.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	block := doc.Extrude(80, "")
	doc.SketchOnFace(block.Solid().Face(0x8003002c), "")
}
