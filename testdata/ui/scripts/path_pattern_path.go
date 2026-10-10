// The model of path_pattern_path.script, made by a script
// (TestPathEquivalencePathPattern).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Circle(V(-20, 0, 0), 2)
	doc.Sketch.Line(V(-20, 20, 0), V(20, 20, 0)).SetConstruction(true)
	post := doc.Features.Extrude(80, "")
	// The copy of the line at the top of the extrusion, which the mouse
	// picks.
	var path Entity
	for _, e := range post.Entities() {
		if e.Kind() == "line" && e.Start().Pos().Z == 80 && e.End().Pos().Z == 80 {
			path = e
		}
	}
	doc.Features.PatternAlongPath([]Entity{path}, 3, "")
}
