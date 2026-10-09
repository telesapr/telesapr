// An assembly's bill of materials, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/bom/bom.go [bolts] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/bom/bom.go
//
// The script makes two parts, a plate and a bolt, saves each as a file
// in a folder "parts" where it runs, and links them into a bracket, saved
// there too: the plate once, the bolt once and then repeated along the
// plate, four in all unless given. The model links the bracket and
// repeats it, two brackets; the parts a linked file links count too.
// Assembly > Bill of Materials lists what the model places, by file; here
// Doc.BillOfMaterials gives it and Doc.ExportBOM writes bom.csv, unless
// given "-". The script checks the counts.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// part saves a block of w x d x h as parts/name.tlspr.
func part(name string, w, d, h float64) string {
	doc := api.New()
	doc.Rect(api.V(0, 0, 0), api.V(w, d, 0))
	doc.Extrude(h, name)
	path := filepath.Join("parts", name+".tlspr")
	must(doc.Save(path))
	return path
}

func main() {
	bolts, write := 4, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if n, err := strconv.Atoi(a); err == nil {
			bolts = n
		}
	}
	must(os.MkdirAll("parts", 0o755))
	plate := part("plate", 100, 20, 5)
	bolt := part("bolt", 4, 4, 20)

	bracket := api.New()
	_, err := bracket.Link(plate, "")
	must(err)
	_, err = bracket.Link(bolt, "")
	must(err)
	bracket.Translate(bolts, api.V(20, 0, 0), "bolts")
	// The bolt stands in the plate, as one through a hole would; the
	// check by shells says by how much each part overlaps another.
	found, _ := bracket.Interferences()
	for _, f := range found {
		fmt.Printf("%s and %s share %.1f mm³\n", f.A, f.B, f.Volume)
	}
	if len(found) == 0 {
		fmt.Fprintln(os.Stderr, "the bolt doesn't meet the plate")
		os.Exit(1)
	}
	must(bracket.Save(filepath.Join("parts", "bracket.tlspr")))

	doc := api.Current()
	_, err = doc.Link(filepath.Join("parts", "bracket.tlspr"), "")
	must(err)
	doc.Translate(2, api.V(0, 50, 0), "brackets")

	items := doc.BillOfMaterials()
	ok := len(items) == 3
	for _, it := range items {
		fmt.Printf("%3d  %s\n", it.Count, it.Part)
		switch it.Part {
		case "bracket", "plate":
			ok = ok && it.Count == 2
		case "bolt":
			ok = ok && it.Count == 2*bolts
		}
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "the counts are wrong")
		os.Exit(1)
	}
	if write {
		must(doc.ExportBOM("bom.csv"))
	}
}
