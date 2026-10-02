// A block with a stepped bore, looked at through the section view,
// made by a script.
//
// Without a window:
//
//	spacesolve-cli run examples/scripts/section/section.go [offset]
//
// In the application: Script > Run File, or
//
//	spacesolve --script examples/scripts/section/section.go
//
// The block is cut parallel to its side face, offset mm into it (20,
// through the bore's axis, unless given), as View > Section View does.
// The script prints the area of the cut and writes section.png, the
// view as Render draws it, unless it is given "-" as a second argument.
package main

import (
	"fmt"
	"os"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	offset, write := "20", true
	if len(os.Args) > 1 {
		offset = os.Args[1]
	}
	if len(os.Args) > 2 && os.Args[2] == "-" {
		write = false
	}

	doc := api.Current()
	// A 40 x 40 block, 30 high, with a bore of 16 mm through it.
	doc.Rect(api.V(0, 0, 0), api.V(40, 40, 0))
	doc.Circle(api.V(20, 20, 0), 8)
	block := doc.Extrude(30, "block")

	// Cut parallel to the face at y = 0. A face's normal points out of
	// the solid, so a negative offset goes into it.
	var side api.Entity
	for _, f := range block.Faces() {
		doc.SetSection(f, "0", false)
		if s := doc.Section(); s.Normal.Y < -0.99 {
			side = f
		}
	}
	// Keep the far side, below the cut as Render's isometric view sees
	// it, so that the cap faces the camera.
	doc.SetSection(side, "-("+offset+")", true)
	s := doc.Section()
	if s.Err != "" {
		fmt.Fprintln(os.Stderr, s.Err)
		os.Exit(2)
	}
	// Through the axis, the cut is the block's side less the bore:
	// 40*30 - 16*30 = 720 mm².
	fmt.Printf("cut at y = %.1f: %.1f mm² in %d loops\n", s.Point.Y, s.Area, s.Loops)

	if write {
		if err := doc.Render("section.png", 640, 480, "isometric"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
