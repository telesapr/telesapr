// A shaft in a housing, each with its own appearance, and one face of
// its own: shading by appearance, set by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/appearance/appearance.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/appearance/appearance.go
//
// With "shade solids by their appearance" on in the configuration, the
// window lights each face by its appearance: matte, plastic (the
// default), glossy or metal, beyond its colour. A group has one, and a
// face may have its own. Here the housing is plastic with a matte top,
// the shaft metal. The script prints them, and saves appearance.tlspr,
// unless given "-"; in the application it also renders appearance.png.
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()
	doc.Rect(api.V(-20, -20, 0), api.V(20, 20, 0))
	housing := doc.Extrude(15, "housing")
	doc.SketchIn(doc.Workplane("XY"), "")
	doc.Circle(api.V(0, 0, 0), 6)
	shaft := doc.Extrude(40, "shaft")
	shaft.SetCombine(api.Assemble)

	housing.SetAppearance("plastic")
	shaft.SetAppearance("metal")
	top := housing.Solid().FaceAt(api.V(15, 15, 15))
	doc.SetFaceAppearance(top, "matte")
	fmt.Printf("appearances %v: the housing %s, its top %s, the shaft %s\n", api.Appearances(),
		housing.Appearance(), doc.FaceAppearance(top), shaft.Appearance())
	if doc.FaceAppearance(top) != "matte" || shaft.Appearance() != "metal" {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("appearance.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		_ = doc.Render("appearance.png", 640, 480, "isometric")
	}
}
