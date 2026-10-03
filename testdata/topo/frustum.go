// Writes frustum.spcs, a model for the cone fillet's scenario: a
// trapezoid in the ZX plane, 10 along its foot, 6 along its top and 8
// high, turned by a lathe about the z axis, so its flat top meets a
// cone.
//
//	spacesolve-cli run testdata/topo/frustum.go testdata/topo/frustum.spcs
package main

import (
	"fmt"
	"os"

	. "github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	doc := Current()
	doc.SketchIn(doc.Workplane("ZX"), "profile")
	axis := doc.Line(V(0, 0, -2), V(0, 0, 12))
	axis.SetConstruction(true)
	doc.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(6, 0, 8), V(0, 0, 8))
	doc.Lathe(axis, "frustum")
	if len(os.Args) > 1 {
		if err := doc.Save(os.Args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
