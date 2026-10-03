// A bar of soap: a rectangle with rounded corners, extruded, its top
// edge rounded all round, made by a script.
//
// Without a window:
//
//	spacesolve-cli run examples/scripts/soap/soap.go [radius] [-]
//
// In the application: Script > Run File, or
//
//	spacesolve --script examples/scripts/soap/soap.go
//
// Fillet Edges goes on from the edges it is given into those they meet
// smoothly: here one side of the top, picked, takes the corners' arcs
// and the other sides with it, cylinders and tori meeting with no
// corner between. A 60 x 40 rectangle with corners of radius 10, 20
// high, its top rounded at 6 mm unless given. The script checks the
// solid against the volume the cross section takes along the path of
// its centroid, prints it, and saves soap.spcs, unless given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	radius, write := 6.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if r, err := strconv.ParseFloat(a, 64); err == nil {
			radius = r
		}
	}
	w, l, corner, h := 60.0, 40.0, 10.0, 20.0
	doc := api.Current()
	doc.SetChordTolerance(0.01)
	sides := doc.Rect(api.V(0, 0, 0), api.V(w, l, 0))
	for _, s := range sides {
		doc.TangentArc(s.Start(), corner, true)
	}
	bar := doc.Extrude(h, "bar")

	// One side of the top: the copy of the front side at the top.
	var front api.Entity
	for _, e := range bar.Entities() {
		if e.Kind() != "line" {
			continue
		}
		a, b := e.Start().Pos(), e.End().Pos()
		if a.Z > h/2 && b.Z > h/2 && math.Abs(a.Y) < 1e-6 && math.Abs(b.Y) < 1e-6 {
			front = e
		}
	}
	soap := doc.Fillet([]api.Entity{front}, radius, "rounded")
	if p := soap.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := soap.Check()
	// The fillet takes a square of the radius less a quarter circle,
	// along the sides and round the corners by its centroid,
	// (10 - 3 pi) / (12 - 3 pi) of the radius in.
	a := radius * radius * (1 - math.Pi/4)
	c := radius * (10 - 3*math.Pi) / (12 - 3*math.Pi)
	area := w*l - (4-math.Pi)*corner*corner
	want := area*h - a*(2*(w-2*corner)+2*(l-2*corner)+2*math.Pi*(corner-c))
	fmt.Printf("top rounded at %g mm: %s, %.1f mm³ (%.1f by formula)\n",
		radius, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || math.Abs(r.Volume-want) > 1e-3*want {
		os.Exit(1)
	}
	if write {
		if err := doc.Save("soap.spcs"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
