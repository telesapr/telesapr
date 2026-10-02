// The model of path_move_group.script, made by a script
// (TestPathEquivalenceMoveGroup): the sketch in 3d moves before the
// rectangle's sketch, as two moves one place earlier do. The mouse
// puts the line's first point at z -0.
package main

import (
	"math"

	. "github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	doc := Current()
	sketch := doc.Active()
	doc.Rect(V(-20, 20, 0), V(16, -6, 0))
	doc.Extrude(80, "")
	in3d := doc.Sketch3D("")
	doc.Line(V(-20, -20, math.Copysign(0, -1)), V(20, -20, 0))
	if err := in3d.MoveBefore(sketch); err != nil {
		panic(err)
	}
}
