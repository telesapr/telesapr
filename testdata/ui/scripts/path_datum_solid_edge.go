// The datum of path_datum_solid_edge.script, made by a script
// (TestPathEquivalence/datum_solid_edge): the edge where the notch of
// testdata/topo/slot.tlspr meets the front, by its name, named "notch".
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/topo/slot.tlspr"); err != nil {
		panic(err)
	}
	doc.Datums.NameEdge("notch", doc.Group("slot").Solid().Edge("edge 80030010 80050019 0"))
}
