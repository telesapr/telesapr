// A stud with a chamfered end and a nut countersunk at both ends, made
// by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/threadchamfer/threadchamfer.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/threadchamfer/threadchamfer.go
//
// ThreadOptions.Chamfer cuts a thread's free end at 45°, from a little
// inside the minor diameter out past the crest, so that the thread
// starts clean rather than at a ridge's blunt end: a screw's end
// chamfered, a nut's mouth countersunk; a Through thread is chamfered
// at both ends. The chamfer is two more groups after the thread's
// last, a sketch and a lathe, which hold the finished part
// (ThreadGroup.Finished), edited with the thread as one. The script
// checks each solid, takes the stud's chamfer off and puts it back,
// prints the volumes and saves threadchamfer.tlspr unless given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// check stops the script unless the thread's finished solid is sound,
// and gives its volume.
func check(t api.ThreadGroup) float64 {
	g := t.Finished()
	r := g.Analyze().Check()
	fmt.Printf("%s: %d groups, %.2f mm³, sound %v, %d dof\n", g, len(t.Groups()), r.Volume, r.Sound(), g.Dof())
	if !r.Sound() || g.Dof() != 0 {
		fmt.Fprintf(os.Stderr, "%s: want a sound solid and 0 dof\n", g)
		os.Exit(1)
	}
	return r.Volume
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()

	// The stud, M10 and 30 long, threaded 20 from its lower end.
	rod := doc.Sketch.Circle(api.V(0, 0, 0), 5)
	doc.Constrain.Coincident(rod.Center(), doc.Origin())
	doc.Constrain.Diameter(rod, 10)
	doc.Features.Extrude(30, "stud")
	stud := doc.Features.Thread(rod, api.ThreadOptions{Designation: "M10", Length: 20, Chamfer: true}).AsThread()
	chamfered := check(stud)

	o := stud.Options()
	o.Chamfer = false
	stud = stud.Edit(o)
	blunt := check(stud)
	o.Chamfer = true
	check(stud.Edit(o))
	fmt.Printf("the chamfer takes %.2f mm³ off the stud's end\n", blunt-chamfered)
	if !(blunt > chamfered) {
		fmt.Fprintln(os.Stderr, "the chamfer took nothing off")
		os.Exit(1)
	}

	// The nut, beside it, through and so countersunk at both ends.
	doc.Features.SketchIn(doc.Workplane("XY"), "nut")
	hole := doc.Sketch.Circle(api.V(30, 0, 0), 10)
	doc.Features.Extrude(8, "nut")
	nut := doc.Features.Thread(hole, api.ThreadOptions{Designation: "M10", Length: 8, Internal: true, Through: true, Chamfer: true}).AsThread()
	check(nut)

	if write {
		if err := doc.File.Save("threadchamfer.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
