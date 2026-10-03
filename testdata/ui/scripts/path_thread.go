// The model of path_thread.script, made by a script
// (TestPathEquivalenceThread). The mouse puts the circle's centre on
// the workplane's origin, which constrains it there, and picks the
// extrusion's top circle to thread.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	circle := doc.Circle(V(0, 0, 0), 10)
	doc.Coincident(doc.ActiveWorkplane().Points()[0], circle.Center())
	ex := doc.Extrude(80, "")
	var top Entity
	for _, e := range ex.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > 40 {
			top = e
		}
	}
	doc.Thread(top, ThreadOptions{Designation: "M6x1", Length: 9, Internal: true})
}
