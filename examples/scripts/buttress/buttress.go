// Buttress threads, which carry their load on one flank: a screw jack's
// S20x4 spindle (DIN 513) threaded down from its top, its nut with the
// load flank turned back to mate it, and a 1-12 BUTT (ASME B1.9) ring,
// made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/buttress/buttress.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/buttress/buttress.go
//
// A buttress ridge leans: its load flank, nearly square to the axis,
// faces along the thread on a screw and back in a nut. The spindle's
// thread runs down from its top and the nut's up from its bottom, so the
// nut takes LoadFlankBack for the two load flanks to face each other,
// as Assembly.Screw requires of buttress threads it fits. The script
// checks the solids, prints them, and saves buttress.tlspr unless given
// "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// check stops the script unless the group's solid is sound.
func check(g api.Group) {
	r := g.Analyze().Check()
	fmt.Printf("%s: %.2f mm³, sound %v, %d dof\n", g, r.Volume, r.Sound(), g.Dof())
	if !r.Sound() || g.Dof() != 0 {
		fmt.Fprintf(os.Stderr, "%s: want a sound solid and 0 dof\n", g)
		os.Exit(1)
	}
}

// last is the last of a thread's groups, which holds its solid.
func last(t api.ThreadGroup) api.Group {
	gs := t.Groups()
	return gs[len(gs)-1]
}

// top is the circle of g's at its top, more than z up.
func top(g api.Group, z float64) api.Entity {
	var out api.Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > z {
			out = e
		}
	}
	return out
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()
	fmt.Println("S20 pitches:", api.ThreadPitches("S20"))

	// The spindle, threaded down from its top for 30 mm.
	rod := doc.Sketch.Circle(api.V(0, 0, 0), 10)
	doc.Constrain.Coincident(rod.Center(), doc.Origin())
	doc.Constrain.Diameter(rod, 20)
	spindle := doc.Features.Extrude(50, "spindle")
	screw := doc.Features.Thread(top(spindle, 25), api.ThreadOptions{Designation: "S20x4", Length: 30, Reverse: true, Clearance: 0.1}).AsThread()
	check(last(screw))

	// Its nut, threaded up from its bottom: the load flank turned back.
	doc.Features.SketchIn(doc.Workplane("XY"), "nut")
	body := doc.Sketch.Circle(api.V(50, 0, 0), 20)
	doc.Constrain.Diameter(body, 40)
	hole := doc.Sketch.Circle(api.V(50, 0, 0), 10)
	doc.Constrain.Diameter(hole, 20)
	doc.Constrain.Coincident(hole.Center(), body.Center())
	doc.Features.Extrude(20, "nut")
	nut := doc.Features.Thread(hole, api.ThreadOptions{Designation: "S20x4", Internal: true, Through: true, Length: 20,
		Clearance: 0.1, LoadFlankBack: true}).AsThread()
	o := nut.Options()
	fmt.Printf("nut %s, load flank back %v\n", o.Designation, o.LoadFlankBack)
	check(last(nut))

	// An inch buttress thread on a ring, its threads per inch from the
	// series.
	doc.Features.SketchIn(doc.Workplane("XY"), "ring")
	ring := doc.Sketch.Circle(api.V(-50, 0, 0), 12.7)
	doc.Constrain.Diameter(ring, 25.4)
	doc.Features.Extrude(30, "ring")
	inch := doc.Features.Thread(ring, api.ThreadOptions{Designation: api.ThreadDesignation("1 butt"), Length: 20}).AsThread()
	fmt.Println("ring", inch.Options().Designation)
	check(last(inch))

	if write {
		if err := doc.File.Save("buttress.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
