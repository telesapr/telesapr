// A rail bent along a cubic and then at a sharp corner, and a twisted
// bar round a corner rounded off: fitted sweeps through corners.
//
//	telesapr --script examples/scripts/zigzag/zigzag.go
//
// The rail, a 4 by 3 rectangle, follows a cubic that leaves the origin
// upward and bends over, runs straight on for 8 mm, turns sharply and
// runs 10 mm more. Along the cubic its frames are fitted; at the corner
// between the two lines they don't turn, so the two straight runs are
// prisms mitred by the plane halving the corner, as a sweep along lines
// alone is, and the volume is the rectangle's area times the path's
// length. The bar, a 3 mm square twisted by 90 degrees along a path up
// 20 mm and over 20 mm, needs a corner radius: a twist turns the frames
// along a line, so a mitre's two sides wouldn't meet. With a radius of 4
// the corner is an arc the twist runs round. A second bar takes the same
// corner sharp: the twist is held still along each line beside it, over
// the stretch the mitre spans, and turns along the rest, so the corner
// is mitred as an untwisted one. The script checks the solids sound, and
// the bars' volumes.
package main

import (
	"fmt"
	"math"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.01)
	doc.Features.Sketch3D("rail path")
	c := []api.Vec{api.V(0, 0, 0), api.V(0, 0, 6), api.V(4, 0, 10), api.V(8, 0, 12)}
	a := c[3].Plus(c[3].Minus(c[2]).WithMagnitude(8))
	b := a.Plus(api.V(1, 0, -2).WithMagnitude(10))
	path := []api.Entity{doc.Sketch.Cubic(c[0], c[1], c[2], c[3]), doc.Sketch.Line(c[3], a), doc.Sketch.Line(a, b)}
	doc.Features.SketchIn(doc.Workplane("XY"), "rail")
	doc.Sketch.Rect(api.V(-2, -1.5, 0), api.V(2, 1.5, 0))
	rail := doc.Features.Sweep(path, "rail")
	if p := rail.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	check := rail.Analyze().Check()
	fmt.Printf("rail: %s\n", check.Summary())
	if !check.Sound() {
		os.Exit(1)
	}

	doc.Features.SketchIn(doc.Workplane("ZX"), "bar path")
	top := api.V(30, 0, 20)
	turn := []api.Entity{doc.Sketch.Line(api.V(30, 0, 0), top), doc.Sketch.Line(top, api.V(50, 0, 20))}
	doc.Features.SketchIn(doc.Workplane("XY"), "bar")
	doc.Sketch.Rect(api.V(28.5, -1.5, 0), api.V(31.5, 1.5, 0))
	bar := doc.Features.Sweep(turn, "bar")
	sw := bar.AsSweep()
	sw.SetCornerRadius(4)
	sw.SetTwist(90)
	if p := bar.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	both := bar.Analyze().Check()
	// The bar is added to the rail: its volume is what the solid gained.
	// Its path is two lines cut back by 4 and a quarter arc of radius 4;
	// the twisted sides' mesh takes a little more than the true sweep.
	got, want := both.Volume-check.Volume, 9*(40-8+4*math.Pi/2)
	fmt.Printf("bar: volume %.3f mm³, %.3f the area times the path; %s\n", got, want, both.Summary())
	if !both.Sound() || math.Abs(got-want) > 0.02*want {
		os.Exit(1)
	}

	doc.Features.SketchIn(doc.Workplane("ZX"), "sharp bar path")
	top = api.V(70, 0, 20)
	sharp := []api.Entity{doc.Sketch.Line(api.V(70, 0, 0), top), doc.Sketch.Line(top, api.V(90, 0, 20))}
	doc.Features.SketchIn(doc.Workplane("XY"), "sharp bar")
	doc.Sketch.Rect(api.V(68.5, -1.5, 0), api.V(71.5, 1.5, 0))
	held := doc.Features.Sweep(sharp, "sharp bar")
	held.AsSweep().SetTwist(90)
	if p := held.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	all := held.Analyze().Check()
	got, want = all.Volume-both.Volume, 9*40.0
	fmt.Printf("sharp bar: volume %.3f mm³, %.3f the area times the path; %s\n", got, want, all.Summary())
	if !all.Sound() || math.Abs(got-want) > 0.02*want {
		os.Exit(1)
	}
}
