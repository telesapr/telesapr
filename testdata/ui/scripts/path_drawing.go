// The drawing of path_drawing.script, made by a script
// (TestPathEquivalenceDrawing): the extrusion of path_extrude, its
// drawing on an A2 sheet at 1:2, first angle, titled PLATE (roadmap
// 18.1).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "")
	d := doc.Drawing()
	d.Sheet, d.Scale, d.FirstAngle, d.Title = "A2", 0.5, true, "PLATE"
	doc.SetDrawing(d)
}
