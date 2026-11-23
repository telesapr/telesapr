//go:build ignore

// The TeleSAPR half of scripts/step-exchange.sh: run on a model, it
// exports the model's solid as STEP to the file given and prints what
// the receiving programs are compared against, measured here:
//
//	telesapr-cli run model.tlspr scripts/step-exchange/export.go out.step
//
// The volume and centre are the solid's mesh's at a chord tolerance of
// 0.01%, so they lie within the chords' sag of the exact ones.
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: export.go out.step")
		os.Exit(1)
	}
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.01)
	a := doc.Active().Analyze()
	vol := a.Volume()
	c := a.CenterOfMass()
	lo, hi := a.Bounds()
	if err := doc.File.Export(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("ours\t%.6f\t%.6f\t%.6f\t%.6f\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f\t%.4f\n",
		vol, c.X, c.Y, c.Z, lo.X, lo.Y, lo.Z, hi.X, hi.Y, hi.Z)
}
