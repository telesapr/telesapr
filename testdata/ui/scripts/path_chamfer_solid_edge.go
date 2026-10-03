// The chamfer of path_chamfer_solid_edge.script, made by a script
// (TestPathEquivalenceChamferSolidEdge): the edge where the notch of
// testdata/topo/slot.tlspr meets the front, by its name, at the distance
// the menu starts with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/topo/slot.tlspr"); err != nil {
		panic(err)
	}
	edge := doc.Group("slot").Solid().Edge("edge 80030010 80050019 0")
	doc.ChamferSolid([]SolidEdge{edge}, 1, "")
}
