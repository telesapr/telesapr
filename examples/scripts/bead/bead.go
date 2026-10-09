// A weld bead along two edges of a block that meet at a corner, made by
// a script (roadmap 28.4).
//
// Without a window:
//
//	telesapr-cli run examples/scripts/bead/bead.go [size]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/bead/bead.go
//
// A square of side size (1 unless given), drawn on the block's left end
// across the front top edge, is swept along the front and right top
// edges of a 40 x 20 x 10 block, picked by their names as Features >
// Sweep Along Path does with them selected on the model. At the corner
// between the two edges the sweep is mitred. The sweep keeps the edges'
// names, so it follows them when the block changes. The script prints
// the volume: the block's, and three quarters of the bead's section
// along the 60 mm of path, a quarter of it lying in the block.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	size := 1.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		size = v
	}

	doc := api.Current()
	doc.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	block := doc.Extrude(10, "block")
	s := block.Solid()
	front := s.EdgeAt(api.V(20, 0, 10))
	right := s.EdgeAt(api.V(40, 10, 10))

	doc.SketchIn(doc.Workplane("YZ"), "bead profile")
	h := size / 2
	doc.Rect(api.V(0, -h, 10-h), api.V(0, h, 10+h))
	bead := doc.SweepEdges([]api.SolidEdge{front, right}, nil, "bead")
	if p := bead.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := bead.Check()
	want := 8000 + 0.75*size*size*60
	fmt.Printf("block and bead: %.2f mm³ (about %.0f), sound %v\n", r.Volume, want, r.Sound())
	if !r.Sound() || math.Abs(r.Volume-want) > size*size {
		os.Exit(1)
	}
}
