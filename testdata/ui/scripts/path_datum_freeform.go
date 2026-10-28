// The datum of path_datum_freeform.script, made by a script
// (TestPathEquivalence/datum_freeform): a side of the loft of
// testdata/solid/loft-circle-to-square.tlspr, neither flat nor round,
// by its identity, named "side": the plane touching it where it was
// picked, the point the mouse hit given here.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/solid/loft-circle-to-square.tlspr"); err != nil {
		panic(err)
	}
	doc.Datums.NameFaceAt("side", doc.Group("loft").Solid().Face(0x40060004), V(-0.26189630213917314, -3.3582418968306533, 7.504989091020432))
}
