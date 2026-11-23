//go:build ignore

// The last step of scripts/step-exchange.sh's round trip: links a STEP
// file as a solid and prints its volume and centre, and what the link
// says of it:
//
//	telesapr-cli run scripts/step-exchange/link.go in.step
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: link.go in.step")
		os.Exit(1)
	}
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.01)
	g, err := doc.Assembly.LinkStep(os.Args[1], "back", true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	a := g.Analyze()
	vol := a.Volume()
	c := a.CenterOfMass()
	// yaegi: the link's problem is put in a variable before the Printf.
	lk := g.AsLink()
	problem := lk.Problem()
	fmt.Printf("back\t%.6f\t%.6f\t%.6f\t%.6f\t%s\n", vol, c.X, c.Y, c.Z, problem)
}
