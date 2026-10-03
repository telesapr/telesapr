// Writes ramp.tlspr, a model for the uneven mitre's scenario: a
// trapezoid drawn upright, 20 along its foot, 15 along its top and 10
// high, extruded 10, so its top meets its front square and its slope at
// 116.6 degrees.
//
//	telesapr-cli run testdata/topo/ramp.go testdata/topo/ramp.tlspr
package main

import (
	"fmt"
	"os"

	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	doc.SketchIn(doc.Workplane("ZX"), "profile")
	doc.Polyline(true, V(0, 0, 0), V(20, 0, 0), V(15, 0, 10), V(0, 0, 10))
	doc.Extrude(10, "ramp")
	if len(os.Args) > 1 {
		if err := doc.Save(os.Args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
