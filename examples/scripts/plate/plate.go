// A mounting plate driven by named parameters, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/plate/plate.go [width]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/plate/plate.go
//
// The plate's width, height, thickness and hole come from the
// parameter table (Edit > Parameters), so changing "width" there, or
// with doc.SetParameter, resizes everything that depends on it. It
// writes plate.tlspr where it is run, unless it is given "-" as a
// second argument.
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
	width, write := "60", true
	if len(os.Args) > 1 {
		width = os.Args[1]
	}
	if len(os.Args) > 2 && os.Args[2] == "-" {
		write = false
	}

	doc := api.Current()
	// The table: each later parameter names earlier ones. Lengths are
	// in mm.
	must(doc.Params.Set("width", width))
	must(doc.Params.Set("height", "width*2/3"))
	must(doc.Params.Set("margin", "height/5"))
	must(doc.Params.Set("hole", "margin*0.8"))

	// The outline, its sides named by the parameters.
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(60, 40, 0))
	doc.Constrain.Coincident(sides[0].Start(), doc.Origin())
	must(doc.Constrain.Length(sides[0], 60).SetExpr("width"))
	must(doc.Constrain.Length(sides[1], 40).SetExpr("height"))

	// A hole in each corner, margin from both sides.
	corners := []api.Vec{api.V(8, 8, 0), api.V(52, 8, 0), api.V(52, 32, 0), api.V(8, 32, 0)}
	for i := 0; i < 4; i++ { // the interpreter has no range over an int
		h := doc.Sketch.Circle(corners[i], 3)
		must(doc.Constrain.Diameter(h, 6).SetExpr("hole"))
		must(doc.Constrain.Distance(h.Center(), sides[i], 8).SetExpr("margin"))
		must(doc.Constrain.Distance(h.Center(), sides[(i+3)%4], 8).SetExpr("margin"))
	}
	body := doc.Features.Extrude(5, "plate")

	r := body.Analyze().Check()
	fmt.Printf("%s: %d faces, %.1f mm³, sound %v\n", body, r.Faces, r.Volume, r.Sound())
	for _, p := range doc.Params.List() {
		fmt.Printf("  %s = %s = %g\n", p.Name, p.Expr, p.Value)
	}
	if !r.Sound() || len(doc.Params.Errors()) > 0 {
		fmt.Fprintln(os.Stderr, r.Summary(), doc.Params.Errors())
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("plate.tlspr"))
	}
}
