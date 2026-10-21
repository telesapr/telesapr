// The model of path_shell_lathe.script, made by a script
// (TestPathEquivalenceShellLathe): a rectangle turned about a
// construction line into a ring, hollowed to walls of the thickness the
// menu starts with, open at its outer side.
package main

import (
	"math"

	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	doc.Sketch.Rect(V(10, 20, 0), V(24, 0, 0))
	axis := doc.Sketch.Line(V(0, 30, 0), V(0, -10, 0))
	axis.SetConstruction(true)
	ring := doc.Features.Lathe(axis, "")
	// The outer side, seen from above at x = 20.
	side := ring.Solid().FaceAt(V(20, 10, math.Sqrt(24*24-20*20)))
	doc.Features.ShellSolid([]SolidFace{side}, 1, "")
}
