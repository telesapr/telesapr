// The equal angles of path_equal.script, made by a script
// (TestPathEquivalence/equal): the two corners of a rectangle equal.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	// The bottom, right, top and left sides, as the rectangle tool
	// makes them.
	lines := doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Constrain.EqualAngle(lines[2], lines[1], lines[0], lines[3])
}
