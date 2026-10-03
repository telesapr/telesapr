// A plate with a row of holes, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/bracket/bracket.go [width [holes]]
//
// In the application, on the sketch that is open: Script > Run File,
// or
//
//	telesapr --script examples/scripts/bracket/bracket.go
//
// It writes bracket.tlspr, bracket.stl, bracket.step and bracket.png
// where it is run, unless it is given "-" as a third argument.
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	width, holes, write := 60.0, 3, true
	if len(os.Args) > 1 {
		width, _ = strconv.ParseFloat(os.Args[1], 64)
	}
	if len(os.Args) > 2 {
		holes, _ = strconv.Atoi(os.Args[2])
	}
	if len(os.Args) > 3 && os.Args[3] == "-" {
		write = false
	}
	const height, thickness, diameter = 20.0, 4.0, 5.0

	// The outline: a rectangle with a corner at the origin, dimensioned.
	doc := api.Current()
	sides := doc.Rect(api.V(0, 0, 0), api.V(width, height, 0))
	doc.Coincident(sides[0].Start(), doc.Origin())
	doc.Length(sides[0], width)
	doc.Length(sides[1], height)

	// The holes, each placed from the bottom and the left sides.
	pitch := width / float64(holes+1)
	for i := 1; i <= holes; i++ {
		x := pitch * float64(i)
		hole := doc.Circle(api.V(x, height/2, 0), diameter/2)
		doc.Diameter(hole, diameter)
		doc.Distance(hole.Center(), sides[0], height/2)
		doc.Distance(hole.Center(), sides[3], x)
	}
	sketch := doc.Active()
	fmt.Printf("%s: solved %v, %d degrees of freedom\n", sketch, sketch.Solved(), sketch.Dof())

	plate := doc.Extrude(thickness, "plate")
	fmt.Printf("%s: %.1f mm³\n", plate, plate.Volume())

	if !write {
		return
	}
	if err := doc.Save("bracket.tlspr"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, name := range []string{"bracket.stl", "bracket.step", "bracket.png"} {
		if err := doc.Export(name); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
