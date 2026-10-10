// An enclosure: a block hollowed to thin walls, open at the top, made
// by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/shell/shell.go [thickness] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/shell/shell.go
//
// Shell Solid hollows the solid so far to walls of one thickness, open
// where faces are taken away: here a 60 x 40 x 25 block, open at its
// top, walls 2 mm thick unless given. The top face is found by a point
// on it. The script checks the solid against the volume of the walls,
// prints it, and saves shell.tlspr, unless given "-".
package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	thick, write := 2.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			thick = v
		}
	}
	const w, d, h = 60.0, 40.0, 25.0
	doc := api.Current()
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(w, d, 0))
	block := doc.Features.Extrude(h, "block")

	top := block.Solid().FaceAt(api.V(w/2, d/2, h))
	box := doc.Features.ShellSolid([]api.SolidFace{top}, thick, "enclosure")
	if p := box.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := box.Analyze().Check()
	want := w*d*h - (w-2*thick)*(d-2*thick)*(h-thick)
	fmt.Printf("a %g x %g x %g block hollowed to walls %g mm thick: %s, %.1f mm³ (%.1f by the formula)\n",
		w, d, h, thick, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || r.Volume < want*(1-1e-9) || r.Volume > want*(1+1e-9) {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("shell.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
