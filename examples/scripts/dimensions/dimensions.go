// A plate whose other sizes read its width dimension by name, made by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/dimensions/dimensions.go [width] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/dimensions/dimensions.go
//
// A dimension can be named, and expressions read its value by the name
// as they read a parameter: here the bottom side's length is named
// width, the side's length is width*2/3 and the hole's diameter
// width/10. Changing the one dimension, in its label or here with
// Doc.Dimension, resizes the rest. The script checks the volume after
// each width, prints it, and saves dimensions.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	width, write := 60.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			width = v
		}
	}
	doc := api.Current()
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(60, 40, 0))
	doc.Constrain.Coincident(sides[0].Start(), doc.Origin())
	doc.Constrain.Length(sides[0], 60).SetName("width")
	must(doc.Constrain.Length(sides[1], 40).SetExpr("width*2/3"))
	hole := doc.Sketch.Circle(api.V(30, 20, 0), 3)
	must(doc.Constrain.Diameter(hole, 6).SetExpr("width/10"))
	doc.Constrain.Distance(hole.Center(), sides[0], 20)
	doc.Constrain.Distance(hole.Center(), sides[3], 30)
	plate := doc.Features.Extrude(5, "plate")

	for _, w := range []float64{60, width} {
		must(doc.Dimension("width").SetExpr(strconv.FormatFloat(w, 'g', -1, 64)))
		r := plate.Analyze().Check()
		want := (w*w*2/3 - math.Pi*(w/20)*(w/20)) * 5
		fmt.Printf("width %g: %s, %.1f mm³ (%.1f by the sizes)\n",
			w, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
		// The hole is drawn as chords, which cut inside it.
		if !r.Sound() || math.Abs(r.Volume-want) > 0.01*want || len(doc.Params.Errors()) > 0 {
			fmt.Fprintln(os.Stderr, doc.Params.Errors())
			os.Exit(1)
		}
	}
	if write {
		must(doc.File.Save("dimensions.tlspr"))
	}
}
