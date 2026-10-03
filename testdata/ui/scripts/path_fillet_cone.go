// The fillet of path_fillet_cone.script, made by a script
// (TestPathEquivalenceFilletCone): the frustum of
// testdata/topo/frustum.spcs, the circle of its top's rim the mouse
// picks rounded at the radius the menu starts with.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/topo/frustum.spcs"); err != nil {
		panic(err)
	}
	doc.Fillet([]Entity{doc.Entity(0x80040022)}, 1, "")
}
