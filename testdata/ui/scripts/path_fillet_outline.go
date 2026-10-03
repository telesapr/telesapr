// The fillet of path_fillet_outline.script, made by a script
// (TestPathEquivalenceFilletOutline): the rectangle of path_extrude with
// the corner the mouse picks rounded by a tangent arc (the sides kept as
// construction, as the menu does), extruded, and the top side the mouse
// picks rounded at the radius the menu starts with, which goes on round
// the arc.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	sides := doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	for _, s := range sides {
		if p := s.Start().Pos(); p.X == 16 && p.Y == -6 {
			doc.TangentArc(s.Start(), 0, false)
		}
	}
	doc.Extrude(80, "")
	doc.Fillet([]Entity{doc.Entity(0x80030035)}, 1, "")
}
