// A bolt and a nut whose threads fit, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/threadfit/threadfit.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/threadfit/threadfit.go
//
// An M10 bolt, of tolerance class 6g, and a nut, a Ø20 disc with an
// M10 6H thread in its hole, are saved as parts and linked into one
// model. The classes give the threads their clearances, the middle of
// each class's zone. A screw mate of the bolt's tip and the nut's hole,
// with a lead of 0, takes the threads' own lead and first moves the nut
// along the axis to where its ridges lie between the bolt's: wherever
// it is turned to then, nothing interferes, which the script checks at
// a few turns, down the bolt. It saves threadfit.tlspr unless given "-". From the
// window: the class's [choose] on the screw thread screen, and
// Assembly > Mate Screw on the two circles.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

// top is g's circle of the thread's radius, 5 mm, highest up.
func top(g api.Group) api.Entity {
	var best api.Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && e.Radius() > 5-1e-6 && e.Radius() < 5+1e-6 &&
			(best.H.V == 0 || e.Center().Pos().Z > best.Center().Pos().Z) {
			best = e
		}
	}
	return best
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	must(os.MkdirAll("parts", 0o755))

	bolt := api.New()
	shank := bolt.Sketch.Circle(api.V(0, 0, 0), 5)
	bolt.Constrain.Coincident(shank.Center(), bolt.Origin())
	rod := bolt.Features.Extrude(40, "shank")
	bolt.Features.Thread(top(rod), api.ThreadOptions{Designation: "M10", Length: 25, Reverse: true, Through: true, Class: "6g"})
	boltPath := filepath.Join("parts", "bolt.tlspr")
	must(bolt.File.Save(boltPath))

	nut := api.New()
	disc := nut.Sketch.Circle(api.V(0, 0, 0), 10)
	nut.Constrain.Coincident(disc.Center(), nut.Origin())
	hole := nut.Sketch.Circle(api.V(0, 0, 0), 5)
	nut.Constrain.Coincident(hole.Center(), nut.Origin())
	body := nut.Features.Extrude(8, "nut")
	nut.Features.Thread(top(body), api.ThreadOptions{Designation: "M10", Internal: true, Length: 8, Reverse: true, Through: true, Class: "6H"})
	nutPath := filepath.Join("parts", "nut.tlspr")
	must(nut.File.Save(nutPath))
	fmt.Printf("classes of an M10 screw: %v\n", api.ThreadClasses("M10", false))

	doc := api.Current()
	b, err := doc.Assembly.Link(boltPath, "bolt")
	must(err)
	n, err := doc.Assembly.Link(nutPath, "nut")
	must(err)
	// Onto the threaded end of the bolt, which the mate keeps it along.
	o := n.AsLink().Offset()
	n.AsLink().Move(api.V(-o.X, -o.Y, 28-o.Z))
	doc.Sketch.Anywhere3D()
	mate := doc.Assembly.Screw(top(b), top(n), 0)
	lead, _ := doc.Params.Get(mate.Lead)
	fmt.Printf("screw: lead %g mm a turn\n", lead.Value)
	for _, turn := range []string{"0", "-1800", "-2000"} {
		must(doc.Params.Set(mate.Turn, turn))
		found, _ := doc.Assembly.Interferences()
		fmt.Printf("turned %s degrees: %d interferences\n", turn, len(found))
		if len(found) != 0 {
			os.Exit(1)
		}
	}
	if write {
		must(doc.File.Save("threadfit.tlspr"))
	}
}
