// The model of path_dimension_name.script, made by a script
// (TestPathEquivalenceDimensionName): a rectangle with the mouse's
// corners, its bottom side's length named width and its right side's
// width/2, extruded 80 mm as the interface does by default (roadmap
// 16.1).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	doc := Current()
	sides := doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	width := doc.Constrain.Length(sides[0], 36)
	height := doc.Constrain.Length(sides[1], 26)
	doc.Edit.Regen()
	width.SetName("width")
	must(width.SetExpr("40"))
	doc.Edit.Regen()
	must(height.SetExpr("width/2"))
	doc.Edit.Regen()
	doc.Features.Extrude(80, "")
}
