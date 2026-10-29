// A screw whose thread fades out at its head, and a triple-start lead
// screw whose starts all begin at its end, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/threadrunout/threadrunout.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/threadrunout/threadrunout.go
//
// RunOut is how far before the shoulder the thread fades: the turned
// surface leaves the root and tapers out to the crest, so the ridge
// sinks into the part as a die leaves it. LevelStarts gives each start
// of a thread of several its own ridge and helix from the circle's
// plane, rather than one helix sweeping the ridges stacked a pitch
// apart. Both are edited later as the rest of the thread is, and a
// Phase, which turns the thread about its axis, goes with either. The
// script checks the solids, prints them, and saves threadrunout.tlspr
// unless given "-".
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

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()

	// An M8 screw, its thread fading out over 3 mm below the head.
	rod := doc.Sketch.Circle(api.V(0, 0, 0), 4)
	doc.Constrain.Coincident(rod.Center(), doc.Origin())
	doc.Constrain.Diameter(rod, 8)
	doc.Features.Extrude(30, "screw")
	screw := doc.Features.Thread(rod, api.ThreadOptions{Designation: "M8", Length: 12, RunOut: 3}).AsThread()
	check(screw.Finished())
	o := screw.Options()
	o.RunOut = 2
	screw.Edit(o)
	fmt.Printf("screw %s, run-out %.1f mm\n", screw.Options().Designation, screw.Options().RunOut)
	check(screw.Finished())
	// Turned a quarter turn about its axis: the ridges move along by a
	// quarter of the pitch, and still fade out into the run-out.
	o = screw.Options()
	o.Phase = 90
	screw.Edit(o)
	fmt.Printf("screw phased %.0f°\n", screw.Options().Phase)
	check(screw.Finished())

	// A triple-start lead screw beside it: the three starts from its
	// foot, each a ridge sketch and a helix of its own.
	doc.Features.SketchIn(doc.Workplane("XY"), "lead screw")
	lead := doc.Sketch.Circle(api.V(30, 0, 0), 6)
	doc.Constrain.Diameter(lead, 12)
	doc.Features.Extrude(20, "lead")
	triple := doc.Features.Thread(lead, api.ThreadOptions{Designation: "M12xPh3P1", Length: 14, LevelStarts: true}).AsThread()
	fmt.Printf("lead screw %s: %d groups\n", triple.Options().Designation, len(triple.Groups()))
	if len(triple.Groups()) != 8 {
		fmt.Fprintln(os.Stderr, "want a ridge sketch and a helix per start")
		os.Exit(1)
	}
	check(triple.Finished())

	if write {
		if err := doc.File.Save("threadrunout.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
