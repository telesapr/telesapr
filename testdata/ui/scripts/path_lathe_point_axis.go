// The model of path_lathe_point_axis.script, made by a script
// (TestPathEquivalenceLathePointAxis): a rectangle turned about a
// construction line through the point at its start.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(10, 20, 0), V(24, 0, 0))
	axis := doc.Sketch.Line(V(0, 30, 0), V(0, -10, 0))
	axis.SetConstruction(true)
	doc.Features.LatheAbout(axis.Start(), axis, "")
}
