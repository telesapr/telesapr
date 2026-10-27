// The fillet of path_fillet_edges.script, made by a script
// (TestPathEquivalence/fillet_edges): two top edges of the extrusion of
// path_extrude rounded, the second taken away, then the first replaced
// by the second (ROADMAP F7).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	f := doc.Features.Fillet([]Entity{doc.Entity(0x80030004), doc.Entity(0x8003001f)}, 1, "")
	f.AsFillet().SetEdges([]Entity{doc.Entity(0x80030004)}, []SolidEdge{})
	f.AsFillet().SetEdges([]Entity{doc.Entity(0x8003001f)}, []SolidEdge{})
}
