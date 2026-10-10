// The model of path_move_group.script, made by a script
// (TestPathEquivalenceMoveGroup): the sketch in 3d moves before the
// rectangle's sketch, as two moves one place earlier do. The mouse
// puts the line's first point at z -0.
package main

import (
	"math"

	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	sketch := doc.Active()
	doc.Sketch.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Features.Extrude(80, "")
	in3d := doc.Features.Sketch3D("")
	doc.Sketch.Line(V(-20, -20, math.Copysign(0, -1)), V(20, -20, 0))
	if err := in3d.History().MoveBefore(sketch); err != nil {
		panic(err)
	}
}
