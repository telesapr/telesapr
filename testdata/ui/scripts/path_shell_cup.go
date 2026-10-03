// The cup of path_shell_cup.script, made by a script
// (TestPathEquivalenceShellCup): the circle the mouse draws, extruded as
// Shift+X does, hollowed to walls of the thickness the menu starts with,
// open at the face the mouse picks (roadmap 15.2).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Circle(V(-2, 7, 0), 10)
	cyl := doc.Extrude(80, "")
	doc.ShellSolid([]SolidFace{cyl.Solid().FaceAt(V(-2, 7, 80))}, 1, "")
}
