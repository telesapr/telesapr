// A bracket and its drawing: the standard views on an A3 sheet with a
// title block, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/drawing/drawing.go [-]
//	telesapr-cli export-drawing -o drawing.pdf drawing.tlspr
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/drawing/drawing.go
//
// A drawing is part of the model: a sheet, a scale (fitted unless
// given), the projection, and a title block. Here an L-shaped bracket
// with a hole in each leg gets one, third angle, fitted to A3. The
// script saves drawing.tlspr, unless given "-"; in the application it
// also writes drawing.pdf (File > Export Drawing does the same), and
// without one telesapr-cli export-drawing writes it from the file.
package main

import (
	"fmt"
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
	// The L in the XZ plane, extruded 40 along y.
	doc.SketchIn(doc.Workplane("ZX"), "profile")
	pts := []api.Vec{api.V(0, 0, 0), api.V(80, 0, 0), api.V(80, 0, 8), api.V(8, 0, 8), api.V(8, 0, 60), api.V(0, 0, 60)}
	for i := range pts {
		doc.Line(pts[i], pts[(i+1)%len(pts)])
	}
	doc.Extrude(40, "bracket")
	// A hole through the foot.
	doc.SketchIn(doc.Workplane("XY"), "hole")
	doc.Circle(api.V(50, 20, 0), 5)
	hole := doc.Extrude(8, "foot-hole")
	hole.SetCombine(api.Difference)

	d := doc.Drawing()
	d.Sheet, d.Title, d.Number, d.Material, d.Author = "A3", "BRACKET", "TS-0001", "steel", "TeleSAPR"
	doc.SetDrawing(d)
	r := hole.Check()
	fmt.Printf("a bracket of %.0f mm³, drawn on %s, %s\n", r.Volume, d.Sheet, map[bool]string{true: "first angle", false: "third angle"}[d.FirstAngle])
	if !r.Sound() {
		os.Exit(1)
	}
	if !write {
		return
	}
	must(doc.Save("drawing.tlspr"))
	if err := doc.ExportDrawing("drawing.pdf"); err != nil {
		fmt.Println("no window to draw in; write the sheet with: telesapr-cli export-drawing -o drawing.pdf drawing.tlspr")
	}
}
