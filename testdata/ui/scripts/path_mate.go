// The mate of path_mate.script, made by a script
// (TestPathEquivalenceMate): two circles where the mouse draws them,
// made concentric, which in a workplane is their centres on each other
// (roadmap 16.9).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	a := doc.Sketch.Circle(V(-2, 7, 0), 10)
	b := doc.Sketch.Circle(V(20, 7, 0), 6)
	doc.Edit.Regen()
	doc.Assembly.Concentric(a, b)
}
