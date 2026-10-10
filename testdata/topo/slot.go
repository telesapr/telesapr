// Writes slot.tlspr, the model of the topology tests: a 40 x 30 x 10
// block with a notch 10 wide and 20 deep cut into its front, its left
// side at x = slot, a parameter. The notch cuts the block's front in
// two, so the top meets the front along two edges, and its rims are
// edges no entity draws.
//
//	telesapr-cli run testdata/topo/slot.go testdata/topo/slot.tlspr
package main

import (
	"fmt"
	"os"

	. "github.com/telesapr/telesapr/pkg/script/api"
	"github.com/telesapr/telesapr/pkg/sketch"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	doc := Current()
	must(doc.Params.Set("slot", "15"))
	block := doc.Sketch.Rect(V(0, 0, 0), V(40, 30, 0))
	doc.Constrain.Coincident(block[0].Start(), doc.Origin())
	doc.Constrain.Length(block[0], 40)
	doc.Constrain.Length(block[1], 30)
	doc.Features.Extrude(10, "block")

	doc.Features.SketchIn(doc.Workplane("XY"), "slot-sketch")
	slot := doc.Sketch.Rect(V(15, -5, 0), V(25, 20, 0))
	doc.Constrain.Length(slot[0], 10)
	doc.Constrain.Length(slot[1], 25)
	must(doc.Constrain.Distance(slot[0].Start(), doc.Workplane("YZ"), 15).SetExpr("slot"))
	doc.Constrain.Distance(slot[0].Start(), doc.Workplane("ZX"), 5)
	cut := doc.Features.Extrude(20, "slot")
	cut.SetCombine(sketch.CombineDifference)
	if len(os.Args) > 1 {
		must(doc.File.Save(os.Args[1]))
	}
	fmt.Println(cut.Analyze().Check().Summary())
}
