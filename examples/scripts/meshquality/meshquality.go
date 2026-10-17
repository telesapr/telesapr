// A rounded block exported as a fine, strictly checked mesh.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/meshquality/meshquality.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/meshquality/meshquality.go
//
// The mesh export follows three settings, a chord tolerance, an angular
// tolerance and a limit on the pieces of a curve; File.SetMeshQuality
// sets all three from a preset (MeshDraft, MeshStandard, MeshFine). Here
// the fine preset is used, and StrictMesh makes the export refuse a mesh
// with naked edges or self-intersections instead of writing it with a
// warning. The script writes meshquality.stl, unless given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()
	doc.Sketch.Circle(api.V(0, 0, 0), 12)
	doc.Features.Extrude(10, "cylinder")

	if err := doc.File.SetMeshQuality(api.MeshFine); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	o := doc.File.ExportOptions()
	o.StrictMesh = true
	if err := doc.File.SetExportOptions(o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	o = doc.File.ExportOptions()
	fmt.Printf("quality %v: chord %g mm, angle %g degrees, %d segments, strict %v\n",
		o.Quality(), o.ChordTol, o.AngleTol, o.MaxSegments, o.StrictMesh)

	if !write {
		return
	}
	if err := doc.File.Export("meshquality.stl"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
