// The model of path_thread_fit.script, made by a script
// (TestPathEquivalence/thread_fit): a thread of class 7H, a quarter turn
// on. The mouse puts the circle's centre on
// the workplane's origin, which constrains it there, and picks the
// extrusion's top circle to thread down into it.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	circle := doc.Sketch.Circle(V(0, 0, 0), 10)
	doc.Constrain.Coincident(doc.Sketch.ActiveWorkplane().Points()[0], circle.Center())
	ex := doc.Features.Extrude(80, "")
	var top Entity
	for _, e := range ex.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 40 {
			top = e
		}
	}
	doc.Features.Thread(top, ThreadOptions{Designation: "M6x1", Length: 9, Internal: true, Reverse: true, Class: "7H", Phase: 90})
}
