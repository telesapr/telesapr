// The circles of path_auto_constrain.script, made by a script
// (TestPathEquivalence/auto_constrain): the second nearly as big as the
// first, then given the constraint the window adds as it is drawn
// (roadmap 29.4).
package main

import (
	"math"

	. "github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := Current()
	doc.Sketch.Circle(V(30, 20, 0), 8)
	c := doc.Sketch.Circle(V(30, -10, 0), math.Sqrt(40*40+1)/5)
	doc.Constrain.Auto(c)
}
