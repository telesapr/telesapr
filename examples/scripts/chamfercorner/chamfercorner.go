// A block with every edge chamfered at two distances, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/chamfercorner/chamfercorner.go [d1 [d2]] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/chamfercorner/chamfercorner.go
//
// The twelve edges of a 40 x 30 x 20 block are cut in one chamfer, 2 mm
// on one face of each edge and 4 mm on the other unless given. At each
// corner three chamfers of unlike distances meet: one ends across
// between where its lines cross the others', the other two are cut
// square to their edges, and the hole left is filled by a patch, as in
// the setback example's fillets. The script checks the solid is sound
// and takes more than the corners' points alone would, prints its
// volume, and saves chamfercorner.tlspr, unless given "-".
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	ds, write := []float64{2, 4}, true
	n := 0
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if d, err := strconv.ParseFloat(a, 64); err == nil && n < 2 {
			ds[n] = d
			n++
		}
	}
	w, l, h := 40.0, 30.0, 20.0
	doc := api.Current()
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(w, l, 0))
	block := doc.Features.Extrude(h, "block")

	c := doc.Features.ChamferSolid(block.Solid().Edges(), ds[0], "corners")
	c.AsChamfer().SetSecondDistance(ds[1])
	if p := c.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := c.Analyze().Check()
	// Each edge's prism of the two distances, run to the corners: more
	// than the solid loses, for the corners' prisms overlap.
	prisms := ds[0] * ds[1] / 2 * 4 * (w + l + h)
	fmt.Printf("every edge chamfered at %g and %g mm, the corners filled: %s, %.1f mm³ (%.1f less every edge's prism)\n",
		ds[0], ds[1], map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, w*l*h-prisms)
	// The block's six faces, a strip for each edge and a face for each
	// corner.
	if faces := len(c.Solid().Faces()); faces != 6+12+8 {
		fmt.Fprintf(os.Stderr, "%d faces, want 26\n", faces)
		os.Exit(1)
	}
	if !r.Sound() || r.Volume <= w*l*h-prisms || r.Volume >= w*l*h {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("chamfercorner.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
