// The fillet of path_fillet_rim.script, made by a script
// (TestPathEquivalence/fillet_rim): the circle at the foot of the boss of
// testdata/topo/boss.tlspr, found by a point on it, filled at the radius
// the menu starts with.
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/topo/boss.tlspr"); err != nil {
		panic(err)
	}
	rim := doc.Group("boss").Solid().EdgeAt(V(20, 23, 0))
	doc.Features.FilletSolid([]SolidEdge{rim}, 1, "")
}
