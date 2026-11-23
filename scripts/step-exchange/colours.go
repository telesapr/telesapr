//go:build ignore

// A model for scripts/step-exchange.sh whose parts have colours of their
// own: a blue block with a grey pin assembled into it, saved as
// colours.tlspr in the directory it runs in.
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(30, 20, 0))
	block := doc.Features.Extrude(10, "block")
	block.SetColor(90, 140, 200)
	doc.Features.SketchIn(doc.Workplane("XY"), "")
	doc.Sketch.Circle(api.V(15, 10, 0), 4)
	pin := doc.Features.Extrude(25, "pin")
	pin.SetCombine(api.Assemble)
	pin.SetColor(180, 180, 190)
	if err := doc.File.Save("colours.tlspr"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
