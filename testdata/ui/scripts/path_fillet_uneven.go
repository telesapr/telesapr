// The fillet of path_fillet_uneven.script, made by a script
// (TestPathEquivalenceFilletUneven): the ramp of testdata/topo/ramp.spcs,
// the two edges the mouse picks at the top's front corner by their
// entities, at the radius the menu starts with.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/topo/ramp.spcs"); err != nil {
		panic(err)
	}
	doc.Fillet([]Entity{doc.Entity(0x8004001a), doc.Entity(0x80040013)}, 1, "")
}
