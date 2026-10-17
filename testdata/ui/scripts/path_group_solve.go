// The solve options of a group, the model of path_group_solve.script
// (TestPathEquivalenceGroupSolve).
package main

import . "github.com/telesapr/telesapr/pkg/script/api"

func main() {
	doc := Current()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	extrude := doc.Features.Extrude(80, "")
	extrude.SetForceToMesh(true)
	extrude.SetRelaxConstraints(true)
	extrude.SetAllowRedundant(true)
	extrude.SetSuppressDof(true)
	extrude.SetAllDimsReference(true)
}
