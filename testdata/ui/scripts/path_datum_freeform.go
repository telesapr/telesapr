// The datum of path_datum_freeform.script, made by a script
// (TestPathEquivalence/datum_freeform): a side of the loft of
// testdata/solid/loft-circle-to-square.tlspr, neither flat nor round,
// by its identity, named "side": the plane touching it at its middle.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/solid/loft-circle-to-square.tlspr"); err != nil {
		panic(err)
	}
	doc.Datums.NameFace("side", doc.Group("loft").Solid().Face(0x40060004))
}
