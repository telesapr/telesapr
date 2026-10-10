// A telescopic rod between a shortest and a longest length, its end
// riding a slot, made by a script: minimum and maximum distances and a
// point on a segment.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/limits/limits.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/limits/limits.go
//
// The rod runs along x from a fixed end; MinDistance and MaxDistance
// hold its length between 20 and 40 mm, and leave it free in between.
// A slider below the rod's end is held on the slot from x = 0 to 35 by
// OnSegment, which, unlike On, keeps it between the slot's ends. The
// rod's end is then put at x = 10, 30 and 60: the rod stops at 20, takes
// about 30, and stops at 35, where the slot ends before the rod's
// longest.
// The script checks each length and the freedom left, which is one
// between the limits and none at one, and saves limits.tlspr, unless
// given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()
	none := api.Entity{}

	rod := doc.Sketch.Line(api.V(0, 0, 0), api.V(30, 0, 0))
	doc.Constrain.Lock(rod.Start())
	doc.Constrain.Horizontal(rod)
	doc.Constrain.MinDistance(rod, none, 20)
	doc.Constrain.MaxDistance(rod, none, 40)

	slot := doc.Sketch.Line(api.V(0, -10, 0), api.V(35, -10, 0))
	doc.Constrain.Lock(slot.Start())
	doc.Constrain.Lock(slot.End())
	slider := doc.Sketch.Point(api.V(30, -10, 0))
	doc.Constrain.OnSegment(slider, slot)
	doc.Constrain.Vertical(slider, rod.End())

	sketch := doc.Groups()[len(doc.Groups())-1]
	failed := false
	for _, step := range []struct {
		to, shortest, longest float64
		dof                   int
	}{
		{10, 20, 20, 0},
		{30, 20, 35, 1},
		{60, 35, 35, 0},
	} {
		// Put there, not dragged: the solver moves the slider and the
		// end alike to meet, so between the limits the rod ends near
		// where it was put, and at one, at it.
		rod.End().MoveTo(api.V(step.to, 0, 0))
		doc.Edit.Regen()
		length := rod.Length()
		fmt.Printf("end put at x = %2.0f: rod %5.2f mm, slider at x = %5.2f, %d DOF\n",
			step.to, length, slider.Pos().X, sketch.Dof())
		if !sketch.Solved() || length < step.shortest-1e-6 || length > step.longest+1e-6 ||
			sketch.Dof() != step.dof {
			fmt.Fprintf(os.Stderr, "want %g to %g mm and %d DOF\n", step.shortest, step.longest, step.dof)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("limits.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
