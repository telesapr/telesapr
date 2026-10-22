// The equalities of a sketch, made from a script: a point as far from a
// line segment as another line segment is long, two angles equal, and
// how far one point is from another along a line segment.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/equal/equal.go
//
// In the application, on the sketch that is open: Script > Run File,
// or
//
//	telesapr --script examples/scripts/equal/equal.go
//
// It writes nothing.
package main

import (
	"fmt"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()

	// Two lines held where they are drawn, and a point: the point ends
	// up as far from the second line as the first is long.
	along := doc.Sketch.Line(api.V(0, 0, 0), api.V(30, 0, 0))
	up := doc.Sketch.Line(api.V(0, 0, 0), api.V(0, 20, 0))
	for _, l := range []api.Entity{along, up} {
		doc.Constrain.Lock(l.Start())
		doc.Constrain.Lock(l.End())
	}
	point := doc.Sketch.Point(api.V(5, 4, 0))
	doc.Constrain.Equal(along, up, point)
	fmt.Printf("the point is %v, as far from the line as the other is long\n", point.Pos())

	// Two corners: the angles at their vertices are held equal.
	a := doc.Sketch.Line(api.V(0, 30, 0), api.V(30, 30, 0))
	b := doc.Sketch.Line(api.V(30, 30, 0), api.V(30, 40, 0))
	c := doc.Sketch.Line(api.V(0, 50, 0), api.V(20, 50, 0))
	e := doc.Sketch.Line(api.V(20, 50, 0), api.V(20, 62, 0))
	doc.Constrain.Coincident(a.End(), b.Start())
	doc.Constrain.Coincident(c.End(), e.Start())
	equal := doc.Constrain.EqualAngle(a, b, c, e)
	// Moving one side of the second corner turns it as the equality
	// requires.
	e.End().MoveTo(api.V(20, 70, 0))
	fmt.Printf("the two corners: %s, solved %t\n", equal.String(), doc.Active().Solved())

	// The same kind of equality as the core package names it, on four
	// entities, as File > Export > Script writes it: two more corners.
	f := doc.Sketch.Line(api.V(40, 30, 0), api.V(70, 30, 0))
	g := doc.Sketch.Line(api.V(70, 30, 0), api.V(70, 40, 0))
	doc.Constrain.Coincident(f.End(), g.Start())
	raw := doc.Constrain.RawFour(core.ConstraintEqualAngle, api.Entity{}, api.Entity{}, a, b, f, g, false, false)
	fmt.Printf("raw: %s, solved %t\n", raw.String(), doc.Active().Solved())

	// How far one point is from another along a line segment, which is
	// the gap between two parallel faces.
	left := doc.Sketch.Point(api.V(0, -10, 0))
	right := doc.Sketch.Point(api.V(12, -14, 0))
	gap := doc.Constrain.ProjectedDistance(left, right, along, 12)
	fmt.Printf("the gap between the points is %g mm\n", gap.Value())
}
