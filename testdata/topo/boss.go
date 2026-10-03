// Writes boss.spcs, the model of the tests of faces of several patches
// (ROADMAP-JUMP.md, step 11.1): a 40 x 30 x 10 block with a round boss
// of radius 8 and height 6 under it, centred at (20, 15), so that the
// tests' turned view, which looks from below, sees it. The boss's side
// is one face of four patches, and the circle at its foot, where the
// Boolean joined it to the block, is one edge of four pieces that no
// entity draws.
//
//	spacesolve-cli run testdata/topo/boss.go testdata/topo/boss.spcs
package main

import (
	"fmt"
	"os"

	. "github.com/spacesolve/spacesolve/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func main() {
	doc := Current()
	block := doc.Rect(V(0, 0, 0), V(40, 30, 0))
	doc.Coincident(block[0].Start(), doc.Origin())
	doc.Length(block[0], 40)
	doc.Length(block[1], 30)
	body := doc.Extrude(10, "block")

	// The boss is sketched on the block's top and extruded down through
	// the block, so its ends lie at z = 10 and z = -6 and no entity
	// lies along its foot at z = 0.
	var corner Entity
	for _, e := range body.Entities() {
		if e.Kind() == "point" && e.Pos().Equals(V(0, 0, 10)) {
			corner = e
		}
	}
	doc.SketchAt(corner, V(1, 0, 0), V(0, 1, 0), "boss-sketch")
	c := doc.Circle(V(20, 15, 10), 8)
	doc.Diameter(c, 16)
	doc.Distance(c.Center(), doc.Workplane("YZ"), 20)
	doc.Distance(c.Center(), doc.Workplane("ZX"), 15)
	boss := doc.Extrude(-16, "boss")
	if len(os.Args) > 1 {
		must(doc.Save(os.Args[1]))
	}
	fmt.Println(boss.Check().Summary())
}
