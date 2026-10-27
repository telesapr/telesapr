// The chamfer of path_chamfer_edges.script, made by a script
// (TestPathEquivalence/chamfer_edges): two top edges of the extrusion
// of path_extrude cut, the second taken away, then the first replaced
// by the second (ROADMAP F7).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	c := doc.Features.Chamfer([]Entity{doc.Entity(0x80030004), doc.Entity(0x8003001f)}, 1, "")
	c.AsChamfer().SetEdges([]Entity{doc.Entity(0x80030004)}, []SolidEdge{})
	c.AsChamfer().SetEdges([]Entity{doc.Entity(0x8003001f)}, []SolidEdge{})
}
