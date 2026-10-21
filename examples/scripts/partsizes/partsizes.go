// One part file in several sizes in an assembly, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/partsizes/partsizes.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/partsizes/partsizes.go
//
// A link may pass the model it links a configuration and values for its
// parameters; the model is then regenerated with them rather than read
// as it was saved. The script saves a plate, its length the parameter L,
// with configurations short and long, as parts/plate.tlspr where it
// runs. It links the plate three times: as saved, in its long
// configuration, and with L passed from the assembly's own parameter W,
// which it then changes. The group screen of a link offers the same, and
// [relink] points a link at another file, as LinkGroup.SetFile does
// here for a fourth link. The script checks each part's volume, and
// saves the assembly as partsizes.tlspr unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// plate saves a plate L x 30 x 10, L 40 or 80 by configuration, and a
// block of 10 mm, in parts.
func plate() (string, string) {
	p := api.New()
	must(p.Params.Set("L", "40"))
	sides := p.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	must(p.Constrain.Length(sides[0], 40).SetExpr("L"))
	p.Constrain.Length(sides[1], 30)
	p.Features.Extrude(10, "plate")
	must(p.Params.AddConfiguration("short"))
	must(p.Params.AddConfiguration("long"))
	must(p.Params.Set("L", "80"))
	must(p.Params.ActivateConfiguration("short"))
	path := filepath.Join("parts", "plate.tlspr")
	must(p.File.Save(path))

	b := api.New()
	b.Sketch.Rect(api.V(0, 0, 0), api.V(10, 10, 0))
	b.Features.Extrude(10, "block")
	block := filepath.Join("parts", "block.tlspr")
	must(b.File.Save(block))
	return path, block
}

// check fails unless the parts have these volumes.
func check(doc *api.Doc, want ...float64) {
	groups := doc.Groups()
	parts := groups[len(groups)-1].Analyze().MassProperties().Parts
	for i, p := range parts {
		fmt.Printf("  part %d: %.0f mm³\n", i+1, p.Volume)
		if i >= len(want) || math.Abs(p.Volume-want[i]) > 1e-6*want[i] {
			fmt.Fprintf(os.Stderr, "part %d: %g mm³, want %v\n", i, p.Volume, want)
			os.Exit(1)
		}
	}
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	must(os.MkdirAll("parts", 0o755))
	path, block := plate()

	doc := api.Current()
	must(doc.Params.Set("W", "50"))
	_, err := doc.Assembly.Link(path, "saved")
	must(err)
	long, err := doc.Assembly.Link(path, "long")
	must(err)
	long.AsLink().SetConfiguration("long")
	driven, err := doc.Assembly.Link(path, "driven")
	must(err)
	driven.AsLink().SetParam("L", "W*2")
	fmt.Println("W = 50:")
	check(doc, 12000, 24000, 30000)

	must(doc.Params.Set("W", "30"))
	fmt.Println("W = 30:")
	check(doc, 12000, 24000, 18000)

	swapped, err := doc.Assembly.Link(path, "swapped")
	must(err)
	swapped.AsLink().SetFile(block)
	fmt.Println("the fourth relinked to the block:")
	check(doc, 12000, 24000, 18000, 1000)
	if write {
		must(doc.File.Save("partsizes.tlspr"))
	}
}
