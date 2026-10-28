// The chamfer of path_chamfer_edge_distances.script, made by a script
// (TestPathEquivalence/chamfer_edge_distances): path_chamfer's chamfer,
// its second edge at a distance of 2 and a second distance of 1.5 of
// its own.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	c := doc.Features.Chamfer([]Entity{doc.Entity(0x80030004), doc.Entity(0x8003001f)}, 1, "")
	c.AsChamfer().SetEdgeDistance(1, 2)
	c.AsChamfer().SetEdgeSecondDistance(1, 1.5)
}
