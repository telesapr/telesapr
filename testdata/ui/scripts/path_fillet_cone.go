// The fillet of path_fillet_cone.script, made by a script
// (TestPathEquivalenceFilletCone): the frustum of
// testdata/topo/frustum.tlspr, the circle of its top's rim the mouse
// picks rounded at the radius the menu starts with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/topo/frustum.tlspr"); err != nil {
		panic(err)
	}
	doc.Features.Fillet([]Entity{doc.Entity(0x80040022)}, 1, "")
}
