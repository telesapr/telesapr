// Writes slot.spcs, the model of the topology tests: a 40 x 30 x 10
// block with a notch 10 wide and 20 deep cut into its front, its left
// side at x = slot, a parameter. The notch cuts the block's front in
// two, so the top meets the front along two edges, and its rims are
// edges no entity draws.
//
//	spacesolve-cli run testdata/topo/slot.go testdata/topo/slot.spcs
package main

import (
	"fmt"
	"os"

	. "github.com/spacesolve/spacesolve/pkg/script/api"
	"github.com/spacesolve/spacesolve/pkg/sketch"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	doc := Current()
	must(doc.SetParameter("slot", "15"))
	block := doc.Rect(V(0, 0, 0), V(40, 30, 0))
	doc.Coincident(block[0].Start(), doc.Origin())
	doc.Length(block[0], 40)
	doc.Length(block[1], 30)
	doc.Extrude(10, "block")

	doc.SketchIn(doc.Workplane("XY"), "slot-sketch")
	slot := doc.Rect(V(15, -5, 0), V(25, 20, 0))
	doc.Length(slot[0], 10)
	doc.Length(slot[1], 25)
	must(doc.Distance(slot[0].Start(), doc.Workplane("YZ"), 15).SetExpr("slot"))
	doc.Distance(slot[0].Start(), doc.Workplane("ZX"), 5)
	cut := doc.Extrude(20, "slot")
	cut.SetCombine(sketch.CombineDifference)
	if len(os.Args) > 1 {
		must(doc.Save(os.Args[1]))
	}
	fmt.Println(cut.Check().Summary())
}
