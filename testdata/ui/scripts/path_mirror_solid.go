// The model of path_mirror_solid.script, made by a script
// (TestPathEquivalence/mirror_solid).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	ext := doc.Features.Extrude(80, "")
	m := doc.Features.Mirror(ext.Faces()[4], "")
	m.AsMirror().SetMirrorSolid(true)
}
