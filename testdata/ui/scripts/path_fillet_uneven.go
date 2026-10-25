// The fillet of path_fillet_uneven.script, made by a script
// (TestPathEquivalence/fillet_uneven): the ramp of testdata/topo/ramp.tlspr,
// the two edges the mouse picks at the top's front corner by their
// entities, at the radius the menu starts with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/topo/ramp.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Fillet([]Entity{doc.Entity(0x8004001a), doc.Entity(0x80040013)}, 1, "")
}
