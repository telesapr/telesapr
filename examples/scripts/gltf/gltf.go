// A block and a pin beside it, each of its own colour, exported as binary
// glTF for Blender or a three.js viewer.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/gltf/gltf.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/gltf/gltf.go
//
// File.Export picks the format from the extension: ".glb" writes the
// mesh of the active group as glTF 2.0, one material for each colour,
// a colour seen through (here the block, at half opacity) as a
// blended one, and lengths in metres, which is what glTF asks for. The
// script writes gltf.glb and prints its size and glTF header, unless
// given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()
	doc.Sketch.Rect(api.V(-20, -20, 0), api.V(20, 20, 0))
	block := doc.Features.Extrude(15, "block")
	block.SetColor(90, 140, 200)
	block.SetOpacity(0.5)
	doc.Features.SketchIn(doc.Workplane("XY"), "")
	doc.Sketch.Circle(api.V(35, 0, 0), 6)
	pin := doc.Features.Extrude(40, "pin")
	pin.SetCombine(api.Assemble)
	pin.SetColor(180, 180, 190)
	if !write {
		return
	}

	if err := doc.File.Export("gltf.glb"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, err := os.ReadFile("gltf.glb")
	if err != nil || len(b) < 12 || string(b[:4]) != "glTF" {
		fmt.Fprintln(os.Stderr, "gltf.glb is not binary glTF", err)
		os.Exit(2)
	}
	// The version is a little-endian 32-bit number after the magic.
	version := int(b[4]) | int(b[5])<<8 | int(b[6])<<16 | int(b[7])<<24
	fmt.Printf("gltf.glb: %d bytes, glTF version %d\n", len(b), version)
}
