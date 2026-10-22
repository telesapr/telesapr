// A plate with a hole, its width a parameter, written out as the script
// that builds it again: what File > Export > Script writes of a model
// edited in the window.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/exportscript/exportscript.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/exportscript/exportscript.go
//
// Doc.Script walks the groups in the order the model is built and
// writes each as the call that makes one of its kind, the requests of a
// sketch where they are solved to and the constraints with their values
// and expressions, so the program it gives builds the same model on a
// new drawing. The script prints how many lines the program has and the
// first group's call, and writes plate_model.go, unless given "-".
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()
	if err := doc.Params.Set("width", "40"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	lines := doc.Sketch.Rect(api.V(0, 0, 0), api.V(40, 25, 0))
	if err := doc.Constrain.Length(lines[0], 40).SetExpr("width"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	doc.Sketch.Circle(api.V(20, 12.5, 0), 5)
	doc.Features.Extrude(6, "plate")

	src, notes := doc.Script()
	if len(notes) > 0 {
		fmt.Fprintln(os.Stderr, "the script leaves out:", notes)
		os.Exit(2)
	}
	for _, l := range strings.Split(src, "\n") {
		if strings.Contains(l, "doc.Features.Extrude") {
			fmt.Println(strings.TrimSpace(l))
		}
	}
	fmt.Printf("%d lines\n", strings.Count(src, "\n"))
	if !write {
		return
	}
	if _, err := doc.File.ExportScript("plate_model.go"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
}
