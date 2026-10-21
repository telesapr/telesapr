// The model of path_drawing_thread.script, made by a script
// (TestPathEquivalenceDrawingThread): the thread of path_thread, its
// drawing set to draw the thread turn by turn, as modelled.
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
	doc.Features.Thread(top, ThreadOptions{Designation: "M6x1", Length: 9, Internal: true})
	d := doc.Drawing.Get()
	d.ModelledThreads = true
	doc.Drawing.Set(d)
}
