// A plate drawn in a DXF file, extruded, and its section exported as a
// 2d file with the export options set, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/dxfsection/dxfsection.go
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/dxfsection/dxfsection.go
//
// The script writes a 60 x 30 rectangle as a DXF file, imports it into
// the sketch as File > Import does, extrudes it, sets the export scale
// and page as the configuration screen would, and writes the section of
// the plate by the plane half way up to dxfsection.svg, as File >
// Export > 2d Section does. It also rotates a linked part a quarter, as
// Edit > Rotate Imported 90° does.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	dir, err := os.MkdirTemp("", "dxfsection")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// Four lines, as a DXF file holds them.
	corners := [][2]string{{"0", "0"}, {"60", "0"}, {"60", "30"}, {"0", "30"}}
	var b strings.Builder
	b.WriteString("0\nSECTION\n2\nENTITIES\n")
	for i, c := range corners {
		n := corners[(i+1)%len(corners)]
		fmt.Fprintf(&b, "0\nLINE\n8\n0\n10\n%s\n20\n%s\n30\n0\n11\n%s\n21\n%s\n31\n0\n", c[0], c[1], n[0], n[1])
	}
	b.WriteString("0\nENDSEC\n0\nEOF\n")
	file := filepath.Join(dir, "plate.dxf")
	if err := os.WriteFile(file, []byte(b.String()), 0o644); err != nil {
		panic(err)
	}

	doc := api.Current()
	ignored, err := doc.File.ImportDXF(file)
	if err != nil {
		panic(err)
	}
	fmt.Printf("imported the plate, %d entities left out\n", ignored)
	plate := doc.Features.Extrude(10, "plate")

	// Half way up the plate, a workplane to cut by.
	doc.Features.Sketch3D("cut")
	mid := doc.Sketch.NewWorkplane(api.V(0, 0, 5), api.V(1, 0, 0), api.V(0, 1, 0))
	doc.Sketch.InWorkplane(mid)

	o := doc.File.ExportOptions()
	o.Scale = 1
	o.CanvasAuto = false
	o.Canvas.Width, o.Canvas.Height = 100, 60
	o.Canvas.Dx, o.Canvas.Dy = 20, 15
	if err := doc.File.SetExportOptions(o); err != nil {
		panic(err)
	}
	if err := doc.File.ExportSection("dxfsection.svg"); err != nil {
		panic(err)
	}
	fmt.Println("wrote dxfsection.svg, the section of", plate.Name())

	// A part linked from a file is placed by a move and a turn.
	part := filepath.Join(dir, "part.tlspr")
	if err := doc.File.Save(part); err != nil {
		panic(err)
	}
	link, err := doc.Assembly.Link(part, "part")
	if err != nil {
		panic(err)
	}
	link.AsLink().Move(api.V(80, 0, 0))
	link.AsLink().Rotate90(api.V(0, 0, 1))
	u, v := link.AsLink().Orientation()
	fmt.Printf("the linked part's x points along (%.0f %.0f %.0f), its y along (%.0f %.0f %.0f)\n",
		u.X, u.Y, u.Z, v.X, v.Y, v.Z)
}
