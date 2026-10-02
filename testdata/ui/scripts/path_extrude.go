// The model of path_extrude.script, made by a script
// (TestPathEquivalenceExtrude): the mouse's corners are at (-20, 20)
// and (16, -6), and the interface extrudes 80 mm by default.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "")
}
