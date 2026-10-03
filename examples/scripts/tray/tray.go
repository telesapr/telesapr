// A tray: a block with a pocket whose inside edges are filled, made by
// a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/tray/tray.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/tray/tray.go
//
// Fillet Edges takes edges that go in as well as edges that stick out:
// those it fills. Here a 60 x 40 x 15 block with a 50 x 30 pocket 10
// deep cut in its top, the pocket's floor edges and its upright edges
// filled at 3 mm unless given; where three meet, in the pocket's
// corners, a sphere fills the corner. The script checks the solid
// against the volume by formula, prints it, and saves tray.tlspr, unless
// given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	radius, write := 3.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil {
			radius = r
		}
	}
	doc := api.Current()
	doc.SetChordTolerance(0.01)
	doc.Rect(api.V(0, 0, 0), api.V(60, 40, 0))
	doc.Extrude(15, "block")
	// The pocket's sketch on the block's top, through a point there, and
	// cut down 10.
	doc.Sketch3D("")
	doc.SketchAt(doc.Point(api.V(0, 0, 15)), api.V(1, 0, 0), api.V(0, 1, 0), "pocket-sketch")
	doc.Rect(api.V(5, 5, 15), api.V(55, 35, 15))
	pocket := doc.Extrude(-10, "pocket")
	pocket.SetCombine(api.Difference)

	solid := pocket.Solid()
	var edges []api.SolidEdge
	corners := []api.Vec{api.V(5, 5, 0), api.V(55, 5, 0), api.V(55, 35, 0), api.V(5, 35, 0)}
	for i, c := range corners {
		n := corners[(i+1)%4]
		edges = append(edges, solid.EdgeAt(api.V((c.X+n.X)/2, (c.Y+n.Y)/2, 5)), solid.EdgeAt(api.V(c.X, c.Y, 10)))
	}
	tray := doc.FilletSolid(edges, radius, "filled")
	if p := tray.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := tray.Check()
	// What fillets of the pocket's box would take from it: each edge a
	// square of the radius less a quarter circle along its length, and
	// at each corner where three meet the corner cube they count three
	// times less the sphere's eighth.
	a := radius * radius * (1 - math.Pi/4)
	length := 2*(50+30) + 4*10.0
	r3 := radius * radius * radius
	want := 60*40*15 - 50*30*10 + a*length + 4*r3*(7*math.Pi/12-2)
	fmt.Printf("pocket filled at %g mm: %s, %.1f mm³ (%.1f by formula)\n",
		radius, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("tray.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
