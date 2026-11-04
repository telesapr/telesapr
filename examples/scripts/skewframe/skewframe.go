// A skew frame: a square bar swept round a closed path that leaves a
// plane and turns sharply at each corner, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/skewframe/skewframe.go [side]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/skewframe/skewframe.go
//
// The path is four lines in 3d, no two sides of it in one plane, its
// first side up the z axis through the origin. The bar's section, a
// square of the given side (2 unless given), is drawn in XY across that
// side. Round such a path the section would come back turned, so the
// sweep fits its surface along the path and takes the turn out between
// the corners, which are mitred as a planar frame's are. The script
// prints the volume, the square's area times the path's length.
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	side := 2.0
	if len(os.Args) > 1 {
		v, err := strconv.ParseFloat(os.Args[1], 64)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		side = v
	}

	doc := api.Current()
	doc.Features.Sketch3D("path")
	corners := []api.Vec{api.V(0, 0, -5), api.V(0, 0, 5), api.V(12, 3, 5), api.V(9, 10, -4)}
	var path []api.Entity
	length := 0.0
	for i := 0; i < len(corners); i++ {
		a, b := corners[i], corners[(i+1)%len(corners)]
		path = append(path, doc.Sketch.Line(a, b))
		length += b.Minus(a).Magnitude()
	}

	doc.Features.SketchIn(doc.Workplane("XY"), "bar")
	h := side / 2
	sq := []api.Vec{api.V(-h, -h, 0), api.V(h, -h, 0), api.V(h, h, 0), api.V(-h, h, 0)}
	for i := 0; i < 4; i++ {
		doc.Sketch.Line(sq[i], sq[(i+1)%4])
	}

	frame := doc.Features.Sweep(path, "skew frame")
	if msg := frame.Problem(); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
	r := frame.Analyze().Check()
	want := side * side * length
	fmt.Printf("skew frame: %.1f mm³ (about %.1f), sound %v\n", r.Volume, want, r.Sound())
	if !r.Sound() || math.Abs(r.Volume-want) > 0.01*want {
		os.Exit(1)
	}
}
