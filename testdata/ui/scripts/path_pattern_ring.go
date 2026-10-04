// The model of path_pattern_ring.script, made by a script
// (TestPathEquivalencePatternRing): a post repeated round a circle.
package main

import (
	"math"

	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	doc.Circle(V(30, -10, 0), 2)
	doc.Circle(V(10, -10, 0), 20).SetConstruction(true)
	post := doc.Extrude(80, "")
	// The copy of the ring at the top of the extrusion, which the mouse
	// picks.
	var ring Entity
	for _, e := range post.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z == 80 && math.Abs(e.Radius()-20) < 1e-9 {
			ring = e
		}
	}
	doc.PatternAlongPath([]Entity{ring}, 3, "")
}
