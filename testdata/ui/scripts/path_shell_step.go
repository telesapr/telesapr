// The model of path_shell_step.script, made by a script
// (TestPathEquivalence/shell_step): stepped_ring.tlspr's ring hollowed
// to walls of the thickness the menu starts with, open at the narrow
// part's outer side, which meets the step at an inside corner.
package main

import (
	"math"

	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	if err := doc.File.Load("../../testdata/ui/stepped_ring.tlspr"); err != nil {
		panic(err)
	}
	ring := doc.Group("lathe")
	// The narrow part's outer side, seen from above at x = 9.
	side := ring.Solid().FaceAt(V(9, 15, math.Sqrt(14*14-9*9)))
	doc.Features.ShellSolid([]SolidFace{side}, 1, "")
}
