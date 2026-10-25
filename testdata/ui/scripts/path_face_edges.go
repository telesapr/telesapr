// The chamfer of path_face_edges.script, made by a script
// (TestPathEquivalence/face_edges): the edges of the top face of the
// extrusion of path_extrude that the mouse picks, at the distance the
// menu starts with. A script's document has no window to hold a
// selection, so it takes the edges SolidFace.Edges gives, which Edit.
// SelectFaceEdges selects in a window (TestScriptHostSelectsFaceEdges).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	block := doc.Features.Extrude(80, "")
	top := block.Solid().Face(0x8003002c)
	doc.Features.ChamferSolid(top.Edges(), 1, "")
}
