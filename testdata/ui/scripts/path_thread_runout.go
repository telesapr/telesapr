// The model of path_thread_runout.script, made by a script
// (TestPathEquivalenceThreadRunOut): a triple-start thread whose starts
// all begin at the circle's plane, fading out before its end. The mouse
// puts the circle's centre on the workplane's origin, which constrains
// it there, and picks the extrusion's top circle to thread.
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
	doc.Features.Thread(top, ThreadOptions{Designation: "M20xPh3P1", Length: 30, RunOut: 4, LevelStarts: true})
}
