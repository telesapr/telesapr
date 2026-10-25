// The model of path_thread_face.script, made by a script
// (TestPathEquivalence/thread_face). The mouse puts the circle's centre
// on the workplane's origin, which constrains it there, and picks the
// extrusion's side to thread, from its other end.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	circle := doc.Sketch.Circle(V(0, 0, 0), 10)
	doc.Constrain.Coincident(doc.Sketch.ActiveWorkplane().Points()[0], circle.Center())
	ex := doc.Features.Extrude(80, "")
	side := ex.Solid().FaceAt(V(7.0710678, 7.0710678, 40))
	doc.Features.ThreadOnFace(side, ThreadOptions{Designation: "M20x1.5", Length: 25, Reverse: true})
}
