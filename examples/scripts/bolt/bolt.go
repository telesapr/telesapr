// A hexagon head bolt with a screw thread, made by a script.
//
// Without a window:
//
//	spacesolve-cli run examples/scripts/bolt/bolt.go [designation [length]]
//
// In the application: Script > Run File, or
//
//	spacesolve --script examples/scripts/bolt/bolt.go
//
// It writes bolt.spcs and bolt.stl where it is run, unless it is given
// "-" as a third argument. The designation is any the screw thread
// screen takes, M8 by default; the length is the shank's, in mm.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/spacesolve/spacesolve/pkg/script/api"
)

func main() {
	designation, length, write := "M8", 30.0, true
	if len(os.Args) > 1 {
		designation = os.Args[1]
	}
	if len(os.Args) > 2 {
		length, _ = strconv.ParseFloat(os.Args[2], 64)
	}
	if len(os.Args) > 3 && os.Args[3] == "-" {
		write = false
	}

	doc := api.Current()
	// The shank, its diameter from the thread, standing on the origin.
	d, err := diameter(designation)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	shank := doc.Circle(api.V(0, 0, 0), d/2)
	doc.Coincident(shank.Center(), doc.Origin())
	doc.Diameter(shank, d)
	rod := doc.Extrude(length, "shank")

	// The head, a hexagon 1.6 diameters across the flats and 0.7 of
	// one high, under the shank.
	doc.SketchIn(doc.Workplane("XY"), "head-sketch")
	across, height := 1.6*d, 0.7*d
	var corners []api.Vec
	for i := 0; i < 6; i++ { // the interpreter has no range over an int
		a := math.Pi / 6 * float64(2*i+1)
		r := across / 2 / math.Cos(math.Pi/6)
		corners = append(corners, api.V(r*math.Cos(a), r*math.Sin(a), 0))
	}
	doc.Polyline(true, corners...)
	doc.Extrude(-height, "head")

	// The thread, from the shank's free end towards the head.
	var end api.Entity
	for _, e := range rod.Entities() {
		if e.Kind() == "circle" && e.Center().Pos().Z > length/2 {
			end = e
		}
	}
	thread := doc.Thread(end, api.ThreadOptions{
		Designation: designation,
		Length:      0.6 * length,
		Reverse:     true,
		Through:     true,
	})

	r := thread.Check()
	fmt.Printf("%s: %d faces, %.1f mm³, sound %v\n", thread, r.Faces, r.Volume, r.Sound())
	if !r.Sound() {
		fmt.Fprintln(os.Stderr, r.Summary())
		os.Exit(1)
	}
	if !write {
		return
	}
	if err := doc.Save("bolt.spcs"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// The mesh is made again at the export's coarser chord tolerance,
	// where the kernel can still leave a thread with naked edges that
	// the model doesn't have; the export then says so, and it is a
	// warning here.
	if err := doc.Export("bolt.stl"); err != nil {
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
}

// diameter is the major diameter of a thread: the number after M, or a
// fraction of an inch before the dash.
func diameter(designation string) (float64, error) {
	var d float64
	if _, err := fmt.Sscanf(designation, "M%g", &d); err == nil {
		return d, nil
	}
	var num, den float64
	if _, err := fmt.Sscanf(designation, "%g/%g", &num, &den); err == nil && den != 0 {
		return api.Inch(num / den), nil
	}
	return 0, fmt.Errorf("can't tell the diameter of %q", designation)
}
