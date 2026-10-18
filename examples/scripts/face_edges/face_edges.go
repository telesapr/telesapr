// The edges round a face of a solid, and a chamfer of them, made by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/face_edges/face_edges.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/face_edges/face_edges.go
//
// SolidFace.Edges gives the edges where a face meets other faces; a
// cylinder's seams are none of them. In the window, right-click a face
// and choose Select Face Edges to select them, and Fillet Edges or
// Chamfer Edges takes them; Edit.SelectFaceEdges does the same from a
// script run in the application. The script here chamfers the four edges
// round the top of a block, which needs no selection.
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 20, 0))
	block := doc.Features.Extrude(10, "block")
	top := block.Solid().FaceAt(api.V(20, 10, 10))

	edges := top.Edges()
	fmt.Printf("the top of the block has %d edges and %d neighbours\n", len(edges), len(top.Neighbours()))
	for _, e := range edges {
		fmt.Println(" ", e.Describe())
	}

	doc.Edit.SelectFaceEdges(top)
	chamfer := doc.Features.ChamferSolid(edges, 1, "top chamfer")
	if p := chamfer.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, "the chamfer:", p)
		os.Exit(2)
	}
	fmt.Println("chamfered", chamfer.Name())
}
