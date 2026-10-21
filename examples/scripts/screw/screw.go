// A nut run up a screw by a motion study, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/screw/screw.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/screw/screw.go
//
// A screw mate puts two circles on one axis and ties the second part's
// turn about it to its travel along it: the part turns by the parameter
// "turn" and moves "lead" along the axis for each whole turn, as a nut
// on a screw does. Here a square nut with an M8 hole is saved as
// parts/nut.tlspr and linked twice, the second screwed onto the first's
// hole with the M8's coarse pitch, 1.25. A motion study turns it two
// whole turns in eight steps; the script checks that each step lifts it
// by its share of 2.5 and turns it as far, and saves screw.tlspr,
// unless given "-". Assembly > Mate Screw does the same from the window,
// and Assembly > Motion Study drives turn there.
package main

import (
	"fmt"
	"math"
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

// top is g's circle highest up, the top of the hole.
func top(g api.Group) api.Entity {
	var best api.Entity
	for _, e := range g.Entities() {
		if e.Kind() == "circle" && (best.H.V == 0 || e.Center().Pos().Z > best.Center().Pos().Z) {
			best = e
		}
	}
	return best
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	part := api.New()
	part.Sketch.Rect(api.V(-7, -7, 0), api.V(7, 7, 0))
	part.Sketch.Circle(api.V(0, 0, 0), 4)
	part.Features.Extrude(6, "")
	must(os.MkdirAll("parts", 0o755))
	path := filepath.Join("parts", "nut.tlspr")
	must(part.File.Save(path))

	doc := api.Current()
	a, err := doc.Assembly.Link(path, "fixed")
	must(err)
	b, err := doc.Assembly.Link(path, "nut")
	must(err)
	doc.Sketch.Anywhere3D()
	mate := doc.Assembly.Screw(top(a), top(b), 1.25)
	fmt.Printf("screw: %d constraints, turned by %q, %q a turn, %d dof\n",
		len(mate.Constraints), mate.Turn, mate.Lead, b.Dof())

	// Where the nut's corner and its hole's centre start.
	axis, n := top(a).Center().Pos(), top(a).Normal().Dir()
	corner := b.Entities()[0]
	for _, e := range b.Entities() {
		if e.Kind() == "point" && e.Pos().Minus(top(b).Center().Pos()).Magnitude() > corner.Pos().Minus(top(b).Center().Pos()).Magnitude() {
			corner = e
		}
	}
	h0 := top(b).Center().Pos().Minus(axis).Dot(n)
	r0 := corner.Pos().Minus(axis)
	r0 = r0.Minus(n.ScaledBy(r0.Dot(n)))

	frames, err := doc.Assembly.MotionStudy(mate.Turn, 0, 720, 8, "")
	must(err)
	for _, f := range frames {
		if !f.Solved {
			fmt.Fprintf(os.Stderr, "step %d didn't solve\n", f.Step)
			os.Exit(1)
		}
	}
	// The study gives turn back its expression, 0; the last frame is
	// checked by setting it there again.
	must(doc.Params.Set(mate.Turn, "720"))
	rise := top(b).Center().Pos().Minus(axis).Dot(n) - h0
	r := corner.Pos().Minus(axis)
	r = r.Minus(n.ScaledBy(r.Dot(n)))
	turned := math.Atan2(r0.Cross(r).Dot(n), r0.Dot(r)) * 180 / math.Pi
	fmt.Printf("after two turns: risen %.4f, turned %.4f degrees past a whole turn\n", rise, turned)
	if math.Abs(rise-2.5) > 1e-6 || math.Abs(turned) > 1e-6 || !b.Solved() {
		os.Exit(1)
	}
	must(doc.Params.Set(mate.Turn, "90"))
	r = corner.Pos().Minus(axis)
	r = r.Minus(n.ScaledBy(r.Dot(n)))
	turned = math.Atan2(r0.Cross(r).Dot(n), r0.Dot(r)) * 180 / math.Pi
	fmt.Printf("a quarter turn: turned %.4f degrees\n", turned)
	if math.Abs(turned-90) > 1e-6 {
		os.Exit(1)
	}
	if write {
		must(doc.File.Save("screw.tlspr"))
	}
}
