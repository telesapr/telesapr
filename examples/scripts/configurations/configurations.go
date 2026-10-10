// One plate in three sizes, by configurations, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/configurations/configurations.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/configurations/configurations.go
//
// A configuration is a named set of the parameters' expressions; one
// may be active, and the table holds its values (Edit > Parameters
// lists them). Here a plate's sides come from the parameter w, and
// three configurations fill it: small,
// medium and large. Each is made active in turn and the plate checked
// against its volume; the file is saved with "medium" active, as
// configurations.tlspr, unless given "-".
package main

import (
	"fmt"
	"math"
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
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()
	must(doc.Params.Set("w", "40"))
	sides := doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 30, 0))
	must(doc.Constrain.Length(sides[0], 40).SetExpr("w"))
	must(doc.Constrain.Length(sides[1], 30).SetExpr("w*3/4"))
	plate := doc.Features.Extrude(4, "plate")

	sizes := map[string]float64{"small": 40, "medium": 60, "large": 100}
	for _, name := range []string{"small", "medium", "large"} {
		must(doc.Params.AddConfiguration(name))
		must(doc.Params.Set("w", fmt.Sprint(sizes[name])))
	}
	for _, name := range []string{"small", "large", "medium"} {
		must(doc.Params.ActivateConfiguration(name))
		w := sizes[name]
		r := plate.Analyze().Check()
		want := w * w * 3 / 4 * 4
		fmt.Printf("%-6s w = %3g: %.0f mm³ (%.0f by the sizes)\n", name, w, r.Volume, want)
		if !r.Sound() || math.Abs(r.Volume-want) > 1e-6*want {
			os.Exit(1)
		}
	}
	if write {
		must(doc.File.Save("configurations.tlspr"))
	}
}
