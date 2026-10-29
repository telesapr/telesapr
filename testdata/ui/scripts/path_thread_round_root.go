// The model of path_thread_round_root.script, made by a script
// (TestPathEquivalence/thread_round_root). The mouse puts the circle's
// centre on the workplane's origin, which constrains it there, and picks
// the extrusion's top circle to thread down into it.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	circle := doc.Sketch.Circle(V(0, 0, 0), 10)
	doc.Constrain.Coincident(doc.Sketch.ActiveWorkplane().Points()[0], circle.Center())
	ex := doc.Features.Extrude(80, "")
	doc.Edit.SetChordTolerance(0.02)
	var top Entity
	for _, e := range ex.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 40 {
			top = e
		}
	}
	doc.Features.Thread(top, ThreadOptions{Designation: "1/2-16 PULL-BUTT", Length: 30, Reverse: true, RoundRoot: true})
}
