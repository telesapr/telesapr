// The mirror of path_mirror.script, made by a script
// (TestPathEquivalenceMirror): the extrusion of path_extrude reflected
// in its fifth face, the one the mouse clicks.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	ext := doc.Extrude(80, "")
	doc.Mirror(ext.Faces()[4], "")
}
