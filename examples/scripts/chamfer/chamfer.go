// A block with its top edges chamfered, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/chamfer/chamfer.go [distance]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/chamfer/chamfer.go
//
// The four edges round the top of a 40 x 30 x 10 block are cut, as New
// Group > Chamfer Edges does with them selected, at distance mm (2
// unless given). The script prints the volume taken off, which for a
// 45 degree chamfer is half the distance squared along each edge, less
// what the corners share.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	distance := 2.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		distance = v
	}

	doc := api.Current()
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	block := doc.Features.Extrude(10, "block")
	before := block.Analyze().Check().Volume

	// The edges round the top are the copies of the sketch's lines at
	// the far end of the extrusion.
	var top []api.Entity
	for _, e := range block.Entities() {
		if e.Kind() == "line" && math.Abs(e.Start().Pos().Z-10) < 1e-9 && math.Abs(e.End().Pos().Z-10) < 1e-9 {
			top = append(top, e)
		}
	}
	c := doc.Features.Chamfer(top, distance, "chamfer")
	if p := c.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := c.Analyze().Check()
	fmt.Printf("%d edges at %g mm: %.2f mm³ taken off, sound %v\n",
		len(top), c.AsChamfer().Distance(), before-r.Volume, r.Sound())
	if !r.Sound() {
		os.Exit(1)
	}
}
