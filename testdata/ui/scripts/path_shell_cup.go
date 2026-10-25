// The cup of path_shell_cup.script, made by a script
// (TestPathEquivalence/shell_cup): the circle the mouse draws, extruded as
// Shift+X does, hollowed to walls of the thickness the menu starts with,
// open at the face the mouse picks (roadmap 15.2).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Circle(V(-2, 7, 0), 10)
	cyl := doc.Features.Extrude(80, "")
	doc.Features.ShellSolid([]SolidFace{cyl.Solid().FaceAt(V(-2, 7, 80))}, 1, "")
}
