// The chamfer of path_chamfer.script, made by a script
// (TestPathEquivalenceChamfer): the two top edges of the extrusion of
// path_extrude that the mouse picks, at the distance the menu starts
// with.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "")
	doc.Chamfer([]Entity{doc.Entity(0x80030004), doc.Entity(0x8003001f)}, 1, "")
}
