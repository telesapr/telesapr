// Writes frustum.tlspr, a model for the cone fillet's scenario: a
// trapezoid in the ZX plane, 10 along its foot, 6 along its top and 8
// high, turned by a lathe about the z axis, so its flat top meets a
// cone.
//
//	telesapr-cli run testdata/topo/frustum.go testdata/topo/frustum.tlspr
package main

import (
	"fmt"
	"os"

	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	doc.Features.SketchIn(doc.Workplane("ZX"), "profile")
	axis := doc.Sketch.Line(V(0, 0, -2), V(0, 0, 12))
	axis.SetConstruction(true)
	doc.Sketch.Polyline(true, V(0, 0, 0), V(10, 0, 0), V(6, 0, 8), V(0, 0, 8))
	doc.Features.Lathe(axis, "frustum")
	if len(os.Args) > 1 {
		if err := doc.File.Save(os.Args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
