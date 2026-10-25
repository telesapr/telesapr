// The model of path_sketch_on_lines.script, made by a script
// (TestPathEquivalence/sketch_on_lines): two line segments, from (-20, -4)
// to (0, -4) and from (20, -10) to (20, 10), and a sketch in the
// workplane through the first one's start parallel to both.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	a := doc.Sketch.Line(V(-20, -4, 0), V(0, -4, 0))
	b := doc.Sketch.Line(V(20, -10, 0), V(20, 10, 0))
	doc.Features.SketchOnLines(a.Start(), a, b, "")
}
