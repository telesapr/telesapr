// The shell of path_shell_faces.script, made by a script
// (TestPathEquivalence/shell_faces): the extrusion of path_extrude
// hollowed open at its top, then at a side instead (ROADMAP F7).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	ext := doc.Features.Extrude(80, "")
	sh := doc.Features.Shell([]Entity{doc.Entity(0x8003002c)}, 1, "")
	for _, f := range ext.Solid().Faces() {
		if f.ID() == 0x80030019 {
			sh.AsShell().SetFaces([]SolidFace{f})
		}
	}
}
