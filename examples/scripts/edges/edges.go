// Edges of a solid by name: a notched block, its edges listed, one that
// no entity draws chamfered by name, and the notch moved, made by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/edges/edges.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/edges/edges.go
//
// A Boolean makes edges that no sketch entity draws, such as where the
// notch's wall meets the block's front. Group.Solid gives the solid's
// faces, edges and vertices; an edge's name is the two faces it lies
// between and its place among their edges, so it holds when the model
// upstream changes. The script chamfers the notch's upright edge by
// name, moves the notch through the parameter "notch", and prints where
// the chamfered edge is each time. Then it bores a hole through the
// block: the hole's wall is one face of four patches, joined along
// seams that are no edges, and its rims are one edge each, of four
// pieces. It saves edges.tlspr, unless given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	doc := api.Current()
	must(doc.Params.Set("notch", "15"))
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	doc.Constrain.Coincident(sides[0].Start(), doc.Origin())
	doc.Constrain.Length(sides[0], 40)
	doc.Constrain.Length(sides[1], 30)
	doc.Features.Extrude(10, "block")

	doc.Features.SketchIn(doc.Workplane("XY"), "notch-sketch")
	n := doc.Sketch.Rect(api.V(15, -5, 0), api.V(25, 20, 0))
	doc.Constrain.Length(n[0], 10)
	doc.Constrain.Length(n[1], 25)
	must(doc.Constrain.Distance(n[0].Start(), doc.Workplane("YZ"), 15).SetExpr("notch"))
	doc.Constrain.Distance(n[0].Start(), doc.Workplane("ZX"), 5)
	cut := doc.Features.Extrude(20, "notch")
	cut.SetCombine(api.Difference)

	solid := cut.Solid()
	fmt.Printf("%d faces, %d edges, %d vertices, Euler characteristic %d\n",
		len(solid.Faces()), len(solid.Edges()), len(solid.Vertices()), solid.Euler())
	// The upright edge where the notch's right wall meets the front.
	edge := solid.EdgeAt(api.V(25, 0, 5))
	fmt.Printf("chamfering %s: %s\n", edge.Name(), edge.Describe())
	ch := doc.Features.ChamferSolid([]api.SolidEdge{edge}, 2, "chamfer")

	for _, at := range []string{"15", "5", "22"} {
		must(doc.Params.Set("notch", at))
		if p := ch.Problem(); p != "" {
			fmt.Fprintln(os.Stderr, p)
			os.Exit(1)
		}
		a, b := ch.SolidEdges()[0].Ends()
		r := ch.Analyze().Check()
		fmt.Printf("notch at %s: the edge runs from (%g, %g, %g) to (%g, %g, %g); sound %v, %.1f mm³\n",
			at, a.X, a.Y, a.Z, b.X, b.Y, b.Z, r.Sound(), r.Volume)
		if !r.Sound() {
			os.Exit(1)
		}
	}
	doc.Features.SketchIn(doc.Workplane("XY"), "hole-sketch")
	doc.Sketch.Circle(api.V(8, 22, 0), 4)
	hole := doc.Features.Extrude(10, "hole")
	hole.SetCombine(api.Difference)
	solid = hole.Solid()
	wall := solid.FaceAt(api.V(12, 22, 5))
	rim := solid.EdgeAt(api.V(12, 22, 10))
	fmt.Printf("with the hole: %d faces, %d edges; its wall is %d patches with %d edges, its rim %d pieces, %.2f mm long\n",
		len(solid.Faces()), len(solid.Edges()), wall.Patches(), len(wall.Edges()), rim.Pieces(), rim.Length())

	if len(os.Args) > 1 && os.Args[1] == "-" {
		return
	}
	must(doc.File.Save("edges.tlspr"))
}
