// The fillet of path_fillet_rim.script, made by a script
// (TestPathEquivalenceFilletRim): the circle at the foot of the boss of
// testdata/topo/boss.spcs, found by a point on it, filled at the radius
// the menu starts with.
package main

import . "github.com/spacesolve/spacesolve/pkg/script/api"

func main() {
	doc := Current()
	if err := doc.Load("../../testdata/topo/boss.spcs"); err != nil {
		panic(err)
	}
	rim := doc.Group("boss").Solid().EdgeAt(V(20, 23, 0))
	doc.FilletSolid([]SolidEdge{rim}, 1, "")
}
