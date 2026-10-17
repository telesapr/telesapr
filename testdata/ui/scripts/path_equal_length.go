// A point as far from a line segment as another line segment is long,
// as path_equal_length.script makes it with the mouse
// (TestPathEquivalenceEqualLength).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	// The bottom, right, top and left sides, as the rectangle tool
	// makes them.
	lines := doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	point := doc.Sketch.Point(V(-10, 10, 0))
	doc.Constrain.Equal(lines[2], lines[1], point)
}
