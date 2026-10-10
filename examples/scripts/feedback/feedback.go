// What a sketch tells a script about itself: the document's unit,
// measurements that add no constraint, how free each entity still is,
// and which constraints keep a group from solving.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/feedback/feedback.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/feedback/feedback.go
//
// It draws a plate 2 by 1 inches in a document whose unit is the inch,
// dimensions it until nothing can move, then over-constrains it and
// takes the extra dimension back. It writes nothing.
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/core"
	"github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	doc := api.Current()

	// Lengths typed as text are in the document's unit from here on, as
	// after View > Dimension Units; the methods that take numbers keep
	// to mm.
	doc.View.SetUnits(core.UnitInches)
	fmt.Printf("unit: %v\n", doc.View.Units() == core.UnitInches)

	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	doc.Constrain.Coincident(sides[0].Start(), doc.Origin())

	// A rectangle with one corner held can still change its size.
	fmt.Printf("before the dimensions, the right side is %v\n", sides[1].Free())
	if sides[1].Free() != sketch.FreedomPartly {
		fail("the right side should be partly free")
	}

	// "2" is 2 inches, since that is the document's unit.
	must(doc.Constrain.Length(sides[0], 50.8).SetExpr("2"))
	must(doc.Constrain.Length(sides[1], 25.4).SetExpr("1"))
	fmt.Printf("after them, it is %v\n", sides[1].Free())
	if sides[1].Free() != sketch.FreedomFixed {
		fail("the right side should be fixed")
	}

	// A measurement adds no constraint, as Analyze > Measure Selection.
	m := doc.Analyze.Measure(sides[0])
	fmt.Printf("the bottom's %s: %.1f mm, %.3f in\n", m.What, m.Value, m.Value/25.4)
	d := doc.Analyze.Measure(sides[0].Start(), sides[2].Start())
	fmt.Printf("corner to corner: %.2f mm\n", d.Value)
	if m.Value < 50.79 || m.Value > 50.81 {
		fail("the bottom measures %g mm, want 50.8", m.Value)
	}

	// The top's length contradicts the bottom's: the group no longer
	// solves, and Conflicts names what could be removed to fix it.
	extra := doc.Constrain.Length(sides[2], 60)
	g := doc.Active()
	fmt.Printf("with a third length, solved: %v\n", g.Solved())
	conflicts := g.Conflicts()
	for _, c := range conflicts {
		fmt.Printf("  could remove %s\n", c)
	}
	if g.Solved() || len(conflicts) == 0 {
		fail("the third length should conflict")
	}
	extra.Delete()
	fmt.Printf("without it, solved: %v\n", g.Solved())
	if !g.Solved() || len(g.Conflicts()) != 0 {
		fail("the sketch should solve again")
	}

	// The commands of the main menu, as Edit > Find Command lists them,
	// and one run as choosing it does.
	cmds := api.Commands()
	fmt.Printf("%d commands, %q first\n", len(cmds), cmds[0])
	must(doc.Edit.RunCommand("View > Zoom To Fit"))
}
