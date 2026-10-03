// The model of path_mirror_solid.script, made by a script
// (TestPathEquivalenceMirrorSolid).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	ext := doc.Extrude(80, "")
	m := doc.Mirror(ext.Faces()[4], "")
	m.SetMirrorSolid(true)
}
