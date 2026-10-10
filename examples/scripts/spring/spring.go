// A coil spring, a circle swept along a helix, made by a script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/spring/spring.go [turns] [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/spring/spring.go
//
// Sweep moves the closed loops of a sketch in a workplane along a path
// of lines, arcs and cubics. The helix here, of radius 15 and pitch 8,
// is drawn as cubics in a 3d sketch, eight a turn, for the turns given,
// 3 unless given. The wire's circle, of radius 1.5, is drawn across the
// helix's start; the sweep's surface is fitted to the true one, and its
// frames are held to the helix's axis, as a spring's wire is. The
// script checks the solid sound and its volume, the wire's section
// times the helix's length, prints it, and saves spring.tlspr, unless
// given "-".
package main

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/telesapr/telesapr/pkg/script/api"
)

func main() {
	turns, write := 3.0, true
	for _, a := range os.Args[1:] {
		if a == "-" {
			write = false
		} else if v, err := strconv.ParseFloat(a, 64); err == nil {
			turns = v
		}
	}
	// Variables, not constants: the interpreter mixes untyped float
	// constants with float64s badly.
	radius, pitch, wire := 15.0, 8.0, 1.5
	doc := api.Current()
	doc.Edit.SetChordTolerance(0.05)

	// A point of the helix and its derivative by the angle.
	at := func(a float64) (api.Vec, api.Vec) {
		return api.V(radius*math.Cos(a), radius*math.Sin(a), pitch*a/(2*math.Pi)),
			api.V(-radius*math.Sin(a), radius*math.Cos(a), pitch/(2*math.Pi))
	}
	doc.Features.Sketch3D("helix")
	n := int(math.Round(8 * turns))
	da := 2 * math.Pi * turns / float64(n)
	var path []api.Entity
	for k := 0; k < n; k++ {
		p0, d0 := at(float64(k) * da)
		p1, d1 := at(float64(k+1) * da)
		path = append(path, doc.Sketch.Cubic(p0, p0.Plus(d0.ScaledBy(da/3)), p1.Minus(d1.ScaledBy(da/3)), p1))
	}

	// The helix's length; the cubics follow it within 0.02%.
	length := turns * math.Hypot(2*math.Pi*radius, pitch)

	// The wire's circle, across the helix's start.
	p0, d0 := at(0)
	o := doc.Sketch.Point(p0)
	doc.Features.SketchAt(o, api.V(1, 0, 0), d0.WithMagnitude(1).Cross(api.V(1, 0, 0)), "wire")
	doc.Sketch.Circle(p0, wire)

	spring := doc.Features.Sweep(path, "spring")
	spring.AsSweep().SetSweepUp(api.V(0, 0, 1))
	if p := spring.Problem(); p != "" {
		fmt.Fprintln(os.Stderr, p)
		os.Exit(1)
	}
	r := spring.Analyze().Check()
	want := math.Pi * wire * wire * length
	fmt.Printf("a spring of %g turns, %.1f mm of wire: %s, %.1f mm³ (the wire's section along it, %.1f)\n",
		turns, length, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume, want)
	if !r.Sound() || math.Abs(r.Volume-want) > 0.02*want {
		os.Exit(1)
	}
	if write {
		if err := doc.File.Save("spring.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
