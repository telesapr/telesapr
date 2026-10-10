// The line of path_nudge.script, made by a script
// (TestPathEquivalenceNudge): drawn with the mouse's ends and moved by a
// pixel of the view right, then ten up, as the arrow keys move it.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	l := doc.Sketch.Line(V(-20, 10, 0), V(10, 10, 0))
	doc.Edit.Regen()
	l.MoveBy(V(0.2, 0, 0))
	doc.Edit.Regen()
	l.MoveBy(V(0, 2, 0))
}
