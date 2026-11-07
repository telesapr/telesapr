// A drawing in words: what Doc.Describe, Group.Describe and
// Group.DescribeSolid say of a plate with a hole, which is what an AI program reads of a drawing
// before it writes a script for it (telesapr-cli mcp, doc/guide/ai.md).
//
// Without a window:
//
//	telesapr-cli run examples/scripts/describe/describe.go
//
// In the application, on the sketch that is open: Script > Run File,
// or
//
//	telesapr --script examples/scripts/describe/describe.go
//
// It writes nothing.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	doc.Constrain.Coincident(sides[0].Start(), doc.Origin())
	doc.Constrain.Length(sides[0], 40)
	doc.Constrain.Length(sides[1], 20)
	hole := doc.Sketch.Circle(api.V(10, 10, 0), 3)
	doc.Constrain.Diameter(hole, 6)
	outline := doc.Active()
	doc.Features.Extrude(4, "plate")

	// The whole drawing: the groups in order, then the active one's
	// requests and constraints.
	text := doc.Describe()
	fmt.Print(text)

	// One group, by the name the description gave it. The hole is
	// held in size but not in place: two degrees of freedom are left.
	fmt.Print(doc.Group("g002").Describe())
	if outline.Dof() != 2 || !strings.Contains(text, "g003-plate, extrude: solved") {
		fmt.Fprintln(os.Stderr, "the description isn't of the plate that was drawn")
		os.Exit(1)
	}

	// The names it gives are the ones the lookups take.
	side := doc.Request("r005").Entity()
	fmt.Printf("%s is %g mm long\n", side, side.Length())

	// The solid's faces and edges, by the IDs and names Solid.Face and
	// Solid.Edge take: what a fillet or a sketch on a face picks from.
	plate := doc.Group("plate")
	solid := plate.DescribeSolid()
	fmt.Print(solid)
	if !strings.Contains(solid, "7 faces, 14 edges") {
		fmt.Fprintln(os.Stderr, "the solid isn't the plate with a hole")
		os.Exit(1)
	}
	top := plate.Solid().FaceAt(api.V(20, 10, 4))
	fmt.Printf("the top is %s, with %d edges\n", top.Name(), len(top.Edges()))
}
