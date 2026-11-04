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
// is mitred as an untwisted one. A hook, up a line and sharply round an
// arc, is twisted too: the twist turns along the line past the corner's
// joint. A crook, up a line and sharply off along a cubic, is made of
// Booleans, which no fitted sweep mitres. A swan neck, two arcs about a
// sharp corner, is twisted with no line to twist along: each arc takes
// the twist past the corner's joint. The script checks the solids sound,
// and their volumes.
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

	// A hook, up a line and sharply round a quarter arc, twisted: the
	// line and the arc about the corner are swept as one rigid piece, the
	// twist held still there, but the line past the joint leaves it and
	// takes the twist. Untwisted first, for the volume to compare.
	doc.Features.SketchIn(doc.Workplane("ZX"), "hook path")
	hookPath := []api.Entity{
		doc.Sketch.Line(api.V(110, 0, 0), api.V(110, 0, 20)),
		doc.Sketch.Arc(api.V(110, 0, 25), api.V(110, 0, 20), api.V(115, 0, 25)),
	}
	doc.Features.SketchIn(doc.Workplane("XY"), "hook")
	doc.Sketch.Rect(api.V(109, -1.5, 0), api.V(111, 1.5, 0))
	hook := doc.Features.Sweep(hookPath, "hook")
	flat := hook.Analyze().Check()
	hook.AsSweep().SetTwist(90)
	if p := hook.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	twisted := hook.Analyze().Check()
	got, want = twisted.Volume-all.Volume, flat.Volume-all.Volume
	fmt.Printf("hook: volume %.3f mm³ twisted, %.3f untwisted; %s\n", got, want, twisted.Summary())
	if !twisted.Sound() || math.Abs(got-want) > 0.02*want {
		os.Exit(1)
	}

	// A hook too sharp to join: up a line and back down at 150° onto an
	// arc of radius 12. Some of the profile's points never meet their
	// paths along the line and the arc, so the corner is made of
	// Booleans: the line and the arc each swept, and the outside of the
	// turn filled by where they overlap carried on past it.
	doc.Features.SketchIn(doc.Workplane("ZX"), "sharp hook path")
	y := api.V(0, 1, 0)
	hookTop := api.V(150, 0, 20)
	dir := api.V(0, 0, 1).RotatedAroundAxis(y, 150*math.Pi/180)
	hc := hookTop.Plus(dir.RotatedAroundAxis(y, math.Pi/2).ScaledBy(12))
	sharpHook := []api.Entity{
		doc.Sketch.Line(api.V(150, 0, 0), hookTop),
		doc.Sketch.Arc(hc, hookTop, hc.Plus(hookTop.Minus(hc).RotatedAroundAxis(y, 100*math.Pi/180))),
	}
	doc.Features.SketchIn(doc.Workplane("XY"), "sharp hook")
	doc.Sketch.Rect(api.V(149, -1.5, 0), api.V(151, 1.5, 0))
	sh := doc.Features.Sweep(sharpHook, "sharp hook")
	if p := sh.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	last := sh.Analyze().Check()
	got = last.Volume - twisted.Volume
	fmt.Printf("sharp hook: volume %.3f mm³; %s\n", got, last.Summary())
	if !last.Sound() || math.Abs(got-210.747) > 0.01*210.747 {
		os.Exit(1)
	}

	// A crook: up a line and sharply off along a cubic, at a right
	// angle. The kernel's fitted sweep mitres no corner beside a cubic, so
	// it is made of Booleans: the line and the cubic each swept, and the
	// outside of the turn filled where the two, carried on past it along
	// their tangents, overlap. The cubic here is straight, so the volume
	// is the rectangle's area times the path's length, 6 × 32.
	doc.Features.SketchIn(doc.Workplane("ZX"), "crook path")
	crookTop := api.V(190, 0, 20)
	crookPath := []api.Entity{
		doc.Sketch.Line(api.V(190, 0, 0), crookTop),
		doc.Sketch.Cubic(crookTop, api.V(194, 0, 20), api.V(198, 0, 20), api.V(202, 0, 20)),
	}
	doc.Features.SketchIn(doc.Workplane("XY"), "crook")
	doc.Sketch.Rect(api.V(189, -1.5, 0), api.V(191, 1.5, 0))
	crook := doc.Features.Sweep(crookPath, "crook")
	if p := crook.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	end := crook.Analyze().Check()
	got, want = end.Volume-last.Volume, 6*32.0
	fmt.Printf("crook: volume %.3f mm³, %.3f the area times the path; %s\n", got, want, end.Summary())
	if !end.Sound() || math.Abs(got-want) > 0.01*want {
		os.Exit(1)
	}

	// A swan neck: two quarter arcs meeting at a right angle, twisted,
	// with no line to twist along. Each arc is held still only as far as
	// the corner's joint reaches and takes the twist along the rest.
	// Untwisted first, for the volume to compare.
	doc.Features.SketchIn(doc.Workplane("ZX"), "swan neck path")
	neckPath := []api.Entity{
		doc.Sketch.Arc(api.V(245, 0, 0), api.V(230, 0, 0), api.V(245, 0, 15)),
		doc.Sketch.Arc(api.V(260, 0, 15), api.V(245, 0, 15), api.V(260, 0, 30)),
	}
	doc.Features.SketchIn(doc.Workplane("XY"), "swan neck")
	doc.Sketch.Rect(api.V(229, -1.5, 0), api.V(231, 1.5, 0))
	neck := doc.Features.Sweep(neckPath, "swan neck")
	straight := neck.Analyze().Check()
	neck.AsSweep().SetTwist(90)
	if p := neck.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	turned := neck.Analyze().Check()
	got, want = turned.Volume-end.Volume, straight.Volume-end.Volume
	fmt.Printf("swan neck: volume %.3f mm³ twisted, %.3f untwisted; %s\n", got, want, turned.Summary())
	if !turned.Sound() || math.Abs(got-want) > 0.02*want {
		os.Exit(1)
	}
}
