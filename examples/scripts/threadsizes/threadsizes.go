// Threads picked from the standard series rather than typed: a fine
// metric screw, a left-hand ACME lead screw and a G1/2 pipe nipple's
// bore, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/threadsizes/threadsizes.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/threadsizes/threadsizes.go
//
// ThreadSeries names the series the screw thread screen's [choose]
// lists; ThreadSizes gives a series' sizes, and ThreadPitches the
// standard pitches of a size, coarsest first. LeftHand is the screen's
// left-hand choice. The script checks the solids, prints them, and
// saves threadsizes.tlspr unless given "-".
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

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()
	fmt.Println("series:", api.ThreadSeries())

	// M10's finest standard pitch.
	pitches := api.ThreadPitches("M10")
	fmt.Println("M10 pitches:", pitches)
	rod := doc.Sketch.Circle(api.V(0, 0, 0), 5)
	doc.Constrain.Coincident(rod.Center(), doc.Origin())
	doc.Constrain.Diameter(rod, 10)
	doc.Features.Extrude(25, "screw")
	fine := doc.Features.Thread(rod, api.ThreadOptions{Designation: pitches[len(pitches)-1], Length: 15}).AsThread()
	check(last(fine))

	// The ACME size nearest 1/2 inch, left-handed.
	doc.Features.SketchIn(doc.Workplane("XY"), "lead screw")
	lead := doc.Sketch.Circle(api.V(30, 0, 0), 6.35)
	doc.Constrain.Diameter(lead, 12.7)
	doc.Features.Extrude(30, "lead")
	var acme string
	for _, size := range api.ThreadSizes("ACME") {
		if size == "1/2-10 ACME" {
			acme = size
		}
	}
	screw := doc.Features.Thread(lead, api.ThreadOptions{Designation: acme, LeftHand: true, Length: 20}).AsThread()
	o := screw.Options()
	fmt.Printf("lead screw %s, left-hand %v\n", o.Designation, o.LeftHand)
	check(last(screw))

	// A G1/2 pipe thread bored through a ring.
	doc.Features.SketchIn(doc.Workplane("XY"), "nipple")
	boss := doc.Sketch.Circle(api.V(-40, 0, 0), 15)
	doc.Constrain.Diameter(boss, 30)
	hole := doc.Sketch.Circle(api.V(-40, 0, 0), 10)
	doc.Constrain.Diameter(hole, 20)
	doc.Constrain.Coincident(hole.Center(), boss.Center())
	doc.Features.Extrude(12, "nipple")
	pipe := doc.Features.Thread(hole, api.ThreadOptions{Designation: api.ThreadDesignation("g 1/2"), Internal: true, Through: true, Length: 12}).AsThread()
	check(last(pipe))

	if write {
		if err := doc.File.Save("threadsizes.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
