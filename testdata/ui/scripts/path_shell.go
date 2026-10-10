// The shell of path_shell.script, made by a script
// (TestPathEquivalenceShell): the extrusion of path_extrude hollowed to
// walls of the thickness the menu starts with, open at the face the
// mouse picks.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	doc.Features.Shell([]Entity{doc.Entity(0x8003002c)}, 1, "")
}
