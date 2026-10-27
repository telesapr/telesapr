// A block's features reworked after they are made (ROADMAP F7), by a
// script.
//
// Without a window:
//
//	telesapr-cli run examples/scripts/rework/rework.go [-]
//
// In the application: Script > Run File, or
//
//	telesapr --script examples/scripts/rework/rework.go
//
// What a feature is built from can be changed after it is made, from
// its group's screen or by a setter on its typed group: here a block is
// hollowed open at its top and then at an end too, filleted along one
// bottom edge and then along another, chamfered and the chamfer moved,
// and mirrored in one plane and then another; a second block is swept and
// re-pathed, patterned and re-pathed, and three circles lofted and the
// loft cut to two. Each change is one step of undo, and each solid is
// checked. The script saves rework.tlspr, unless given "-".
package main

import (
	"fmt"
	"os"

	"github.com/telesapr/telesapr/pkg/script/api"
)

// check reports the solid of g and fails when it is not sound.
func check(g api.Group, what string) {
	if p := g.Problem(); p != "" {
		fmt.Fprintf(os.Stderr, "%s: %s\n", what, p)
		os.Exit(1)
	}
	r := g.Analyze().Check()
	fmt.Printf("%-28s %s, %.1f mm³\n", what, map[bool]string{true: "sound", false: "NOT sound"}[r.Sound()], r.Volume)
	if !r.Sound() {
		os.Exit(1)
	}
}

// lineAlong is g's line segment from a to b, either way round.
func lineAlong(g api.Group, a, b api.Vec) api.Entity {
	near := func(p, q api.Vec) bool { return p.Minus(q).Magnitude() < 1e-6 }
	for _, e := range g.Entities() {
		if e.Kind() != "line" {
			continue
		}
		s, f := e.Start().Pos(), e.End().Pos()
		if (near(s, a) && near(f, b)) || (near(s, b) && near(f, a)) {
			return e
		}
	}
	fmt.Fprintf(os.Stderr, "no line from %v to %v\n", a, b)
	os.Exit(1)
	return api.Entity{}
}

func main() {
	write := len(os.Args) < 2 || os.Args[1] != "-"
	doc := api.Current()

	// A block 20 by 10 by 10 hollowed open at its top, then at its top
	// and its right end: the shell's faces are picked on the solid
	// before it, the block. The shell comes first, as it takes a solid
	// of flat faces.
	doc.Sketch.Rect(api.V(0, 0, 0), api.V(20, 10, 0))
	block := doc.Features.Extrude(10, "block")
	top := block.Solid().FaceAt(api.V(10, 5, 10))
	end := block.Solid().FaceAt(api.V(20, 5, 5))
	hollow := doc.Features.ShellSolid([]api.SolidFace{top}, 1, "hollow")
	check(hollow, "shell open at the top")
	hollow.AsShell().SetFaces([]api.SolidFace{top, end})
	check(hollow, "open at the top and the end")

	// The bottom front edge rounded, then the bottom back edge instead,
	// each within the wall.
	front := lineAlong(block, api.V(0, 0, 0), api.V(20, 0, 0))
	back := lineAlong(block, api.V(0, 10, 0), api.V(20, 10, 0))
	round := doc.Features.Fillet([]api.Entity{front}, 0.5, "rounded")
	check(round, "fillet of the bottom front")
	round.AsFillet().SetEdges([]api.Entity{back}, []api.SolidEdge{})
	check(round, "moved to the bottom back")

	// The bottom left edge chamfered, then the bottom right edge
	// instead, by its name on the solid before the chamfer.
	left := lineAlong(block, api.V(0, 0, 0), api.V(0, 10, 0))
	cut := doc.Features.Chamfer([]api.Entity{left}, 0.5, "chamfered")
	check(cut, "chamfer of the bottom left")
	right := round.Solid().EdgeAt(api.V(20, 5, 0))
	cut.AsChamfer().SetEdges([]api.Entity{}, []api.SolidEdge{right})
	check(cut, "moved to the bottom right")

	// Mirrored in the YZ plane, then in ZX.
	mirror := doc.Features.Mirror(doc.Workplane("YZ"), "mirrored")
	check(mirror, "mirrored in YZ")
	mirror.AsMirror().SetPlane(doc.Workplane("ZX"))
	check(mirror, "mirrored in ZX")

	// A square swept along a line, then along the line and a bend.
	doc.Features.SketchIn(doc.Workplane("ZX"), "path")
	line := doc.Sketch.Line(api.V(40, 0, 0), api.V(40, 0, 10))
	bend := doc.Sketch.Arc(api.V(46, 0, 10), api.V(40, 0, 10), api.V(46, 0, 16))
	doc.Features.SketchIn(doc.Workplane("XY"), "profile")
	doc.Sketch.Rect(api.V(38, -2, 0), api.V(42, 2, 0))
	swept := doc.Features.Sweep([]api.Entity{line}, "swept")
	check(swept, "swept along the line")
	swept.AsSweep().SetPath([]api.Entity{line, bend}, []api.SolidEdge{})
	check(swept, "swept round the bend too")

	// A post repeated along one construction line, then along another.
	doc.Features.SketchIn(doc.Workplane("XY"), "posts")
	doc.Sketch.Circle(api.V(60, 0, 0), 1)
	short := doc.Sketch.Line(api.V(60, 0, 0), api.V(70, 0, 0))
	long := doc.Sketch.Line(api.V(60, 5, 0), api.V(80, 5, 0))
	short.SetConstruction(true)
	long.SetConstruction(true)
	doc.Features.Extrude(10, "post")
	posts := doc.Features.PatternAlongPath([]api.Entity{short}, 3, "posts")
	check(posts, "three posts along 10 mm")
	posts.AsRepeat().SetPath([]api.Entity{long})
	check(posts, "three posts along 20 mm")

	// Three circles lofted, then the loft between the first and the
	// last.
	var profiles []api.Group
	for i, r := range []float64{4, 6, 4} {
		doc.Features.Sketch3D("")
		o := doc.Sketch.Point(api.V(100, 0, float64(10*i)))
		profiles = append(profiles, doc.Features.SketchAt(o, api.V(1, 0, 0), api.V(0, 1, 0), ""))
		doc.Sketch.Circle(api.V(100, 0, float64(10*i)), r)
	}
	loft := doc.Features.LoftThrough(profiles, "lofted")
	check(loft, "loft through three circles")
	loft.AsLoft().SetProfiles([]api.Group{profiles[0], profiles[2]})
	check(loft, "loft between two")

	if write {
		if err := doc.File.Save("rework.tlspr"); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	}
}
