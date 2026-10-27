// The pattern of path_pattern_path_changed.script, made by a script
// (TestPathEquivalence/pattern_path_changed): a post repeated along one
// construction line, then along a second (ROADMAP F7).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Circle(V(-20, 0, 0), 2)
	doc.Sketch.Line(V(-20, 20, 0), V(20, 20, 0)).SetConstruction(true)
	doc.Sketch.Line(V(-20, 10, 0), V(20, 10, 0)).SetConstruction(true)
	post := doc.Features.Extrude(80, "")
	// The copies of the lines at the top of the extrusion, which the
	// mouse picks.
	var first, second Entity
	for _, e := range post.Entities() {
		if e.Kind() == "line" && e.Start().Pos().Z == 80 && e.End().Pos().Z == 80 {
			if e.Start().Pos().Y == 20 {
				first = e
			} else {
				second = e
			}
		}
	}
	p := doc.Features.PatternAlongPath([]Entity{first}, 3, "")
	p.AsRepeat().SetPath([]Entity{second})
}
