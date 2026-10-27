// The mirror of path_mirror_plane.script, made by a script
// (TestPathEquivalence/mirror_plane): the extrusion of path_extrude
// mirrored in its top face, then in the YZ workplane (ROADMAP F7).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	m := doc.Features.Mirror(doc.Entity(0x8003002c), "")
	m.AsMirror().SetPlane(doc.Workplane("YZ"))
}
